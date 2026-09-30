#!/usr/bin/env bash
# 在镜像上跑 S1、S3（docs/v0.1/M0-foundation/06-P6-e2e-delivery.md 3.5）：在临时的 Docker 网络里启动
# PostgreSQL，用镜像执行 migrate up、启动 serve，核对 /healthz、/readyz、/api/v0/instance 与进程的用户，
# 最后停止服务并核对它正常退出。无论成败都清理容器与网络；失败时打印服务的日志。
# 用法：deploy/image-smoke.sh <镜像> <期望的版本号>（make image-smoke）
set -euo pipefail

image=$1
version=$2

name=nervewiki-smoke-$$
db=$name-db
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
  docker rm -f "$app" "$db" >/dev/null 2>&1 || true
  docker network rm "$name" >/dev/null 2>&1 || true
  # 否则脚本的退出码是上一行的，set -u 之类的失败会被当成通过
  exit "$status"
}
trap cleanup EXIT

# 等 $1（命令）成功，最多 timeout_s 秒
wait_for() {
  for _ in $(seq "$timeout_s"); do
    if "$@" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

# 与开发库、端到端测试相同的 PostgreSQL：builtin C.UTF-8。
# 初始化期间 PostgreSQL 只听 Unix 套接字，所以按 TCP 判断就绪
docker network create "$name" >/dev/null
docker run -d --name "$db" --network "$name" \
  -e POSTGRES_USER=nervewiki -e POSTGRES_PASSWORD=nervewiki -e POSTGRES_DB=nervewiki \
  -e POSTGRES_INITDB_ARGS="--locale-provider=builtin --locale=C.UTF-8" \
  postgres:18.6-trixie >/dev/null
wait_for docker exec "$db" pg_isready -h 127.0.0.1 -U nervewiki -d nervewiki || fail "PostgreSQL 没有就绪"

# 部署的顺序：prod 配置不自动迁移，先 migrate up，再 serve
docker run --rm --network "$name" -e NWIKI_DATABASE__URL="$database_url" "$image" migrate up
docker run -d --name "$app" --network "$name" -e NWIKI_DATABASE__URL="$database_url" -p 127.0.0.1::8080 "$image" >/dev/null
base="http://$(docker port "$app" 8080/tcp | head -n 1)"
wait_for curl -fsS "$base/readyz" || fail "/readyz 在 ${timeout_s} 秒内没有答 200"

# S1
[[ $(curl -fsS "$base/healthz") == '{"status":"ok"}' ]] || fail "/healthz 不是 {\"status\":\"ok\"}"
[[ $(curl -fsS "$base/readyz") == '{"status":"ok"}' ]] || fail "/readyz 不是 {\"status\":\"ok\"}"

# S3：注入的版本号，与构建时带进来的 .git 取出的提交
instance=$(curl -fsS "$base/api/v0/instance")
jq -e --arg version "$version" \
  '. == {product: "Nerve Wiki", version: $version, commit: .commit, api_version: "v0"} and (.commit | test("^[0-9a-f]{40}$"))' \
  <<<"$instance" >/dev/null || fail "/api/v0/instance 与预期不符：$instance"

# 进程以非 root 用户运行（distroless 的 nonroot，uid 65532）
uid=$(docker top "$app" -o pid,uid | awk 'NR > 1 { print $2 }')
[[ $uid == 65532 ]] || fail "进程的 uid 是 ${uid:-空}，应为 65532（nonroot）"

# SIGTERM 之后优雅停机，退出码 0
docker stop -t 30 "$app" >/dev/null
exit_code=$(docker container inspect -f '{{.State.ExitCode}}' "$app")
[[ $exit_code == 0 ]] || fail "停止后的退出码是 $exit_code，应为 0"

echo "image-smoke: ${image} 通过（版本 ${version}）"
