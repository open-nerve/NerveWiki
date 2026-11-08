package postgresadapter_test

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// An upgrade's rows the deletion guardrail reads (M11/P1 design 3.4):
// those of the ids asked that have a row of the upgrade, its baseline's
// and its protected source root's, in the unit's transaction; another
// upgrade's rows and nodes with none are not.
func TestUpgradeProtected(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	upgrade, other := uuid.NewV7(), uuid.NewV7()
	page, asset, root, others, fresh := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash) VALUES
		($1, $3, $7, true, 'page', NULL, 'Guide', 0, 1, sha256('x'), 1, 0, sha256('x')),
		($1, $4, $7, true, 'asset', $3, 'a.png', 1, NULL, NULL, NULL, NULL, NULL),
		($1, $5, $7, false, 'page', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL),
		($2, $6, $8, true, 'asset', NULL, 'b.png', 0, NULL, NULL, NULL, NULL, NULL)`,
		upgrade, other, page, asset, root, others, f.eng, f.ops)
	want := map[uuid.UUID]bool{page: true, asset: true, root: true}
	if err := postgres.NewTxManager(f.pool, 5*time.Second).WithinTx(ctx, func(ctx context.Context) error {
		got, err := f.s.Protected(ctx, upgrade, []uuid.UUID{page, asset, root, others, fresh})
		if err != nil {
			return err
		}
		if !maps.Equal(got, want) {
			t.Errorf("Protected = %v, want the upgrade's three rows %v", got, want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got, err := f.s.Protected(ctx, upgrade, []uuid.UUID{fresh}); err != nil || len(got) != 0 {
		t.Errorf("Protected of a node with no row = %v, %v; want none", got, err)
	}
}

// A changeset is written with the upgrade it is of, NULL for none (M11/P1
// design 3.4).
func TestCreateChangesetWritesItsUpgrade(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	upgrade := uuid.NewV7()
	of, none := uuid.NewV7(), uuid.NewV7()
	for id, u := range map[uuid.UUID]uuid.UUID{of: upgrade, none: {}} {
		if err := f.s.CreateChangeset(ctx, app.Changeset{ID: id, NotebookID: f.eng, Kind: domain.ChangesetEdit,
			Client: domain.MCPClient("claude-code"), By: f.alice, At: now(), UpgradeID: u}); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM changesets WHERE id = $1 AND upgrade_id = $2", of, upgrade); n != 1 {
		t.Errorf("the upgrade's changeset has upgrade_id %d times, want once", n)
	}
	if n := f.count(t, "SELECT count(*) FROM changesets WHERE id = $1 AND upgrade_id IS NULL", none); n != 1 {
		t.Errorf("a changeset of no upgrade has upgrade_id NULL %d times, want once", n)
	}
}

// A notebook's deletion deletes its upgrades' rows, baseline and
// protected, by the notebook (M11/P1 design 3.12); another notebook's
// stay.
func TestTheNotebooksDeletionDeletesTheirUpgradesRows(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash) VALUES
		($1, gen_random_uuid(), $3, true, 'page', NULL, 'Guide', 0, 1, sha256('x'), 1, 0, sha256('x')),
		($1, gen_random_uuid(), $3, false, 'page', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL),
		($2, gen_random_uuid(), $4, true, 'asset', NULL, 'b.png', 0, NULL, NULL, NULL, NULL, NULL)`,
		uuid.NewV7(), uuid.NewV7(), f.eng, f.ops)
	if err := f.s.DeleteNotebooksUpgradeNodes(ctx, []uuid.UUID{f.eng}); err != nil {
		t.Fatal(err)
	}
	var eng, ops int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE notebook_id = $1), count(*) FILTER (WHERE notebook_id = $2)
		FROM upgrade_nodes`, f.eng, f.ops).Scan(&eng, &ops); err != nil {
		t.Fatal(err)
	}
	if eng != 0 || ops != 1 {
		t.Errorf("after eng's deletion, eng has %d rows, ops %d; want none, ops' one", eng, ops)
	}
}

// An upgrade's baseline is the notebook as it started (M11/P1 design
// 3.12): CaptureBaseline's one INSERT … SELECT writes exactly the rows a
// read of each live node gives, a page with its latest version and its
// hash accepted, wanting 0, an attachment with none; a deleted node and
// another notebook's are not in it; it answers its nodes and pages.
// SetBaselineWanted sets a baseline page's count and changes no row of an
// attachment, a protected row or an id of none. RootNodesByKey reads the
// live nodes at the root by their keys, none below the root or deleted.
func TestTheBaselineIsTheNotebookAsItStarted(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	guide := f.page(t, f.eng, nil, "Guide", 0)
	raw := f.page(t, f.eng, nil, "raw", 1)
	note := f.page(t, f.eng, &raw.ID, "Note", 0)
	photo := f.asset(t, f.eng, &guide.ID, "photo.png", 1)
	agents := f.asset(t, f.eng, nil, "AGENTS", 2)
	gone := f.page(t, f.eng, nil, "Gone", 3)
	f.exec(t, "UPDATE nodes SET deleted_at = now() WHERE id = $1", gone.ID)
	f.exec(t, "UPDATE page_contents SET content = 'twelve bytes', revision = 7, byte_size = 12, content_hash = sha256('twelve bytes') "+
		"WHERE node_id = $1", note.ID)
	f.page(t, f.ops, nil, "Ops", 0)
	upgrade := uuid.NewV7()
	tx := postgres.NewTxManager(f.pool, 5*time.Second)
	var base app.Baseline
	if err := tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		base, err = f.s.CaptureBaseline(ctx, upgrade, f.eng)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if base != (app.Baseline{Nodes: 5, Pages: 3}) {
		t.Errorf("CaptureBaseline = %+v, want 5 nodes, 3 pages", base)
	}
	// The baseline's rows against a read of each live node, field by field.
	differ := f.count(t, `SELECT count(*) FROM (
		(SELECT node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision, content_hash, byte_size, wanted,
			accepted_hash FROM upgrade_nodes WHERE upgrade_id = $1
		EXCEPT
		SELECT n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision, c.content_hash, c.byte_size,
			CASE WHEN n.kind = 'page' THEN 0 END, c.content_hash
		FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id WHERE n.notebook_id = $2 AND n.deleted_at IS NULL)
		UNION ALL
		(SELECT n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision, c.content_hash, c.byte_size,
			CASE WHEN n.kind = 'page' THEN 0 END, c.content_hash
		FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id WHERE n.notebook_id = $2 AND n.deleted_at IS NULL
		EXCEPT
		SELECT node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision, content_hash, byte_size, wanted,
			accepted_hash FROM upgrade_nodes WHERE upgrade_id = $1)) d`, upgrade, f.eng)
	if differ != 0 {
		t.Errorf("the baseline and the notebook's live nodes differ in %d rows", differ)
	}
	if n := f.count(t, `SELECT count(*) FROM upgrade_nodes WHERE upgrade_id = $1 AND node_id = $2 AND revision = 7 AND byte_size = 12
		AND content_hash = sha256('twelve bytes') AND accepted_hash = content_hash`, upgrade, note.ID); n != 1 {
		t.Errorf("Note's row has its latest version, its hash accepted: %d rows, want 1", n)
	}
	if n := f.count(t, `SELECT count(*) FROM upgrade_nodes WHERE upgrade_id = $1 AND node_id = ANY($2) AND revision IS NULL
		AND content_hash IS NULL AND byte_size IS NULL AND wanted IS NULL AND accepted_hash IS NULL`, upgrade,
		[]uuid.UUID{photo.ID, agents.ID}); n != 2 {
		t.Errorf("the attachments' rows without a version: %d, want 2", n)
	}
	if n := f.count(t, "SELECT count(*) FROM upgrade_nodes WHERE node_id = $1", gone.ID); n != 0 {
		t.Errorf("the deleted page has %d rows, want none", n)
	}

	protected := uuid.NewV7()
	none := uuid.NewV7()
	if err := tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := f.s.ProtectSourceRoot(ctx, upgrade, f.eng, protected); err != nil {
			return err
		}
		return f.s.SetBaselineWanted(ctx, upgrade, []uuid.UUID{guide.ID, note.ID, photo.ID, protected, none}, []int32{3, 1, 4, 5, 9})
	}); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM upgrade_nodes WHERE upgrade_id = $1 AND (node_id = $2 AND wanted = 3 OR node_id = $3
		AND wanted = 1 OR node_id = $4 AND wanted = 0)`, upgrade, guide.ID, note.ID, raw.ID); n != 3 {
		t.Errorf("the baseline pages' wanted set, the others kept: %d rows, want 3", n)
	}
	if n := f.count(t, `SELECT count(*) FROM upgrade_nodes WHERE upgrade_id = $1 AND node_id = ANY($2) AND wanted IS NULL`, upgrade,
		[]uuid.UUID{photo.ID, protected}); n != 2 {
		t.Errorf("the attachment's and the protected row's wanted untouched: %d rows, want 2", n)
	}
	if n := f.count(t, "SELECT count(*) FROM upgrade_nodes WHERE node_id = $1", none); n != 0 {
		t.Errorf("an id of none has %d rows, want none", n)
	}

	got, err := f.s.RootNodesByKey(ctx, f.eng, []string{"raw", "agents", "note", "gone"})
	slices.SortFunc(got, func(a, b app.KeyedRootNode) int { return strings.Compare(a.Key, b.Key) })
	want := []app.KeyedRootNode{{ID: agents.ID, Kind: domain.KindAsset, Key: "agents"}, {ID: raw.ID, Kind: domain.KindPage, Key: "raw"}}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("RootNodesByKey = %+v, %v; want %+v: Note is below the root, Gone deleted", got, err, want)
	}
}

