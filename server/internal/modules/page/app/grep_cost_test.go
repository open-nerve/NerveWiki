//go:build !race

package app_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// grepGlobTimeBound is how long a grep over the glob guard tree may take:
// three times its time here, 0.40-0.41 s (M9/P4 design 6.1, S1's report).
const grepGlobTimeBound = 1250 * time.Millisecond

// guardNodes is P1's guard tree as a notebook's nodes (page/domain's
// guardTree, written again here): 100,000 roots of about 255 bytes, each
// above a chain of 10 pages of such titles, 1.1 million nodes, sharing no
// path. Its ListNodes answers them; its PageStates and GrepReads every
// page's content a short line. The rest of Nodes is not called.
type guardNodes struct {
	app.Nodes
	nodes []domain.Node
}

func (g guardNodes) ListNodes(context.Context, uuid.UUID) ([]domain.Node, error) { return g.nodes, nil }

// guardTitle is a title of about 255 bytes of a, ending in four letters
// that tell siblings apart: page/domain's guardKey.
func guardTitle(i int) string {
	const letters = "bcdefghijklmnopqrstuvwxyz"
	var b strings.Builder
	b.WriteString(strings.Repeat("a", 251))
	for range 4 {
		b.WriteByte(letters[i%len(letters)])
		i /= len(letters)
	}
	return b.String()
}

func newGuardNodes(notebook uuid.UUID) guardNodes {
	chain := make([]string, 11)
	for d := range chain {
		chain[d] = guardTitle(d)
	}
	nodes := make([]domain.Node, 0, 1_100_000)
	for i := range 100_000 {
		root := domain.Node{ID: uuid.NewV7(), NotebookID: notebook, Kind: domain.KindPage, Name: guardTitle(i)}
		root.NameKey = shared.TitleKey(root.Name)
		nodes = append(nodes, root)
		parent := root.ID
		for d := 1; d < 11; d++ {
			n := domain.Node{ID: uuid.NewV7(), NotebookID: notebook, ParentID: &parent, Kind: domain.KindPage, Name: chain[d],
				NameKey: chain[d]}
			nodes = append(nodes, n)
			parent = n.ID
		}
	}
	return guardNodes{nodes: nodes}
}

// shortContents are every page's content one short line.
type shortContents struct{}

func (shortContents) PageStates(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]app.PageState, error) {
	out := make(map[uuid.UUID]app.PageState, len(ids))
	for _, id := range ids {
		out[id] = app.PageState{Revision: 1, Bytes: 12}
	}
	return out, nil
}

func (shortContents) AliveNodes(context.Context, []uuid.UUID) (map[uuid.UUID]bool, error) {
	return nil, nil
}

func (shortContents) GrepCandidates(context.Context, uuid.UUID, []string) ([]uuid.UUID, error) {
	return nil, nil
}

func (shortContents) GrepContents(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(ids))
	for _, id := range ids {
		out[id] = "a short line"
	}
	return out, nil
}

func (shortContents) SearchBuilding(context.Context, []uuid.UUID) (bool, error) { return false, nil }

// TestGrepOverTheGlobGuardTreeTakesItsTime: a pattern of no keys and one
// of P1's worst globs (its ligature-free walk over pieces of 1,000
// characters, which no title holds) over P1's guard tree, 1.1 million
// nodes: the walk reaches MaxGlobWork and answers its cursor, within
// three times its time here (M9/P4 design 6.1, review C-I3). make
// test-go-costs runs it alone, with -v; -short skips it.
func TestGrepOverTheGlobGuardTreeTakesItsTime(t *testing.T) {
	if testing.Short() {
		t.Skip("a bound in time, measured alone by make test-go-costs")
	}
	f := newFixture()
	f.grant(domain.ActionRead)
	nodes := newGuardNodes(f.eng)
	var b strings.Builder
	b.WriteString("**/*")
	for i := range 21 {
		b.WriteString(strings.Repeat("a", 47))
		if i < 20 {
			b.WriteString("?")
		}
	}
	glob := b.String() + "b*/**"
	for len(glob) > domain.MaxGlobBytes {
		glob = glob[:len(glob)-1]
	}
	glob = strings.TrimSuffix(glob, "/")
	g := app.NewGrep(app.GrepDeps{Notebooks: f.notebooks, Nodes: nodes, States: shortContents{}, Reads: shortContents{},
		Auth: f.auth, Parses: app.NewGrepParses(1), Slots: app.NewGrepSlots(1), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Clock: f.clock})
	start := time.Now()
	got, err := g.Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: "TODO|FIXME", Glob: &glob})
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d bytes of glob over 1.1 million nodes: %v, stop %s, %d pages", len(glob), took, got.Walk.Stop, got.Walk.Pages)
	if got.Walk.Stop != "glob" || !got.Incomplete || got.Next == "" || took > grepGlobTimeBound {
		t.Errorf("took %v of %v: stop %q, incomplete %v, next %q", took, grepGlobTimeBound, got.Walk.Stop, got.Incomplete, got.Next)
	}
}
