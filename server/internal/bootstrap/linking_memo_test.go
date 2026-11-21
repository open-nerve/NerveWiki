package bootstrap

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// oneNodeTree is the page module's tree as the index reads it, of one node
// that a rename reaches; its other reads are none of the index's here.
type oneNodeTree struct {
	linking.Pages
	node uuid.UUID
}

func (o oneNodeTree) Subtree(context.Context, uuid.UUID, uuid.UUID) ([]linking.Step, error) {
	return []linking.Step{{ID: o.node, Key: "b", Name: "b"}}, nil
}

// An event no unit made has no memo, a nil one in the interface the index
// gets: the index follows it as it does any rename, reading the links
// itself (M12/P4/S7 design 3.2, 4.3 item 8; review B's N3).
func TestTheIndexFollowsAnEventWithoutAMemo(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	node, nb := uuid.NewV7(), uuid.NewV7()
	index := linkIndex{linking.NewIndex(pool, oneNodeTree{node: node}, linkEvents{events.NewPublisher()}, linking.IngestDeps{
		Metadata: linkIngestMetadata{page.NewIngestMetadata(pool)}, Marks: linkIngestMarks{page.NewIngestMarks(pool)},
		MaxContentBytes: page.MaxContentBytes,
	})}
	e := page.Event{Write: page.Write{NotebookID: nb}, Changes: []page.Change{
		{NodeID: node, Before: &page.TreeState{Name: "a"}, After: &page.TreeState{Name: "b"}},
	}}
	if e.Memo != nil {
		t.Fatal("the event has a memo")
	}
	err = postgres.NewTxManager(pool, time.Second).WithinTx(ctx, func(ctx context.Context) error {
		return index.PagesChanged(ctx, e)
	})
	if err != nil {
		t.Fatalf("the index of an event without a memo: %v", err)
	}
}
