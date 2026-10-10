package page_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The wired module's TreeWrites create an attachment's node in a unit on
// the database: the guard sees its file, after runs in the transaction
// with the node written, the observers follow; after's error rolls the
// node back. Check decides as the unit does.
func TestTheAttachmentsWritesReachTheUnit(t *testing.T) {
	f := newFixture(t)
	g, o := &guard{}, &observer{f: f}
	writes := f.module([]page.WriteGuard{g}, nil, []page.PageObserver{o}).TreeWrites()
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: f.alice, SessionID: uuid.NewV7()})
	sum := bytes.Repeat([]byte{1}, 32)
	a := page.NewAsset{NotebookID: f.eng, ParentID: &f.notes, Name: "photo.png", Action: "asset.upload", Client: "web",
		Meta: page.AssetMeta{MIME: "image/png", Bytes: 3, SHA256: sum}}

	if err := writes.Check(ctx, a); err != nil {
		t.Errorf("Check(photo.png) = %v, want nil", err)
	}
	var seen int
	var inTx bool
	n, err := writes.CreateAsset(ctx, a, func(ctx context.Context, n page.NodeInfo) error {
		inTx = postgres.InTx(ctx)
		return postgres.DB(ctx, f.pool).QueryRow(ctx, "SELECT count(*) FROM nodes WHERE id = $1 AND kind = 'asset'", n.ID).Scan(&seen)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !n.Asset || n.Name != "photo.png" || *n.ParentID != f.notes || n.NotebookID != f.eng || n.CreatedBy != f.alice {
		t.Errorf("node = %+v, want photo.png, an attachment under Notes by alice", n)
	}
	if !inTx || seen != 1 {
		t.Errorf("after ran in a transaction: %v, seeing the node %d times; want once, in it", inTx, seen)
	}
	if len(g.steps) != 1 || !g.inTx || g.steps[0].Asset == nil || !bytes.Equal(g.steps[0].Asset.SHA256, sum) {
		t.Errorf("the guard saw %+v, want the attachment's file", g.steps)
	}
	if len(o.events) != 1 || o.events[0].Changes[0].NodeID != n.ID || o.events[0].Changes[0].Revision != 0 {
		t.Errorf("the observer saw %+v, want the attachment created", o.events)
	}

	failed := errors.New("the row failed")
	a.Name = "other.png"
	if _, err := writes.CreateAsset(ctx, a, func(context.Context, page.NodeInfo) error { return failed }); !errors.Is(err, failed) {
		t.Errorf("CreateAsset with after failing = %v, want its error", err)
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE name = 'other.png'"); got != 0 {
		t.Errorf("%d nodes named other.png, want none: after's error rolls the unit back", got)
	}
	a.Name = "PHOTO.png"
	var e *shared.Error
	if err := writes.Check(ctx, a); !errors.As(err, &e) || e.Code != "page.title_taken" {
		t.Errorf("Check(PHOTO.png) = %v, want page.title_taken", err)
	}
}

// The wired module's TreeWrites run an import's units on the database: a
// job acting for alice creates pages and attachments in a changeset of the
// import kind from the job's client, numbering a name taken; a later unit
// merges into it, moving its time on. The content is checked and parsed
// outside the unit; the read port tells a page's depth.
func TestTheImportsWritesReachTheUnit(t *testing.T) {
	f := newFixture(t)
	clock := &steppingClock{at: testNow()}
	f.clock = clock
	o := &observer{f: f}
	writes := f.module(nil, nil, []page.PageObserver{o}).TreeWrites()
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: f.alice, JobID: uuid.NewV7()})
	if err := writes.CheckContent("# Hi"); err != nil {
		t.Errorf("CheckContent(# Hi) = %v, want nil", err)
	}
	var e *shared.Error
	if err := writes.CheckContent("a\x00b"); !errors.As(err, &e) || e.Code != shared.CodeValidationFailed {
		t.Errorf("CheckContent(NUL) = %v, want 422", err)
	}
	parsed, err := writes.Parse(ctx, "# Hi")
	if err != nil {
		t.Fatal(err)
	}
	spec := page.ImportSpec{NotebookID: f.eng, Action: "transfer.import", Client: page.Client("api")}
	var top, photo page.NodeInfo
	first, err := writes.Import(ctx, spec, func(ctx context.Context, u page.ImportUnit) error {
		var err error
		if top, err = u.CreatePage(ctx, page.ImportedPage{Name: "NOTES", Content: "# Hi", Parsed: parsed}); err != nil {
			return err
		}
		photo, err = u.CreateAsset(ctx, page.ImportedAsset{ParentID: &top.ID, Name: "photo.png",
			Meta: page.AssetMeta{MIME: "image/png", Bytes: 3, SHA256: bytes.Repeat([]byte{1}, 32)}},
			func(context.Context, page.NodeInfo) error { return nil })
		return err
	})
	parsed.Release()
	if err != nil {
		t.Fatal(err)
	}
	if top.Name != "NOTES 2" || top.Asset || !photo.Asset || *photo.ParentID != top.ID {
		t.Errorf("created %+v and %+v, want NOTES 2 numbered past Notes, the photo under it", top, photo)
	}
	if got := f.count(t, "SELECT count(*) FROM changesets WHERE id = $1 AND kind = 'import' AND client = 'api' AND created_by_id = $2",
		first, f.alice); got != 1 {
		t.Errorf("%d changesets %s of an import from the API by alice, want 1", got, first)
	}
	if got := f.count(t, "SELECT count(*) FROM page_contents WHERE node_id = $1 AND content = '# Hi' AND revision = 1", top.ID); got != 1 {
		t.Errorf("%d contents of NOTES 2 at revision 1, want 1", got)
	}
	second, err := writes.Import(ctx, page.ImportSpec{NotebookID: f.eng, Action: "transfer.import", Client: page.Client("api"), Changeset: first},
		func(ctx context.Context, u page.ImportUnit) error {
			_, err := u.CreatePage(ctx, page.ImportedPage{ParentID: &top.ID, Name: "Child"})
			return err
		})
	if err != nil || second != first {
		t.Fatalf("a later unit = %s, %v; want it merged into %s", second, err, first)
	}
	if got := f.count(t, "SELECT count(*) FROM changesets WHERE notebook_id = $1", f.eng); got != 1 {
		t.Errorf("%d changesets, want the one", got)
	}
	if got := f.count(t, "SELECT count(*) FROM changesets WHERE id = $1 AND updated_at > created_at", first); got != 1 {
		t.Errorf("the changeset's time did not move on with the later unit")
	}
	if len(o.events) != 2 {
		t.Errorf("the observer saw %d units, want 2", len(o.events))
	}
	nodes := page.NewExportNodes(f.pool)
	for _, tt := range []struct {
		id    uuid.UUID
		depth int
		ok    bool
	}{{f.notes, 1, true}, {top.ID, 1, true}, {photo.ID, 0, false}, {uuid.NewV7(), 0, false}} {
		if depth, ok, err := nodes.Depth(context.Background(), f.eng, tt.id); err != nil || depth != tt.depth || ok != tt.ok {
			t.Errorf("Depth(%s) = %d, %t, %v; want %d, %t", tt.id, depth, ok, err, tt.depth, tt.ok)
		}
	}
	child := f.count(t, "SELECT count(*) FROM nodes WHERE parent_id = $1 AND name = 'Child'", top.ID)
	var childID uuid.UUID
	if err := f.pool.QueryRow(context.Background(), "SELECT id FROM nodes WHERE parent_id = $1 AND name = 'Child'", top.ID).Scan(&childID); err != nil || child != 1 {
		t.Fatalf("Child: %v", err)
	}
	if depth, ok, err := nodes.Depth(context.Background(), f.eng, childID); err != nil || depth != 2 || !ok {
		t.Errorf("Depth(Child) = %d, %t, %v; want 2", depth, ok, err)
	}
}

// steppingClock moves on a second each time it is read.
type steppingClock struct {
	at time.Time
}

func (c *steppingClock) Now() time.Time {
	c.at = c.at.Add(time.Second)
	return c.at
}
