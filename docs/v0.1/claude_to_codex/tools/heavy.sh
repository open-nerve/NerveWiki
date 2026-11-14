#!/bin/bash
# Runs "$@" holding a machine-wide lock, so that only one Docker or PostgreSQL
# heavy run (bootstrap tests, make test, make e2e, timing measurements) runs at
# a time when several agents share one machine. Waits until mkdir takes the
# lock, and releases only the lock it took. The lock stays in /tmp whatever
# TMPDIR says, so that agents that move TMPDIR off /tmp still share it; set
# NW_HEAVY_LOCK to move it.
#
#   tools/heavy.sh make test
L=${NW_HEAVY_LOCK:-/tmp/nervewiki-heavy.lock}
until mkdir "$L" 2>/dev/null; do sleep 5; done
trap 'rmdir "$L"' EXIT
"$@"
