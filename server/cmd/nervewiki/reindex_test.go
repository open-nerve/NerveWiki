package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// reindexed is a database as an upgrade to M6 leaves it: notebooks with
// pages and their contents, no index. eng has Note, linking to Other and
// to nothing; ops has Plan; gone is deleted.
type reindexed struct {
	environ                []string
	pool                   *pgxpool.Pool
	eng, ops, gone         uuid.UUID
	note, other, plan, old uuid.UUID
}

func newReindexed(t *testing.T) reindexed {
	t.Helper()
	environ, pool := usersDatabase(t)
	ids := []uuid.UUID{uuid.NewV7(), uuid.NewV7(), uuid.NewV7()}
	slices.SortFunc(ids, uuid.UUID.Compare)
	r := reindexed{environ: environ, pool: pool, eng: ids[0], ops: ids[1], gone: ids[2],
		note: uuid.NewV7(), other: uuid.NewV7(), plan: uuid.NewV7(), old: uuid.NewV7()}
	user, acme := uuid.NewV7(), uuid.NewV7()
	r.exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'ada@corp.com', 'x', 'Ada', now(), now())", user)
	r.exec(t, "INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())", acme, user)
	for _, nb := range []uuid.UUID{r.eng, r.ops, r.gone} {
		r.exec(t, `INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			VALUES ($1, $2, 'Notes', $3, $3, now(), now(), CASE WHEN $4 THEN now() END)`, nb, acme, user, nb == r.gone)
	}
	for _, p := range []struct {
		id, notebook uuid.UUID
		name         string
		content      string
	}{
		{r.note, r.eng, "Note", "[[Other]] [[Nowhere]]"},
		{r.other, r.eng, "Other", ""},
		{r.plan, r.ops, "Plan", "[[Plan]]"},
		{r.old, r.gone, "Old", "[[Old]]"},
	} {
		r.exec(t, `INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
			VALUES ($1, $2, 'page', $3, $4, 0, $5, $5, now(), now())`, p.id, p.notebook, p.name, shared.TitleKey(p.name), user)
		r.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
			VALUES ($1, $2, 2, sha256(convert_to($2, 'UTF8')), octet_length($2), $3, now())`, p.id, p.content, user)
	}
	return r
}

func (r reindexed) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := r.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (r reindexed) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// indexed is how many pages of the notebook are indexed, at their
// contents' revision.
func (r reindexed) indexed(t *testing.T, nb uuid.UUID) int {
	t.Helper()
	return r.count(t, "SELECT count(*) FROM indexed_pages WHERE notebook_id = $1 AND revision = 2", nb)
}

// nervewiki reindex rebuilds one notebook's index, or every notebook's not
// deleted, by id, a line each, and publishes a links event of each; the
// links resolve.
func TestReindexRebuildsTheNotebooks(t *testing.T) {
	r := newReindexed(t)
	ctx := context.Background()
	listener, err := r.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Release()
	if _, err := listener.Exec(ctx, "LISTEN nwiki_events"); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := execute(ctx, r.environ, "reindex", "--notebook", r.eng.String())
	if want := fmt.Sprintf("notebook %s: 2 pages, 2 links, 1 unresolved\n", r.eng); code != 0 || stdout != want {
		t.Fatalf("reindex --notebook eng = %d, %q, %s; want %q", code, stdout, stderr, want)
	}
	if r.indexed(t, r.eng) != 2 || r.indexed(t, r.ops) != 0 {
		t.Errorf("indexed %d pages of eng, %d of ops; want eng's two alone", r.indexed(t, r.eng), r.indexed(t, r.ops))
	}
	if n := r.count(t, "SELECT count(*) FROM page_links WHERE source_id = $1 AND resolved_id = $2", r.note, r.other); n != 1 {
		t.Errorf("Note's link to Other resolves %d times, want once", n)
	}
	expectLinksEvent(t, listener.Conn(), r.eng)

	code, stdout, stderr = execute(ctx, r.environ, "reindex")
	want := fmt.Sprintf("notebook %s: 2 pages, 2 links, 1 unresolved\nnotebook %s: 1 page, 1 link, 0 unresolved\n", r.eng, r.ops)
	if code != 0 || stdout != want {
		t.Fatalf("reindex = %d, %q, %s; want %q", code, stdout, stderr, want)
	}
	if r.indexed(t, r.ops) != 1 || r.indexed(t, r.gone) != 0 {
		t.Errorf("indexed %d pages of ops, %d of the deleted notebook; want ops's one alone", r.indexed(t, r.ops), r.indexed(t, r.gone))
	}
	expectLinksEvent(t, listener.Conn(), r.eng)
	expectLinksEvent(t, listener.Conn(), r.ops)
}

// expectLinksEvent fails t unless the next notification is a links event
// of the notebook nb that lists no page.
func expectLinksEvent(t *testing.T, conn *pgx.Conn, nb uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n, err := conn.WaitForNotification(ctx)
	if err != nil {
		t.Fatalf("no links event of %s: %v", nb, err)
	}
	if !strings.Contains(n.Payload, `"type":"links"`) || !strings.Contains(n.Payload, nb.String()) ||
		!strings.Contains(n.Payload, `"data":{"pages":null,"targets":null}`) {
		t.Errorf("the event %s, want a links event of %s listing no page", n.Payload, nb)
	}
}

// A notebook whose siblings' title keys would clash under the current
// Unicode data is left as it was, its rows and keys, with no links event,
// and listed on stderr, as is one whose rebuild fails; the others are
// rebuilt, and the command fails; a --notebook that is not a notebook's
// id fails at once.
func TestReindexReportsClashesAndFailuresAndRefusesNoNotebook(t *testing.T) {
	r := newReindexed(t)
	ctx := context.Background()
	if code, _, stderr := execute(ctx, r.environ, "reindex"); code != 0 {
		t.Fatalf("reindex = %d, %s", code, stderr)
	}
	twin := uuid.NewV7()
	r.exec(t, "UPDATE nodes SET name = 'Straße', name_key = 'old' WHERE id = $1", r.plan)
	r.exec(t, `INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
		SELECT $1, notebook_id, 'page', 'STRASSE', 'strasse', 1, created_by_id, created_by_id, now(), now() FROM nodes WHERE id = $2`, twin, r.plan)
	r.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT $1, '', 2, sha256(''), 0, created_by_id, now() FROM nodes WHERE id = $1`, twin)
	r.exec(t, "UPDATE nodes SET parent_id = $1 WHERE id = $2", r.other, r.note)
	r.exec(t, "UPDATE nodes SET parent_id = $1 WHERE id = $2", r.note, r.other)
	listener, err := r.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Release()
	if _, err := listener.Exec(ctx, "LISTEN nwiki_events"); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := execute(ctx, r.environ, "reindex")
	clash := fmt.Sprintf(`notebook %s: not reindexed: titles that would share a key: "Straße" (%s), "STRASSE" (%s)`, r.ops, r.plan, twin)
	failure := fmt.Sprintf("notebook %s: not reindexed: ", r.eng)
	if code != 1 || stdout != "" || !strings.Contains(stderr, clash+"\n") || !strings.Contains(stderr, failure) ||
		!strings.Contains(stderr, "nervewiki: 2 notebooks not reindexed") {
		t.Errorf("reindex with a clash and a loop = %d, %q, %q; want 1, no line, and %q", code, stdout, stderr, clash)
	}
	if n := r.count(t, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND name_key = 'old'", r.ops); n != 1 || r.indexed(t, r.ops) != 1 ||
		r.count(t, "SELECT count(*) FROM page_links WHERE source_id = $1 AND resolved_id = $1", r.plan) != 1 {
		t.Errorf("ops after a clash: %d stale keys, %d pages indexed; want it as it was", n, r.indexed(t, r.ops))
	}
	if r.indexed(t, r.eng) != 2 {
		t.Errorf("eng after its rebuild failed: %d pages indexed; want its two kept", r.indexed(t, r.eng))
	}
	wait, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if n, err := listener.Conn().WaitForNotification(wait); err == nil {
		t.Errorf("a notebook not reindexed published %s", n.Payload)
	}

	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"--notebook", "x"}, `nervewiki: --notebook "x" is not a notebook id` + "\n"},
		{[]string{"--notebook", ""}, `nervewiki: --notebook "" is not a notebook id` + "\n"},
		{[]string{"--notebook", uuid.Nil().String()}, fmt.Sprintf("nervewiki: --notebook %q is not a notebook id\n", uuid.Nil())},
		{[]string{"--notebook", r.gone.String()}, fmt.Sprintf("nervewiki: no notebook %s\n", r.gone)},
		{[]string{"--notebook", r.eng.String()}, fmt.Sprintf("nervewiki: notebook %s: ", r.eng)},
	} {
		code, stdout, stderr := execute(ctx, r.environ, append([]string{"reindex"}, tt.args...)...)
		if code != 1 || stdout != "" || !strings.Contains(stderr, tt.want) {
			t.Errorf("reindex %v = %d, %q, %q; want 1 and %q", tt.args, code, stdout, stderr, tt.want)
		}
	}
}
