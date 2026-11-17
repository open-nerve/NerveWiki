#!/usr/bin/env bash
# A production serve over a PostgreSQL container of its own, for M12/P4's measurements
# (docs/v0.1/M12-release/04-P4-performance.md 3.4.1), written as deploy/image-smoke.sh is: each step's
# status checked (set -e is not relied on), and only what it started stopped or removed, by its PID and
# its container's id.
#
#   stack.sh up <dir> [--fsync-off] [--env KEY=VALUE ...]
#       A round a directory: <dir> is new or empty, else up refuses it, so that a round's serve.log (its
#       gc trace) and data/ are never another round's or overwritten (review B's M3).
#       PostgreSQL 18.7 (builtin C.UTF-8, its port on 127.0.0.1 alone; fsync=off only with --fsync-off,
#       B4's control), a JWT key, migrate up, the account perf@example.com (its password in
#       <dir>/password), then serve: NWIKI_ENV=prod on 127.0.0.1:0, storage in <dir>/data,
#       GODEBUG=gctrace=1, the three password and login buckets at 600 a minute (decision 460), and the
#       --env given; its standard output and error in <dir>/serve.log. Ready once /readyz answers 200 and
#       the log has the notification listener's and the jobs' lines. <dir>/stack.env is then written,
#       a file the shell can source: the address, serve's PID, the container's id, the database's
#       address, the versions, fsync and the --env given.
#   stack.sh restart <dir>
#       serve stopped (SIGTERM, waited for), the container restarted, serve started again, its earlier
#       log kept as serve.log.<unix seconds>; the data stays (batch E's second round).
#   stack.sh down <dir>
#       serve stopped, the container stopped and removed with its volumes; <dir> stays, stack.env
#       renamed stack.env.down: its serve.log and data/ are the round's record, and the next round
#       takes another directory.
#
# The binary is $NERVEWIKI, by default bin/nervewiki of this script's worktree: make build on the
# commit measured, in its worktree. Needs Docker, curl and openssl.
set -u

usage="usage: stack.sh up <dir> [--fsync-off] [--env KEY=VALUE ...] | restart <dir> | down <dir>"
command=${1:-}
dir=${2:-}
if [[ -z $command || -z $dir ]]; then
  echo "$usage" >&2
  exit 2
fi
shift 2

here=$(cd "$(dirname "$0")" && pwd) || exit 1
repo=$(git -C "$here" rev-parse --show-toplevel) || exit 1
bin=${NERVEWIKI:-$repo/bin/nervewiki}
image=postgres:18.7-trixie
email=perf@example.com
timeout_s=120

fail() {
  echo "stack.sh: $*" >&2
  exit 1
}

# wait_for runs its command each second until it succeeds, timeout_s seconds at most; with $watch set,
# a PID, it gives up as soon as that process is gone (a serve that failed to start, review B's N1).
wait_for() {
  local _
  for _ in $(seq "$timeout_s"); do
    if "$@" >/dev/null 2>&1; then
      return 0
    fi
    if [[ -n ${watch:-} ]] && ! kill -0 "$watch" 2>/dev/null; then
      echo "stack.sh: serve $watch exited" >&2
      return 1
    fi
    sleep 1
  done
  return 1
}

# ours reports whether pid is a nervewiki serve: a PID is stopped only when it is still the process
# it started.
ours() {
  local cmdline
  cmdline=$(tr '\0' ' ' <"/proc/$1/cmdline" 2>/dev/null) || return 1
  [[ $cmdline == *nervewiki*serve* ]]
}

# stop_serve stops serve by its PID: SIGTERM, then its graceful stop waited for, timeout_s seconds.
stop_serve() {
  local pid=$1
  if ! ours "$pid"; then
    echo "stack.sh: serve $pid is gone already" >&2
    return 0
  fi
  kill -TERM "$pid" || return 1
  for _ in $(seq "$timeout_s"); do
    if ! kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "stack.sh: serve $pid did not stop in ${timeout_s} s" >&2
  return 1
}

# database_url is the container's database's address on its port on 127.0.0.1.
database_url() {
  local port
  port=$(docker port "$1" 5432/tcp | head -n 1) || return 1
  [[ $port == 127.0.0.1:* ]] || return 1
  echo "postgres://nervewiki:nervewiki@$port/nervewiki?sslmode=disable"
}

