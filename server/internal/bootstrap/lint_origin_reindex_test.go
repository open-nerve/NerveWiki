package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
	"uuid"

	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

const originDocument = "---\n\"\": '[[Target]]'\nnamed: '[[Target]]'\n---\n[[Target]]\n"

type originReindexFixture struct {
	acmeTeam
	nb, target, source, later, laterSource string
}

func newOriginReindex(t *testing.T, all bool) originReindexFixture {
	t.Helper()
	tm := newAcmeTeam(t, "member", "") // SearchInterval is zero: no consumer of the queue.
	r := originReindexFixture{acmeTeam: tm}
	r.nb = tm.openNotebook(t, "alice", "Eng")
	r.target = tm.createPage(t, "alice", r.nb, "", "Target")
	r.source = tm.createPageWith(t, "alice", r.nb, "", "Source", originDocument)
	if all {
		r.later = tm.openNotebook(t, "alice", "Ops")
		r.laterSource = tm.createPageWith(t, "alice", r.later, "", "Later", originDocument)
		if r.nb >= r.later {
			t.Fatal("continuation witness must follow the failed notebook by id")
		}
	}
	tm.originLegacy(t, r.target, r.source)
	if all {
		tm.originLegacy(t, r.laterSource)
	}
	tm.originExec(t, `UPDATE nodes SET name_key='old-target-key' WHERE id=$1`, r.target)
	tm.originExec(t, `DELETE FROM page_search_queue`)
	return r
}

func (r originReindexFixture) command(t *testing.T, all bool) (string, string, error) {
	t.Helper()
	id := uuid.MustParse(r.nb)
	if all {
		id = uuid.Nil()
	}
	var stdout, stderr bytes.Buffer
	err := Reindex(t.Context(), testConfig(t, r.url, false), &stderr, &stdout, id)
	return stdout.String(), stderr.String(), err
}

func (r originReindexFixture) contents(t *testing.T) []string {
	t.Helper()
	out := queryStrings(t, r.pool, `SELECT to_jsonb(c)::text FROM page_contents c JOIN nodes n ON n.id=c.node_id WHERE n.notebook_id=$1 ORDER BY n.id`, r.nb)
	return append(out, queryStrings(t, r.pool, `SELECT to_jsonb(v)::text FROM page_revisions v JOIN nodes n ON n.id=v.node_id WHERE n.notebook_id=$1 ORDER BY v.id`, r.nb)...)
}

func (r originReindexFixture) keys(t *testing.T) []string {
	t.Helper()
	return queryStrings(t, r.pool, `SELECT json_build_array(id,name,name_key)::text FROM nodes WHERE notebook_id=$1 ORDER BY id`, r.nb)
}

func (r originReindexFixture) queued(t *testing.T, nb string) []string {
	t.Helper()
	return queryStrings(t, r.pool, `SELECT node_id::text FROM page_search_queue WHERE notebook_id=$1 ORDER BY node_id`, nb)
}

func (r originReindexFixture) rebuilt(t *testing.T) {
	t.Helper()
	r.originVersion(t, r.target, 2)
	r.originVersion(t, r.source, 2)
	got := queryStrings(t, r.pool, `SELECT json_build_array(property_key, resolved_id)::text FROM page_links WHERE source_id=$1 ORDER BY range_start`, r.source)
	want := []string{fmt.Sprintf(`["", "%s"]`, r.target), fmt.Sprintf(`["named", "%s"]`, r.target), fmt.Sprintf(`[null, "%s"]`, r.target)}
	if !slices.Equal(got, want) {
		t.Errorf("rebuilt empty/named/body origins = %q, want %q", got, want)
	}
	if got := queryStrings(t, r.pool, `SELECT name_key FROM nodes WHERE id=$1`, r.target); !slices.Equal(got, []string{"target"}) {
		t.Errorf("rekey not committed: %q", got)
	}
}

func (r originReindexFixture) continued(t *testing.T, stdout string) {
	t.Helper()
	if !strings.Contains(stdout, "notebook "+r.later+": 1 page, 3 links, 3 unresolved; 1 node queued for the search\n") {
		t.Errorf("all mode did not report later notebook success: %q", stdout)
	}
	r.originVersion(t, r.laterSource, 2)
	if got := r.queued(t, r.later); !slices.Equal(got, []string{r.laterSource}) {
		t.Errorf("later notebook queue = %v", got)
	}
	if n := count(t, r.pool, `SELECT count(*) FROM page_links WHERE source_id=$1 AND property_key=''`, r.laterSource); n != 1 {
		t.Errorf("later notebook empty origin = %d", n)
	}
}

