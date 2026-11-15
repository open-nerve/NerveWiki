#!/usr/bin/env bash
# make outdated（00-M12 4.3、M12/P1/S2）：只读地列出过时的 Go 模块、Go 工具链、npm 包、actions、基础镜像的摘要，
# 与 Node、PostgreSQL 的补丁版本，即 M12 的调查 I1 第 5 节的那张表；改什么由人定，它不改文件。
# 它把依赖的清单发给 Go 的模块代理与 npm registry，所以只经 make outdated 在 GitHub 的 runner 上跑（vuln.yml 的
# outdated 一项，手动触发与改了依赖清单的推送），本机不跑（00-M12 4.3）；不在 runner 上时它自己也拒绝，直接执行
# 或 make -i 都绕不过。
# 六节各打一行 "== " 起头的标题，每节或列出过时的、或写"没有过时的"（只说读到了的）。一节里的命令出错时打一行
# "!! " 起头的说明，写出是哪个命令与它的退出码，照样往下；末行写出几节出了错。它只列出、不挡，退出码是 0，除非某一节要的命令根本没有。
# 需要 go、pnpm（装好依赖）、gh（GH_TOKEN）、docker（buildx）、curl、jq。
set -uo pipefail
if [ "${GITHUB_ACTIONS:-}" != true ]; then
  echo "tools/outdated.sh 只在 GitHub 的 runner 上跑：它把依赖的清单发给外部的服务。结论看 .github/workflows/vuln.yml"
  exit 1
fi
cd "$(dirname "$0")/.." || exit 1

sections=0
failed=0
failing=0
missing=0
finish() {
  failed=$((failed + failing))
  failing=0
}
section() {
  finish
  sections=$((sections + 1))
  printf '\n== %s\n' "$1"
}
problem() {
  echo "!! $*"
  failing=1
}
need() {
  local c
  for c in "$@"; do
    command -v "$c" >/dev/null || {
      problem "没有 $c"
      missing=1
      return 1
    }
  done
}

section "1. Go 模块（server 与 server/tools 的直接依赖）"
if need go; then
  for mod in server server/tools; do
    # -f 对不列的模块也打一个空行
    if out=$(cd "$mod" && go list -m -u -f '{{if and (not .Main) (not .Indirect) .Update}}{{.Path}} {{.Version}} -> {{.Update.Version}}{{end}}' all); then
      out=$(grep -v '^$' <<<"$out")
      if [ -n "$out" ]; then sed "s|^|$mod: |" <<<"$out"; else echo "$mod: 没有过时的"; fi
    else
      problem "$mod: go list -m -u 退出码 $?"
    fi
  done
fi

section "2. Go 工具链（server/go.mod 的 toolchain 与 go.dev 上同一小版本线的最新）"
if need curl jq; then
  ours=$(sed -n 's/^toolchain //p' server/go.mod)
  line=$(sed -E 's/^go([0-9]+\.[0-9]+).*/\1/' <<<"$ours")
  # 取与解析分开跑，出错时报的是出错的那一个的退出码（接成管道时 pipefail 报的是最右边的 jq 读到空输入的 4）
  list=$(curl -fsS 'https://go.dev/dl/?mode=json&include=all')
  rc=$?
  if [ "$rc" -ne 0 ]; then
    problem "go.dev 的下载列表读不到（curl 退出码 $rc）"
  elif latest=$(jq -er --arg p "go$line." '[.[] | select(.stable and (.version | startswith($p)))][0].version' <<<"$list"); then
    if [ "$latest" = "$ours" ]; then echo "$ours：没有过时的"; else echo "$ours -> $latest"; fi
  else
    problem "go.dev 的下载列表里读不出 go$line 线的最新（jq 退出码 $?）"
  fi
fi

section "3. npm 包（pnpm outdated -r）"
if need pnpm; then
  # 有过时的时以 1 退出，出错时多半也是 1：出错的认它的 ERR_PNPM
  out=$(pnpm outdated -r --format list 2>&1)
  rc=$?
  if [ "$rc" -gt 1 ] || grep -q 'ERR_PNPM' <<<"$out"; then
    [ -n "$out" ] && echo "$out"
    problem "pnpm outdated 退出码 $rc"
  elif [ -n "$out" ]; then
    echo "$out"
  else
    echo "没有过时的"
  fi
fi