# start_serve starts serve with the stack's environment and <dir>/serve.env's lines, waits for it to be
# ready, and sets pid and addr.
start_serve() {
  local url=$1 extra=() line
  while IFS= read -r line; do
    [[ -n $line ]] && extra+=("$line")
  done <"$dir/serve.env"
  rm -f "$dir/addr"
  env NWIKI_ENV=prod NWIKI_DATABASE__URL="$url" NWIKI_AUTH__JWT__PRIVATE_KEY_FILE="$dir/jwt.pem" \
    NWIKI_SERVER__ADDR=127.0.0.1:0 NWIKI_SERVER__ADDR_FILE="$dir/addr" NWIKI_STORAGE__DIR="$dir/data" GODEBUG=gctrace=1 \
    NWIKI_RATELIMIT__PASSWORD_USER__PER_MINUTE=600 NWIKI_RATELIMIT__LOGIN_IP__PER_MINUTE=600 \
    NWIKI_RATELIMIT__LOGIN_IP_EMAIL__PER_MINUTE=600 "${extra[@]}" \
    nohup "$bin" serve >"$dir/serve.log" 2>&1 </dev/null &
  pid=$!
  local watch=$pid
  wait_for test -s "$dir/addr" || return 1
  addr=$(cat "$dir/addr") || return 1
  wait_for curl -fsS --max-time 5 "http://$addr/readyz" || return 1
  # Production's log is JSON (config.yaml's log.format): e2e's waits read the text msg="…" (review A's M8).
  wait_for grep -q '"msg":"notification listener listening"' "$dir/serve.log" || return 1
  wait_for grep -q '"msg":"jobs started"' "$dir/serve.log" || return 1
}

# write_env writes stack.env, which the shell can source.
write_env() {
  local version pg
  version=$("$bin" version) || return 1
  pg=$(docker exec "$container" psql -XAt -U nervewiki -d nervewiki -c 'SELECT version()') || return 1
  {
    printf 'ADDR=%q\nBASE=%q\nSERVE_PID=%q\nCONTAINER=%q\nDATABASE_URL=%q\n' "$addr" "http://$addr" "$pid" "$container" "$url"
    printf 'STORAGE=%q\nPASSWORD_FILE=%q\nEMAIL=%q\nNERVEWIKI_VERSION=%q\nPOSTGRES_VERSION=%q\n' "$dir/data" "$dir/password" \
      "$email" "$version" "$pg"
    printf 'FSYNC_OFF=%q\nSERVE_ENV=%q\n' "$fsync_off" "$(paste -sd ' ' "$dir/serve.env")"
  } >"$dir/stack.env.tmp" || return 1
  mv "$dir/stack.env.tmp" "$dir/stack.env"
}

