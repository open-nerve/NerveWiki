#!/usr/bin/env bash
# 发布的闸门（00-M12 4.10、M12/P5/S1）。推 tag 时 release.yml 在构建之前先跑它，在仓库根执行：
#   deploy/release-gate.sh <tag>
# 依次核对，一条不过就以 1 退出并写明是哪一条：
#   1. tag 是 vX.Y.Z 或 vX.Y.Z-rc.N，数字没有前导的 0；
#   2. tag 指的提交，与触发这次运行的 GITHUB_SHA 一致（设了时）；
#   3. 它在 origin/main 上（检出要 fetch-depth: 0）；
#   4. CHANGELOG.md 恰有一个"## X.Y.Z（…）"的标题（rc 取去掉 -rc.N 的那一个）；正式版本的括号里要是日期 YYYY-MM-DD；
#      这一节照 deploy/release-notes.sh 取得出、不是空的；
#   5. 这个提交在 main 上由 push 触发的 ci.yml 的运行已成功：只查 ci.yml，取最后建的那一次，它的结论是最后一次尝试的；
#      release.yml 的演练绿了不算。还在跑的也拒绝：CI 绿了之后在同一次运行上重跑这个作业。
# 都过时打出 version=、prerelease=、sha= 三行，设了 GITHUB_OUTPUT 时也写进它。make vuln（npm 一侧照 high）是工作流的
# 下一步：它只在 runner 上跑，这个脚本要能在本机对着假的输入跑。
# 要 git、jq、awk、gh（GH_TOKEN 要 actions: read）、GITHUB_REPOSITORY 与 IMAGE（第 4 条取一节时用）。
# server/internal/platform/buildinfo/release_test.go 用假的 gh 与临时的仓库核对每一条。
set -uo pipefail

fail() {
  echo "release-gate: $*" >&2
  exit 1
}

[ $# -eq 1 ] || fail "用法：deploy/release-gate.sh <tag>"
tag=$1
repo=${GITHUB_REPOSITORY:-}
[ -n "$repo" ] || fail "要 GITHUB_REPOSITORY（owner/repo）"
[ -n "${IMAGE:-}" ] || fail "要 IMAGE（镜像名，不带 tag）"

# 1. 写法
[[ $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$ ]] ||
  fail "tag \"$tag\" 不是 vX.Y.Z 或 vX.Y.Z-rc.N"
version=${tag#v}
base=${version%%-rc.*}
prerelease=false
[ "$version" = "$base" ] || prerelease=true

# 2. 提交
sha=$(git rev-parse --verify --quiet "refs/tags/$tag^{commit}") || fail "本地没有 tag $tag"
if [ -n "${GITHUB_SHA:-}" ]; then
  pushed=$(git rev-parse --verify --quiet "$GITHUB_SHA^{commit}") || fail "本地没有 GITHUB_SHA $GITHUB_SHA"
  [ "$pushed" = "$sha" ] || fail "tag $tag 指 $sha，触发这次运行的是 $GITHUB_SHA"
fi

# 3. 在 main 上
git rev-parse --verify --quiet refs/remotes/origin/main >/dev/null || fail "没有 origin/main：检出要 fetch-depth: 0"
git merge-base --is-ancestor "$sha" refs/remotes/origin/main || fail "提交 $sha 不在 origin/main 上"

# 4. CHANGELOG.md 的一节。按字面比，同 release-notes.sh：版本号里的 . 不当通配
[ -f CHANGELOG.md ] || fail "没有 CHANGELOG.md"
heads=$(awk -v h="## $base（" 'index($0, h) == 1 && /）$/' CHANGELOG.md)
count=$(printf '%s' "$heads" | grep -c '^')
[ "$count" = 1 ] || fail "CHANGELOG.md 里\"## $base（…）\"的标题有 $count 个，要恰好一个"
if [ "$prerelease" = false ]; then
  [[ $heads =~ （[0-9]{4}-[0-9]{2}-[0-9]{2}）$ ]] ||
    fail "正式版本 $version 的标题要写发布的日期：## $base（YYYY-MM-DD）"
fi
"$(dirname "$0")/release-notes.sh" "$tag" "sha256:$(printf '0%.0s' {1..64})" >/dev/null ||
  fail "CHANGELOG.md 里 $base 的一节取不出或是空的（见上一行）"

# 5. ci.yml 在 main 上的运行
runs=$(gh api "repos/$repo/actions/workflows/ci.yml/runs?head_sha=$sha&branch=main&event=push&per_page=100") ||
  fail "读不到 ci.yml 在 main 上的运行"
run=$(jq -c --arg sha "$sha" '[.workflow_runs[]
    | select(.path == ".github/workflows/ci.yml" and .head_sha == $sha and .head_branch == "main" and .event == "push")]
  | sort_by(.created_at) | last // empty' <<<"$runs") || fail "ci.yml 的运行读不懂：$(head -c 300 <<<"$runs")"
[ -n "$run" ] || fail "提交 $sha 在 main 上没有 push 触发的 ci.yml 的运行：tag 要打在单独推上 main、有自己的 CI 运行的提交上"
read -r status conclusion id attempt <<<"$(jq -r '"\(.status) \(.conclusion) \(.id) \(.run_attempt)"' <<<"$run")"
[ "$status" = completed ] && [ "$conclusion" = success ] ||
  fail "ci.yml 的运行 $id（第 $attempt 次尝试）是 $status $conclusion，要 completed success"

printf 'version=%s\nprerelease=%s\nsha=%s\n' "$version" "$prerelease" "$sha"
if [ -n "${GITHUB_OUTPUT:-}" ]; then
  printf 'version=%s\nprerelease=%s\nsha=%s\n' "$version" "$prerelease" "$sha" >>"$GITHUB_OUTPUT"
fi