// An upgrade's start's sessions (M11/P1 design 3.12): the notebook's rows
// expired at the start go, tombstones among them, another notebook's and
// those alive stay; the sessions alive are read by page, the first 100,
// with how many there are.
func TestTheNotebooksSessionsAsAnUpgradeStarts(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	at := now().Add(time.Hour)
	pages := f.nodes(t, f.eng, "P", 101)
	opsPage := f.page(t, f.ops, nil, "Ops", 0)
	bob := f.user(t, "bob@corp.com")
	session := func(node, notebook uuid.UUID, user uuid.UUID, expires time.Time, ended bool) uuid.UUID {
		id := uuid.NewV7()
		f.exec(t, `INSERT INTO edit_sessions (id, node_id, notebook_id, user_id, client, created_at, expires_at, ended_reason,
			ended_by_id, ended_at) VALUES ($1, $2, $3, $4, 'web', $5, $6, CASE WHEN $7 THEN 'unlocked' END, CASE WHEN $7 THEN $4::uuid END,
			CASE WHEN $7 THEN $5::timestamptz END)`, id, node, notebook, user, now(), expires, ended)
		return id
	}
	expired := session(pages[0], f.eng, f.alice, at, false)
	tombstone := session(pages[1], f.eng, f.alice, at.Add(-time.Second), true)
	ended := session(pages[2], f.eng, bob, at.Add(time.Minute), true)
	opsExpired := session(opsPage.ID, f.ops, f.alice, at, false)
	for i := range pages {
		session(pages[i], f.eng, []uuid.UUID{f.alice, bob}[i%2], at.Add(time.Second), false)
	}
	n, err := f.s.DeleteExpiredSessionsOfNotebook(ctx, f.eng, at)
	if err != nil || n != 2 {
		t.Errorf("DeleteExpiredSessionsOfNotebook = %d, %v; want the expired and the expired tombstone", n, err)
	}
	if left := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = ANY($1)", []uuid.UUID{expired, tombstone}); left != 0 {
		t.Errorf("%d expired rows left, want none", left)
	}
	if left := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = ANY($1)", []uuid.UUID{ended, opsExpired}); left != 2 {
		t.Errorf("%d of the ended tombstone alive and ops' expired row left, want both", left)
	}
	holders, total, err := f.s.LiveSessionsOfNotebook(ctx, f.eng, at)
	if err != nil || total != 101 || len(holders) != 100 {
		t.Fatalf("LiveSessionsOfNotebook = %d of %d, %v; want 100 of 101", len(holders), total, err)
	}
	byPage := slices.Clone(pages)
	slices.SortFunc(byPage, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	for i, h := range holders {
		if h.PageID != byPage[i] {
			t.Fatalf("holder %d is page %v, want %v: by page", i, h.PageID, byPage[i])
		}
		want := f.alice
		if j := slices.Index(pages, h.PageID); j%2 == 1 {
			want = bob
		}
		if h.UserID != want {
			t.Errorf("page %v's holder is %v, want %v", h.PageID, h.UserID, want)
		}
	}
	if slices.ContainsFunc(holders, func(h app.SessionHolder) bool { return h.PageID == byPage[100] }) {
		t.Error("the 101st page by id is in the list")
	}
	if holders, total, err := f.s.LiveSessionsOfNotebook(ctx, f.ops, at); err != nil || total != 0 || len(holders) != 0 {
		t.Errorf("ops' live sessions = %v of %d, %v; want none", holders, total, err)
	}
}