case $command in
up)
  fsync_off=false
  extra=()
  while [[ $# -gt 0 ]]; do
    case $1 in
    --fsync-off) fsync_off=true ;;
    --env)
      [[ $# -ge 2 && $2 =~ ^[A-Z_][A-Z0-9_]*=.*$ ]] || fail "--env wants KEY=VALUE: ${2:-}"
      extra+=("$2")
      shift
      ;;
    *) fail "$usage" ;;
    esac
    shift
  done
  [[ -x $bin ]] || fail "no binary $bin: make build in the measured commit's worktree, or set NERVEWIKI"
  [[ ! -e $dir/stack.env ]] || fail "$dir has a stack up: stack.sh down $dir first"
  if [[ -e $dir ]]; then
    [[ -d $dir && -z $(ls -A "$dir") ]] || fail "$dir is not an empty directory: a round a directory, its serve.log and data/ kept"
  fi
  mkdir -p "$dir/data" || fail "mkdir $dir"
  dir=$(cd "$dir" && pwd) || exit 1
  : >"$dir/serve.env" || fail "writing serve.env"
  if [[ ${#extra[@]} -gt 0 ]]; then
    printf '%s\n' "${extra[@]}" >"$dir/serve.env" || fail "writing serve.env"
  fi

  container=
  pid=
  # Whatever up started is stopped and removed when a step fails, by its PID and id.
  undo() {
    local status=$?
    if [[ $status -ne 0 ]]; then
      [[ -n $pid ]] && stop_serve "$pid"
      [[ -n $container ]] && docker rm -fv "$container" >/dev/null
      [[ -f $dir/serve.log ]] && tail -n 40 "$dir/serve.log" >&2
    fi
    exit "$status"
  }
  trap undo EXIT

  args=()
  [[ $fsync_off == true ]] && args=(-c fsync=off)
  container=$(docker run -d -p 127.0.0.1::5432 -e POSTGRES_USER=nervewiki -e POSTGRES_PASSWORD=nervewiki \
    -e POSTGRES_DB=nervewiki -e POSTGRES_INITDB_ARGS="--locale-provider=builtin --locale=C.UTF-8" \
    "$image" "${args[@]}") || fail "docker run $image"
  # While it initializes, PostgreSQL listens on its Unix socket alone: ready by TCP.
  wait_for docker exec "$container" pg_isready -h 127.0.0.1 -U nervewiki -d nervewiki || fail "PostgreSQL not ready"
  url=$(database_url "$container") || fail "the container's port"

  openssl genpkey -algorithm ed25519 -out "$dir/jwt.pem" 2>/dev/null || fail "openssl genpkey"
  chmod 600 "$dir/jwt.pem" || fail "chmod jwt.pem"
  # Production does not migrate by itself (config.prod.yaml).
  NWIKI_ENV=prod NWIKI_DATABASE__URL="$url" NWIKI_AUTH__JWT__PRIVATE_KEY_FILE="$dir/jwt.pem" "$bin" migrate up \
    >"$dir/migrate.log" 2>&1 || fail "migrate up: $(tail -n 5 "$dir/migrate.log")"
  password=$(openssl rand -hex 16) || fail "openssl rand"
  (umask 077 && printf '%s\n' "$password" >"$dir/password") || fail "writing the password"
  printf '%s\n' "$password" | NWIKI_ENV=prod NWIKI_DATABASE__URL="$url" NWIKI_AUTH__JWT__PRIVATE_KEY_FILE="$dir/jwt.pem" \
    "$bin" users create --email "$email" >"$dir/users.log" 2>&1 || fail "users create: $(cat "$dir/users.log")"

  start_serve "$url" || fail "serve not ready"
  write_env || fail "writing stack.env"
  trap - EXIT
  echo "stack.sh: up at http://$addr, serve $pid, container $container; $dir/stack.env"
  ;;
restart)
  [[ -f $dir/stack.env ]] || fail "no stack up in $dir"
  dir=$(cd "$dir" && pwd) || exit 1
  # shellcheck source=/dev/null
  source "$dir/stack.env" || fail "reading stack.env"
  container=$CONTAINER
  fsync_off=$FSYNC_OFF
  stop_serve "$SERVE_PID" || fail "stopping serve $SERVE_PID"
  mv "$dir/serve.log" "$dir/serve.log.$(date +%s)" || fail "keeping serve.log"
  docker restart "$container" >/dev/null || fail "docker restart $container"
  wait_for docker exec "$container" pg_isready -h 127.0.0.1 -U nervewiki -d nervewiki || fail "PostgreSQL not ready"
  url=$(database_url "$container") || fail "the container's port"
  pid=
  start_serve "$url" || {
    [[ -n $pid ]] && stop_serve "$pid"
    fail "serve not ready after the restart"
  }
  write_env || fail "writing stack.env"
  echo "stack.sh: restarted at http://$addr, serve $pid, container $container"
  ;;
down)
  [[ -f $dir/stack.env ]] || fail "no stack up in $dir"
  # shellcheck source=/dev/null
  source "$dir/stack.env" || fail "reading stack.env"
  status=0
  stop_serve "$SERVE_PID" || status=1
  docker stop "$CONTAINER" >/dev/null || status=1
  docker rm -v "$CONTAINER" >/dev/null || status=1
  [[ $status -eq 0 ]] || fail "down left something: serve $SERVE_PID, container $CONTAINER"
  mv "$dir/stack.env" "$dir/stack.env.down" || fail "renaming stack.env"
  echo "stack.sh: down; serve $SERVE_PID stopped, container $CONTAINER removed"
  ;;
*)
  echo "$usage" >&2
  exit 2
  ;;
esac