// The listener is a separate connection, attached only after fixtures.
// A committed marker on the same channel closes the observation window;
// an absent event is not inferred from a sleep or a rolled-back NOTIFY.
func (tm acmeTeam) originListen(t *testing.T) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(t.Context(), tm.url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	if _, err := c.Exec(t.Context(), `LISTEN nwiki_events`); err != nil {
		t.Fatal(err)
	}
	return c
}

func (tm acmeTeam) originEvents(t *testing.T, c *pgx.Conn, want ...string) {
	t.Helper()
	marker := "origin-end-" + uuid.NewV7().String()
	tm.originExec(t, `SELECT pg_notify('nwiki_events',$1)`, marker)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var got []string
	for {
		n, err := c.WaitForNotification(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n.Payload == marker {
			break
		}
		if !strings.Contains(n.Payload, `"type":"links"`) {
			t.Fatalf("unexpected event after command: %s", n.Payload)
		}
		matched := ""
		for _, nb := range want {
			if strings.Contains(n.Payload, nb) {
				matched = nb
				break
			}
		}
		if matched == "" || !strings.Contains(n.Payload, `"data":{"pages":null,"targets":null,"aliases":false}`) {
			t.Fatalf("unexpected reindex event: %s; want %v", n.Payload, want)
		}
		got = append(got, matched)
	}
	if !slices.Equal(got, want) {
		t.Errorf("committed linking events = %v, want %v", got, want)
	}
}

// originSeedEmptyPages preserves the ordinary SQL seed's nodes, contents,
// history and facts, with explicit ordered ids for the 10000-row boundary.
// Literal 1 is truthful for these empty old documents. No command runs
// inside this setup transaction, and the production batch size is intact.
func (r originReindexFixture) seedEmptyPages(t *testing.T, n int) []string {
	t.Helper()
	ids := make([]string, n)
	names := make([]string, n)
	for i := range n {
		ids[i] = fmt.Sprintf("00000000-0000-7000-8000-%012x", i+1)
		names[i] = fmt.Sprintf("seed %05d", i+1)
	}
	tx, err := r.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	changeset := uuid.NewV7()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO changesets (id,notebook_id,kind,client,created_by_id,created_at,updated_at) SELECT $1,id,'edit','web',created_by_id,now(),now() FROM notebooks WHERE id=$2`, []any{changeset, r.nb}},
		{`INSERT INTO nodes (id,notebook_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at)
   SELECT u.id::uuid,b.id,'page',u.name,u.name,1000+u.n,b.created_by_id,b.created_by_id,now(),now()
   FROM unnest($1::text[],$2::text[]) WITH ORDINALITY u(id,name,n),notebooks b WHERE b.id=$3`, []any{ids, names, r.nb}},
		{`INSERT INTO page_contents (node_id,content,revision,content_hash,byte_size,updated_by_id,updated_at)
   SELECT id,'',1,sha256(''),0,created_by_id,now() FROM nodes WHERE id=ANY($1::uuid[])`, []any{ids}},
		{`INSERT INTO page_revisions (id,changeset_id,node_id,revision,content,content_hash,byte_size,created_at,updated_at)
   SELECT gen_random_uuid(),$1,node_id,1,content,content_hash,byte_size,now(),now() FROM page_contents WHERE node_id=ANY($2::uuid[])`, []any{changeset, ids}},
		{`INSERT INTO changeset_items (id,changeset_id,node_id,after_name,after_sort_order,revision,created_at,updated_at)
   SELECT gen_random_uuid(),$1,id,name,sort_order,1,now(),now() FROM nodes WHERE id=ANY($2::uuid[])`, []any{changeset, ids}},
		{`INSERT INTO indexed_pages (node_id,notebook_id,revision,extractor,frontmatter_valid) SELECT id,notebook_id,1,1,true FROM nodes WHERE id=ANY($1::uuid[])`, []any{ids}},
	} {
		if _, err := tx.Exec(t.Context(), q.sql, q.args...); err != nil {
			t.Fatalf("%s: %v", q.sql, err)
		}
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestTheLintOriginReindex(t *testing.T) {
	t.Run("literal-v1", func(t *testing.T) {
		r := newOriginReindex(t, false)
		before := r.contents(t)
		listener := r.originListen(t)
		out, errout, err := r.command(t, false)
		if err != nil || errout != "" || out != fmt.Sprintf("notebook %s: 2 pages, 3 links, 0 unresolved; 2 nodes queued for the search\n", r.nb) {
			t.Fatalf("reindex = %v, %q, %q", err, out, errout)
		}
		r.rebuilt(t)
		if got := r.contents(t); !reflect.DeepEqual(got, before) {
			t.Error("reindex changed content/revisions/hash/history")
		}
		if got := r.queued(t, r.nb); !slices.Equal(got, []string{r.target, r.source}) {
			t.Errorf("queue = %v", got)
		}
		r.originEvents(t, listener, r.nb)
		t.Run("locks-end-before-queue", testOriginReindexLocks)
	})
	for _, scenario := range []string{"linking-fails-queue-succeeds", "clash-queues", "linking-succeeds-queue-fails", "queue-partial-commit"} {
		t.Run(scenario, func(t *testing.T) {
			for _, mode := range []string{"single", "all"} {
				t.Run(mode, func(t *testing.T) {
					all := mode == "all"
					r := newOriginReindex(t, all)
					wantQueue := []string{r.target, r.source}
					fault := "origin linking fault after writes"
					switch scenario {
					case "linking-fails-queue-succeeds":
						r.originExec(t, fmt.Sprintf(`CREATE FUNCTION origin_fault() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
       IF NEW.source_id='%s' AND NEW.range_start=%d THEN
        IF NOT EXISTS (SELECT 1 FROM page_links WHERE source_id=NEW.source_id AND property_key='')
         OR (SELECT count(*) FROM page_links WHERE source_id=NEW.source_id) <> 2
         OR (SELECT name_key FROM nodes WHERE id='%s') <> 'target'
         OR (SELECT extractor FROM indexed_pages WHERE node_id='%s') <> 2
        THEN RAISE EXCEPTION 'origin fixture did not reach partial linking writes'; END IF;
        RAISE EXCEPTION 'origin linking fault after writes';
       END IF; RETURN NEW; END $$;
       CREATE TRIGGER origin_fault BEFORE INSERT ON page_links FOR EACH ROW EXECUTE FUNCTION origin_fault()`, r.source, strings.LastIndex(originDocument, "Target"), r.target, r.target))
					case "clash-queues":
						// Two existing pages get names whose new Unicode keys collide,
						// while their legal old keys and nonempty facts remain as before.
						r.originExec(t, `UPDATE nodes SET name='Straße',name_key='old-strasse' WHERE id=$1`, r.target)
						r.originExec(t, `UPDATE nodes SET name='STRASSE',name_key='strasse' WHERE id=$1`, r.source)
						fault = "whose titles would share a key"
					case "linking-succeeds-queue-fails":
						fault = "origin queue fault after first row"
						wantQueue = nil
						r.originExec(t, fmt.Sprintf(`CREATE FUNCTION origin_fault() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
       IF NEW.node_id='%s' THEN
        IF NOT EXISTS (SELECT 1 FROM page_search_queue WHERE node_id='%s') THEN RAISE EXCEPTION 'origin queue first row was not written'; END IF;
        RAISE EXCEPTION 'origin queue fault after first row';
       END IF; RETURN NEW; END $$;
       CREATE TRIGGER origin_fault BEFORE INSERT ON page_search_queue FOR EACH ROW EXECUTE FUNCTION origin_fault()`, r.source, r.target))
					case "queue-partial-commit":
						ids := r.seedEmptyPages(t, 10002)
						wantQueue = ids[:10000]
						fault = "origin queue fault in second batch"
						r.originExec(t, fmt.Sprintf(`CREATE FUNCTION origin_fault() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
       IF NEW.node_id='%s' THEN
        IF NOT EXISTS (SELECT 1 FROM page_search_queue WHERE node_id='%s')
         OR (SELECT count(*) FROM page_search_queue WHERE notebook_id=NEW.notebook_id) <> 10001
        THEN RAISE EXCEPTION 'origin queue did not write a complete first batch and part of second'; END IF;
        RAISE EXCEPTION 'origin queue fault in second batch';
       END IF; RETURN NEW; END $$;
       CREATE TRIGGER origin_fault BEFORE INSERT ON page_search_queue FOR EACH ROW EXECUTE FUNCTION origin_fault()`, ids[10001], ids[10000]))
					}
					beforeIndex, beforeKeys, beforeContents := r.originIndex(t, r.nb), r.keys(t), r.contents(t)
					listener := r.originListen(t)
					out, errout, err := r.command(t, all)
					if err == nil || !strings.Contains(fmt.Sprint(err)+errout, fault) {
						t.Fatalf("%s %s = %v, %q, %q; want %q", scenario, mode, err, out, errout, fault)
					}
					if all {
						r.continued(t, out)
					} else if out != "" {
						t.Errorf("failed single command output = %q", out)
					}
					if got := r.contents(t); !reflect.DeepEqual(got, beforeContents) {
						t.Error("command changed content/revisions/hash/history")
					}
					// The command's own pool has closed; this connection observes
					// committed state, not its transaction or an enclosing test tx.
					if got := r.queued(t, r.nb); !slices.Equal(got, wantQueue) {
						t.Errorf("committed queue = %d IDs, want %d (exact id sets differ)", len(got), len(wantQueue))
					}
					if n := count(t, r.pool, `SELECT count(*) FROM page_search_queue WHERE notebook_id=$1 AND NOT bulk`, r.nb); n != 0 {
						t.Errorf("reindex queued %d nonbulk rows", n)
					}
					var events []string
					if scenario == "linking-fails-queue-succeeds" || scenario == "clash-queues" {
						if got := r.originIndex(t, r.nb); !reflect.DeepEqual(got, beforeIndex) {
							t.Error("failed linking stage left partial derived facts")
						}
						if got := r.keys(t); !reflect.DeepEqual(got, beforeKeys) {
							t.Error("failed linking stage left partial rekey")
						}
						r.originVersion(t, r.source, 1)
					} else {
						r.rebuilt(t)
						events = append(events, r.nb)
					}
					if all {
						events = append(events, r.later)
					}
					r.originEvents(t, listener, events...)
				})
			}
		})
	}
	t.Run("tokenizer-eligibility", func(t *testing.T) {
		r := newOriginReindex(t, false)
		// All API setup must precede the deliberate old-format conversion.
		// The seed helper adds two valid empty old pages without checkLinks.
		ids := r.seedEmptyPages(t, 2)
		current := "5/" + unicode.Version + "/" + norm.Version
		for id, tokenizer := range map[string]string{r.target: "0/0.0.0", ids[0]: current, ids[1]: current} {
			r.originExec(t, `INSERT INTO page_search (node_id,revision,name,tokenizer,built_at,title,body)
    SELECT id,1,name,$2,now(),''::tsvector,''::tsvector FROM nodes WHERE id=$1`, id, tokenizer)
		}
		r.originExec(t, `INSERT INTO page_search_queue(node_id,notebook_id,bulk) VALUES($1,$2,false)`, ids[1], r.nb)
		queueBefore := queryStrings(t, r.pool, `SELECT to_jsonb(q)::text FROM page_search_queue q WHERE node_id=$1`, ids[1])
		searchBefore := queryStrings(t, r.pool, `SELECT to_jsonb(s)::text FROM page_search s ORDER BY node_id`)
		out, errout, err := r.command(t, false)
		if err != nil || errout != "" || !strings.Contains(out, "2 nodes queued for the search") {
			t.Fatalf("tokenizer command = %v, %q, %q", err, out, errout)
		}
		if got := r.queued(t, r.nb); !slices.Equal(got, []string{ids[1], r.target, r.source}) {
			t.Errorf("missing/old/current eligible queue: %v", got)
		}
		if got := queryStrings(t, r.pool, `SELECT to_jsonb(q)::text FROM page_search_queue q WHERE node_id=$1`, ids[1]); !reflect.DeepEqual(got, queueBefore) {
			t.Error("current tokenizer's preexisting queue/version/bulk changed")
		}
		if got := queryStrings(t, r.pool, `SELECT to_jsonb(s)::text FROM page_search s ORDER BY node_id`); !reflect.DeepEqual(got, searchBefore) {
			t.Error("queueing rebuilt or changed search rows")
		}
		if n := count(t, r.pool, `SELECT count(*) FROM page_search_queue WHERE node_id=ANY($1::uuid[]) AND bulk`, []string{r.target, r.source}); n != 2 {
			t.Errorf("missing/old bulk rows = %d", n)
		}
		r.rebuilt(t)
	})
}

// A real row trigger holds linking after its first write. A content
// writer is observed waiting at the notebook; the index lock is held too.
// A second trigger then holds the later queue statement, when linking's
// committed facts are visible and both of its locks can be taken anew.
func testOriginReindexLocks(t *testing.T) {
	r := newOriginReindex(t, false)
	// This writer's search row is current, so Requeue does not lock its
	// queue entry. It can finish while the independent queue barrier holds.
	writer := r.seedEmptyPages(t, 1)[0]
	r.originExec(t, `INSERT INTO page_search (node_id,revision,name,tokenizer,built_at,title,body)
  SELECT id,1,name,$2,now(),''::tsvector,''::tsvector FROM nodes WHERE id=$1`, writer, "5/"+unicode.Version+"/"+norm.Version)
	r.originExec(t, fmt.Sprintf(`CREATE TABLE origin_pause(id int PRIMARY KEY); INSERT INTO origin_pause VALUES(1),(2);
  CREATE FUNCTION origin_pause_link() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
   IF NEW.source_id='%s' AND NEW.range_start=11 THEN PERFORM 1 FROM origin_pause WHERE id=1 FOR SHARE; END IF; RETURN NEW; END $$;
  CREATE TRIGGER origin_pause_link AFTER INSERT ON page_links FOR EACH ROW EXECUTE FUNCTION origin_pause_link();
  CREATE FUNCTION origin_pause_queue() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
   IF NEW.node_id='%s' THEN PERFORM 1 FROM origin_pause WHERE id=2 FOR SHARE; END IF; RETURN NEW; END $$;
  CREATE TRIGGER origin_pause_queue BEFORE INSERT ON page_search_queue FOR EACH ROW EXECUTE FUNCTION origin_pause_queue()`, r.source, r.source))
	hold := func(id int) pgx.Tx {
		tx, err := r.pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
		if _, err := tx.Exec(t.Context(), `SELECT 1 FROM origin_pause WHERE id=$1 FOR UPDATE`, id); err != nil {
			t.Fatal(err)
		}
		return tx
	}
	linking, queue := hold(1), hold(2)
	cfg := testConfig(t, r.url, false)
	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { done <- Reindex(ctx, cfg, &stderr, &stdout, uuid.MustParse(r.nb)) }()
	pgtest.WaitForLockWaitsOn(t, r.pool, "origin_pause", 1, interleavingWait)
	id := uuid.MustParse(r.nb)
	h := fnv.New32a()
	_, _ = h.Write(id[:])
	space, key := int32(0x6c696e6b), int32(h.Sum32())
	probe := func(want bool) {
		t.Helper()
		tx, err := r.pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		var available bool
		if err := tx.QueryRow(t.Context(), `SELECT pg_try_advisory_xact_lock($1,$2)`, space, key).Scan(&available); err != nil {
			t.Fatal(err)
		}
		if available != want {
			t.Errorf("index lock available = %t, want %t", available, want)
		}
		if want {
			if _, err := tx.Exec(t.Context(), `SELECT 1 FROM notebooks WHERE id=$1 FOR NO KEY UPDATE NOWAIT`, r.nb); err != nil {
				t.Fatalf("notebook lock extended into queue: %v", err)
			}
		}
	}
	probe(false)
	send := r.sender(t, contentWrite("alice", writer, "writer", 1, ""))
	wrote := make(chan answer, 1)
	go func() { wrote <- send() }()
	pgtest.WaitForLockWaitsOn(t, r.pool, "notebooks", 1, interleavingWait)
	if err := linking.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	pgtest.WaitForLockWaitsOn(t, r.pool, "origin_pause", 1, interleavingWait)
	select {
	case a := <-wrote:
		if a.status != 200 {
			t.Fatalf("writer = %d %s, %v", a.status, a.body, a.err)
		}
	case <-time.After(interleavingWait):
		t.Fatal("writer did not finish after linking committed")
	}
	probe(true)
	r.originVersion(t, r.source, 2)
	r.wrote(t, r.source, originDocument, 1)
	if err := queue.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("reindex = %v; %s", err, stderr.String())
		}
	case <-time.After(interleavingWait):
		t.Fatal("reindex did not finish after queue barrier")
	}
}
