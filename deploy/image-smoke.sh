#!/usr/bin/env bash
# 在镜像上跑 S1、S3（docs/v0.1/M0-foundation/06-P6-e2e-delivery.md 3.5）：在临时的 Docker 网络里启动
# PostgreSQL，用镜像执行 migrate up、启动 serve，核对探针、内嵌的前端、实例信息、提交信息与进程的用户，
# 最后停止服务并核对它正常退出。无论成败都清理容器与网络；失败时打印服务的日志。
# 镜像要由当前工作区构建：提交信息与本地的 git 核对（make image-smoke 先执行 make image）。
# 需要 Docker、curl、jq。
set -euo pipefail

usage="用法：deploy/image-smoke.sh <镜像> <期望的版本号>"
image=${1:?$usage}
version=${2:?$usage}

repo=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
commit=$(git -C "$repo" rev-parse HEAD)
modified=false
if [[ -n $(git -C "$repo" status --porcelain) ]]; then
  modified=true
fi

name=nervewiki-smoke-$$
db=$name-db
migrate=$name-migrate
app=$name-app
# 本机临时的账号密码，不是机密
database_url="postgres://nervewiki:nervewiki@$db:5432/nervewiki?sslmode=disable"
timeout_s=60

fail() {
  echo "image-smoke: $*" >&2
  exit 1
}

cleanup() {
  local status=$?
  if [[ $status -ne 0 ]] && docker container inspect "$app" >/dev/null 2>&1; then
    echo "--- nervewiki 的日志 ---" >&2
    docker logs "$app" >&2 || true
  fi
  # -v：PostgreSQL 镜像声明了数据卷，不带 -v 每次都会留下一个匿名卷
  docker rm -fv "$app" "$migrate" "$db" >/dev/null 2>&1 || true
  docker network rm "$name" >/dev/null 2>&1 || true
  # 否则脚本的退出码是上一行的，set -u 之类的失败会被当成通过
  exit "$status"
}
trap cleanup EXIT

# 等命令成功，最多 timeout_s 秒
wait_for() {
  for _ in $(seq "$timeout_s"); do
    if "$@" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

# 服务接受连接却不回答时，每个请求最多等 5 秒
get() {
  curl -fsS --max-time 5 "$@"
}

# 二进制里的提交信息与构建它的工作区一致：构建上下文带着完整的工作区与 .git（.dockerignore）
want="nervewiki $version commit=$commit commit_time=[^ ]+ modified=$modified"
got=$(docker run --rm "$image" version)
[[ $got =~ ^$want$ ]] || fail "version 输出 \"$got\"，应为 \"$want\""

# 与开发库、端到端测试相同的 PostgreSQL：builtin C.UTF-8。
# 初始化期间 PostgreSQL 只听 Unix 套接字，所以按 TCP 判断就绪
docker network create "$name" >/dev/null
docker run -d --name "$db" --network "$name" \
  -e POSTGRES_USER=nervewiki -e POSTGRES_PASSWORD=nervewiki -e POSTGRES_DB=nervewiki \
  -e POSTGRES_INITDB_ARGS="--locale-provider=builtin --locale=C.UTF-8" \
  postgres:18.6-trixie >/dev/null
wait_for docker exec "$db" pg_isready -h 127.0.0.1 -U nervewiki -d nervewiki || fail "PostgreSQL 没有就绪"

# 部署的顺序：prod 配置不自动迁移，先 migrate up，再 serve
docker run --name "$migrate" --network "$name" -e NWIKI_DATABASE__URL="$database_url" "$image" migrate up
docker run -d --name "$app" --network "$name" -e NWIKI_DATABASE__URL="$database_url" -p 127.0.0.1::8080 "$image" >/dev/null
base="http://$(docker port "$app" 8080/tcp | head -n 1)"
wait_for get "$base/readyz" || fail "/readyz 在 ${timeout_s} 秒内没有答 200"

# S1
[[ $(get "$base/healthz") == '{"status":"ok"}' ]] || fail "/healthz 不是 {\"status\":\"ok\"}"
[[ $(get "$base/readyz") == '{"status":"ok"}' ]] || fail "/readyz 不是 {\"status\":\"ok\"}"

# 内嵌的前端：没有它，页面路径答 404 与"未构建"的提示
page=$(get -D - "$base/") || fail "/ 没有答 200：镜像里没有内嵌前端？"
grep -qi "^content-security-policy: default-src 'self'; script-src 'self';" <<<"$page" || fail "/ 没有页面的 CSP"
grep -q '<div id="root"></div>' <<<"$page" || fail "/ 不是前端的 index.html"

# S3：注入的版本号，构建它的提交
instance=$(get "$base/api/v0/instance")
jq -e --arg version "$version" --arg commit "$commit" \
  '. == {product: "Nerve Wiki", version: $version, commit: $commit, api_version: "v0"}' \
  <<<"$instance" >/dev/null || fail "/api/v0/instance 与预期不符：$instance"

# 进程以非 root 用户运行（distroless 的 nonroot，uid 65532）
uid=$(docker top "$app" -o pid,uid | awk 'NR > 1 { print $2 }')
[[ $uid == 65532 ]] || fail "进程的 uid 是 ${uid:-空}，应为 65532（nonroot）"

# SIGTERM 之后优雅停机，退出码 0
docker stop -t 30 "$app" >/dev/null
exit_code=$(docker container inspect -f '{{.State.ExitCode}}' "$app")
[[ $exit_code == 0 ]] || fail "停止后的退出码是 $exit_code，应为 0"

echo "image-smoke: ${image} 通过（版本 ${version}，提交 ${commit}，modified=${modified}）"