section "4. actions（.github/workflows/ 里按 SHA 引用的与它们最新的发布）"
if need gh; then
  refs=$(grep -hoE 'uses: [A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+[^@ ]*@[0-9a-f]{40} # v[^ ]+' .github/workflows/*.yml | sort -u)
  [ -n "$refs" ] || problem "workflows 里没有按 SHA 引用的 action"
  # "没有过时的"只说读到了的：一个也没读到时不写，读到一部分时写出读到几个
  stale=0
  got=0
  all=0
  while read -r _ ref _ version; do
    [ -n "${ref:-}" ] || continue
    all=$((all + 1))
    repo=$(cut -d/ -f1-2 <<<"${ref%@*}")
    if latest=$(gh api "repos/$repo/releases/latest" --jq .tag_name); then
      got=$((got + 1))
      if [ "$latest" != "$version" ]; then
        echo "$repo $version -> $latest"
        stale=1
      fi
    else
      problem "$repo: 读不出最新的发布（gh 退出码 $?）"
    fi
  done <<<"$refs"
  if [ "$stale" -eq 0 ] && [ "$got" -eq "$all" ] && [ "$all" -gt 0 ]; then
    echo "没有过时的（$all 个 action）"
  elif [ "$stale" -eq 0 ] && [ "$got" -gt 0 ]; then
    echo "没有过时的（读到 $got 个，共 $all 个）"
  fi
fi

section "5. 基础镜像（deploy/Dockerfile 的摘要与 tag 现在指向的）"
if need docker jq; then
  froms=$(grep -oE '^FROM [^ ]+:[^ @]+@sha256:[0-9a-f]{64}' deploy/Dockerfile | cut -d' ' -f2)
  [ -n "$froms" ] || problem "deploy/Dockerfile 里没有带摘要的 FROM"
  stale=0
  got=0
  all=0
  while read -r from; do
    [ -n "$from" ] || continue
    all=$((all + 1))
    image=${from%@*}
    ours=${from#*@}
    manifest=$(docker buildx imagetools inspect "$image" --format '{{json .Manifest}}')
    rc=$?
    if [ "$rc" -ne 0 ]; then
      problem "$image: 读不出 tag 现在的摘要（docker 退出码 $rc）"
    elif now=$(jq -er .digest <<<"$manifest"); then
      got=$((got + 1))
      if [ "$now" != "$ours" ]; then
        echo "$image: $ours -> $now"
        stale=1
      fi
    else
      problem "$image: docker 的回答里没有摘要（jq 退出码 $?）"
    fi
  done <<<"$froms"
  if [ "$stale" -eq 0 ] && [ "$got" -eq "$all" ] && [ "$all" -gt 0 ]; then
    echo "没有过时的（$all 个镜像）"
  elif [ "$stale" -eq 0 ] && [ "$got" -gt 0 ]; then
    echo "没有过时的（读到 $got 个，共 $all 个）"
  fi
fi

section "6. Node 与 PostgreSQL 的补丁版本（.node-version 的大版本；pgtest 的镜像）"
if need curl jq; then
  major=$(tr -dc 0-9 <.node-version)
  list=$(curl -fsS https://nodejs.org/dist/index.json)
  rc=$?
  if [ "$rc" -ne 0 ]; then
    problem "nodejs.org 的版本列表读不到（curl 退出码 $rc）"
  elif latest=$(jq -er --arg p "v$major." '[.[] | select(.version | startswith($p))][0].version' <<<"$list"); then
    echo "Node $major：线上最新 $latest（镜像里的 Node 跟着第 5 节的 node 摘要）"
  else
    problem "nodejs.org 的版本列表里读不出 Node $major 的最新（jq 退出码 $?）"
  fi
  pg=$(sed -nE 's/^[[:space:]]*image[[:space:]]*= "postgres:([0-9]+\.[0-9]+)-.*/\1/p' server/internal/platform/postgres/pgtest/pgtest.go)
  if [ -z "$pg" ]; then
    problem "pgtest.go 里读不出 PostgreSQL 的镜像"
  else
    list=$(curl -fsS https://www.postgresql.org/versions.json)
    rc=$?
    if [ "$rc" -ne 0 ]; then
      problem "postgresql.org 的版本列表读不到（curl 退出码 $rc）"
    elif latest=$(jq -er --arg m "${pg%%.*}" '.[] | select(.major == $m) | .latestMinor' <<<"$list"); then
      if [ "$latest" = "${pg#*.}" ]; then
        echo "PostgreSQL $pg：没有过时的"
      else
        echo "PostgreSQL $pg -> ${pg%%.*}.$latest"
      fi
    else
      problem "postgresql.org 的版本列表里读不出 PostgreSQL ${pg%%.*} 的最新（jq 退出码 $?）"
    fi
  fi
fi

finish
printf '\noutdated: %d sections, %d with errors\n' "$sections" "$failed"
[ "$missing" -eq 0 ]
