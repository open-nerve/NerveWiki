#!/bin/bash
# Finds the GitHub Actions run of a commit and waits until it completes, then
# prints each job's conclusion and time. Uses gh when it is installed and
# logged in, else the public REST API (60 requests an hour per IP: it polls
# every 2 minutes).
#
#   tools/ci-wait.sh <full sha>
sha=$1
repo=${NW_REPO:-open-nerve/NerveWiki}
api=https://api.github.com/repos/$repo/actions
get() { if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then gh api "${1#https://api.github.com/}"; else curl -s "$1"; fi; }
for _ in $(seq 1 15); do
  run=$(get "$api/runs?head_sha=$sha&per_page=5" | python3 -c 'import json,sys; r=json.load(sys.stdin).get("workflow_runs", []); print(r[0]["id"] if r else "")')
  [ -n "$run" ] && break
  sleep 60
done
[ -z "$run" ] && { echo "no run for $sha"; exit 1; }
echo "run $run for $sha"
while true; do
  state=$(get "$api/runs/$run" | python3 -c 'import json,sys; r=json.load(sys.stdin); print(r["status"], r.get("conclusion"))')
  set -- $state
  [ "$1" = completed ] && break
  sleep 120
done
echo "run $run: completed $2"
get "$api/runs/$run/jobs?per_page=50" | python3 -c '
import json, sys
from datetime import datetime
f = lambda s: datetime.strptime(s, "%Y-%m-%dT%H:%M:%SZ")
for j in json.load(sys.stdin)["jobs"]:
    if not j.get("completed_at"):
        print(j["name"], j["conclusion"]); continue
    d = (f(j["completed_at"]) - f(j["started_at"])).seconds
    steps = [s for s in j["steps"] if s["name"] in ("Test", "E2E") and s.get("completed_at")]
    st = "".join(" (%s %d:%02d)" % (s["name"], (f(s["completed_at"]) - f(s["started_at"])).seconds // 60, (f(s["completed_at"]) - f(s["started_at"])).seconds % 60) for s in steps)
    print("%s %s %d:%02d%s" % (j["name"], j["conclusion"], d // 60, d % 60, st))
'
[ "$2" = success ]
