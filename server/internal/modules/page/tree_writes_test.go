package page_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

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
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: f.alice})
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
