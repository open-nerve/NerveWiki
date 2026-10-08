package app_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
)

// An attachment's link is its name when no other attachment of the
// notebook has its title key, a page's no matter; its path from the root
// otherwise, and for one without an extension, which nothing leads to. A
// page, or a node gone, has none (M7/P3 design 4.6).
func TestAnAttachmentsLinkIsItsNameOrItsPath(t *testing.T) {
	w := newWorld(t, "A", "B", "C", "D", "D/y.png")
	a, b := w.tree.addAsset("A/x.png"), w.tree.addAsset("B/X.PNG")
	y, data := w.tree.addAsset("C/y.png"), w.tree.addAsset("C/data")
	gone := w.tree.addAsset("C/gone.png")
	w.tree.nodes[gone].gone = true
	links := app.AssetLinks{Pages: w.tree}
	got, err := links.Of(context.Background(), w.notebook, []uuid.UUID{y, a, b, data, gone, w.id("D/y.png"), a})
	want := map[uuid.UUID]string{a: "A/x.png", b: "B/X.PNG", y: "y.png", data: "C/data"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Of = %v, %v; want %v", got, err, want)
	}
	if got, err := links.Of(context.Background(), w.notebook, nil); err != nil || len(got) != 0 || got == nil {
		t.Errorf("Of(none) = %#v, %v; want an empty map", got, err)
	}
}

// A read that fails is the links' failure.
func TestAnAttachmentsLinkAnswersItsReadsFailure(t *testing.T) {
	w := newWorld(t, "A")
	a := w.tree.addAsset("A/x.png")
	for _, fails := range []int{1, 2} {
		w.tree.failAt, w.tree.reads = fails, 0
		if _, err := (app.AssetLinks{Pages: w.tree}).Of(context.Background(), w.notebook, []uuid.UUID{a}); !errors.Is(err, errRead) {
			t.Errorf("read %d failing: %v, want the read's failure", fails, err)
		}
	}
}
