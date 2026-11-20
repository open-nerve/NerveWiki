// Command perf is M12/P4's measuring tools (docs/v0.1/M12-release/
// 04-P4-performance.md, section 3.4): a serve's and its database's
// memory and CPU, the vault an import takes, and the drivers of the
// measurements a browser or a composition root's test cannot take. It
// uses the standard library alone: it reads the database through psql in
// its container, and speaks MCP's JSON-RPC by hand (design 2.3's P4-6).
//
// A round starts a production serve over a PostgreSQL container of its
// own with stack.sh (3.4.1; restart keeps the data for batch E's second
// round, down removes the container), in a directory of its own (up
// refuses one that is not empty: its serve.log and data/ are the round's),
// then prepares the accounts:
//
//	server/tools/perf/stack.sh up <dir> [--fsync-off] [--env KEY=VALUE ...]
//	. <dir>/stack.env
//	PERF_PASSWORD=$(cat <dir>/password) perf prepare -base $BASE -notebooks vault,tree -pats 50 -out <dir>/pats.json
//
// The subcommands, and the rows of the design they serve:
//
//	perf sample -pid $SERVE_PID -container $CONTAINER -out <csv>
//	    3.4.2: a row every 100 ms of the serve's VmRSS, VmHWM and CPU and
//	    the container's memory.current, CPU and memory.stat's anon, shmem
//	    and file (memory.current is mostly a PostgreSQL's page cache), the
//	    container's memory.peak and memory.stat at SIGINT with the rows'
//	    largest anon, shmem and file; the serve's start in the file's first
//	    line, for gctrace's -start. file holds shmem: the page cache the
//	    kernel can take back is about file - shmem, what it cannot is
//	    anon + shmem; the three do not add up (review B's N7).
//	perf gctrace -log <dir>/serve.log -start <ns> -base <t> -from <t> -to <t>
//	    3.4.2: the round's baseline, a combination's peak, its delta, its
//	    upper bound and the collections missing; times in Unix nanoseconds
//	    or RFC 3339. The baseline is the live heap of the first collection
//	    at or after -base: the one the runtime forces two minutes into the
//	    round's idle of 130 s, which the log has after a line "GC forced"
//	    (the line itself does not say forced). Take -base at the idle's
//	    start, not after -from: the first collection after it is the forced
//	    one unless the serve collected on its own while idle;
//	    base_forced=true says the line read is the forced one, false that
//	    it is not (then -base just before the "GC forced" line's
//	    collection, its @T from -start). A window without a collection
//	    says no_gc_in_window: its peak and delta are none, its upper bound
//	    the next collection's goal.
//	perf vault -out <zip> -pages 33750 -attachments 3750 -fanout 10,10,73 -links 30 -body 2KiB -attachment-bytes 800KiB -random 12 -seed 1201 [-part k/n]
//	    3.4.3: the import-limit vault (batches F and B4), 44,910 nodes
//	    within an import's 45,000 (3.12; S3's was -pages 45000
//	    -attachments 5000 -fanout 10,10,98, 59,910 nodes, before the
//	    limit came down). -part k/n writes the k-th of n parts of a
//	    vault, its folders of the first level cut in n runs: the parts
//	    imported one after another under one page are the whole vault
//	    imported. S3's vault in -part 1/2 and 2/2 is 34,355 and 25,555
//	    nodes (P4/S7's top of 59,910); the 100,000 pages of batch E are
//	    two vaults of 50,510 nodes, -pages 50000 -fanout 10,50 -links 10
//	    -body 1KiB -seed 2001 and 2002, each in -part 1/2 and 2/2 of
//	    25,255 nodes.
//	perf bodies -prepared <dir>/pats.json -records <jsonl> -k 8 -kind normal|pathological [-same-notebook]
//	    B2: readers and writers of 5 MiB pages; the busy answers early.
//	perf mcp -prepared … -records … -shape 1|2a|2b|2c|3|4 -reps 3 -gap 20s
//	    B3: three credentials' large calls at once.
//	perf editors -prepared … -records … -n 20 -for 120s -container $CONTAINER -monitor <jsonl> [-heavy-links 10000]
//	    3.7's M4-performance 1 and 6, and batch G's bytes a version.
//	perf tabs -prepared … -records … -k 4 -for 60s -notebook tree
//	    3.7's reread of a tree of 100,000 pages.
//	perf ops -prepared … -records … -notebook vault -container $CONTAINER
//	    B4: the vault's deletion, its move to another page and its renames
//	    to a new name, each a change, the background quiet before each;
//	    then, not timed, the vault back at the root and its names back.
//	perf transfer -prepared … -records … -notebook vault -container $CONTAINER -storage $STORAGE -import <zip> | -export <zip>
//	    B8: the import and the export, their jobs and their footprint.
//
// Each driver writes a JSON line a request to -records (3.4.4) and a
// summary to its standard output. A run of -for stops sending at its end
// and waits for the requests in flight, 60 s at most (drive.DefaultDrain,
// which no flag changes; the serve's own deadline is 15 s), so that its
// times are not those of requests cut short; past its end a loop sends
// nothing but an editor's session close. The summary times the requests
// answered whole, and counts apart those cut (cut=, with cut_longest=,
// how long the longest had run) and those the transport failed (errors=),
// which also make the command fail.
// The tests of each package run without a network or Docker, in
// test-go-costs-3's go -C server/tools test.
package main
