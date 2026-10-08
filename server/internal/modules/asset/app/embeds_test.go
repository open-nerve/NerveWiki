package app_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"

	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/mac"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
)

// Embeds answers each of the ids with a row of the notebook, signed as of
// one read of the clock, and leaves out one of another notebook or without
// a row; a read's error is its (M7/P3 design 5.4).
func TestEmbedsAreTheNotebooksAttachmentsSignedAtOnce(t *testing.T) {
	rows := newRows()
	nb, other := uuid.NewV7(), uuid.NewV7()
	a, b, elsewhere, rowless := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	for id, notebook := range map[uuid.UUID]uuid.UUID{a: nb, b: nb, elsewhere: other} {
		rows.rows[id] = domain.Blob{ID: uuid.NewV7(), NodeID: id, NotebookID: notebook, MIME: "image/png", Bytes: 9, Width: 7, Height: 5}
	}
	c := &clock{step: 2 * time.Hour}
	e := app.NewEmbeds(rows, macadapter.New(signKey()), c)
	got, err := e.Of(context.Background(), nb, []uuid.UUID{a, b, elsewhere, rowless})
	if err != nil {
		t.Fatal(err)
	}
	signer := macadapter.New(signKey())
	if len(got) != 2 || c.reads != 1 {
		t.Fatalf("got %v after %d reads of the clock", got, c.reads)
	}
	for _, id := range []uuid.UUID{a, b} {
		if x := got[id]; !reflect.DeepEqual(x.Blob, rows.rows[id]) || x.Signed != signer.Sign(now(), id, rows.rows[id].ID) {
			t.Errorf("%s: %+v", id, x)
		}
	}
	rows.readErr = errors.New("down")
	if _, err := e.Of(context.Background(), nb, []uuid.UUID{a}); !errors.Is(err, rows.readErr) {
		t.Errorf("a read's error: %v", err)
	}
}
