// Command perf is M12/P4's measuring tools (docs/v0.1/M12-release/
// 04-P4-performance.md, section 3.4): a serve's and its database's
// memory and CPU, the vault an import takes, and the drivers of the
// measurements a browser or a composition root's test cannot take. It
// uses the standard library alone: it reads the database through psql in
// its container, and speaks MCP's JSON-RPC by hand (design 2.3's P4-6).
//
// A round starts a production serve over a PostgreSQL container of its
// own with stack.sh (3.4.1; restart keeps the data for batch E's second
// round, down removes the container), then prepares the accounts:
//
//	server/tools/perf/stack.sh up <dir> [--fsync-off] [--env KEY=VALUE ...]
//	. <dir>/stack.env
//	PERF_PASSWORD=$(cat <dir>/password) perf prepare -base $BASE -notebooks vault,tree -pats 50 -out <dir>/pats.json
//
// The subcommands, and the rows of the design they serve:
//
//	perf sample -pid $SERVE_PID -container $CONTAINER -out <csv>
//	    3.4.2: a row every 100 ms of the serve's VmRSS, VmHWM and CPU and
//	    the container's memory.current and CPU, the container's
//	    memory.peak and memory.stat at SIGINT; the serve's start in the
//	    file's first line, for gctrace's -start.
//	perf gctrace -log <dir>/serve.log -start <ns> -base <t> -from <t> -to <t>
//	    3.4.2: the round's baseline (its forced collection after an idle
//	    of 130 s), a combination's peak, its delta, its upper bound and the
//	    collections missing; times in Unix nanoseconds or RFC 3339.
//	perf vault -out <zip> -pages 45000 -attachments 5000 -fanout 10,10,98 -links 30 -body 2KiB -attachment-bytes 800KiB -random 12 -seed 1201
//	    3.4.3: the import-limit vault (batches F and B4); the 50,000 pages
//	    of batch E are -pages 50000 -fanout 10,50 -links 10 -body 1KiB.
//	perf bodies -prepared <dir>/pats.json -records <jsonl> -k 8 -kind normal|pathological [-same-notebook]
//	    B2: readers and writers of 5 MiB pages; the busy answers early.
//	perf mcp -prepared … -records … -shape 1|2a|2b|2c|3|4 -reps 3 -gap 20s
//	    B3: three credentials' large calls at once.
//	perf editors -prepared … -records … -n 20 -for 120s -container $CONTAINER -monitor <jsonl> [-heavy-links 10000]
//	    3.7's M4-performance 1 and 6, and batch G's bytes a version.
//	perf tabs -prepared … -records … -k 4 -for 60s -notebook tree
//	    3.7's reread of a tree of 100,000 pages.
//	perf ops -prepared … -records … -notebook vault -container $CONTAINER
//	    B4: the vault's deletion, move and renames, the background quiet
//	    before each.
//	perf transfer -prepared … -records … -notebook vault -container $CONTAINER -storage $STORAGE -import <zip> | -export <zip>
//	    B8: the import and the export, their jobs and their footprint.
//
// Each driver writes a JSON line a request to -records (3.4.4) and a
// summary to its standard output. The tests of each package run without a
// network or Docker, in test-go-costs-3's go -C server/tools test.
package main
