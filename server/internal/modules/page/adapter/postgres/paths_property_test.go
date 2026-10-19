package postgresadapter_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"uuid"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// everyone may do anything: the paths' property is no decision's.
type everyone struct{}

func (everyone) Authorize(context.Context, shared.Actor, shared.Action, shared.Target) (shared.Grant, error) {
	return shared.Grant{WorkspaceRole: shared.WorkspaceMember, NotebookRole: shared.NotebookEditor}, nil
}

// inWorkspace has every notebook in one workspace.
type inWorkspace struct{ id uuid.UUID }

func (w inWorkspace) WorkspaceOf(context.Context, uuid.UUID) (uuid.UUID, bool, error) {
	return w.id, true, nil
}
func (inWorkspace) ShareByID(context.Context, uuid.UUID) (bool, error) { return true, nil }
func (inWorkspace) LockByID(context.Context, uuid.UUID) (bool, error)  { return true, nil }

// anyInfo is an info for every attachment.
type anyInfo struct{}

func (anyInfo) Of(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]app.AttachmentInfo, error) {
	out := make(map[uuid.UUID]app.AttachmentInfo, len(ids))
	for _, id := range ids {
		out[id] = app.AttachmentInfo{MIME: "application/octet-stream", Bytes: 1}
	}
	return out, nil
}

// titlePieces are what the random titles are made of: pages whose titles
// end with .md and .MD, pages named x.png, attachments with an extension
// and without, NFD's e and acute, a fold's ß and SS, and Korean, whose NFD
// is two or three times as long.
func titlePieces() []string {
	return []string{"a", "B", "é", "e" + "\xcc\x81", "ß", "SS", ".md", ".MD", "x.png", " ", "-", "漢", "한글", "data"}
}

// randomName is a title of the pieces, or an attachment's name, that its
// check keeps, as the store holds it.
func randomName(rnd *rand.Rand, asset bool) (string, bool) {
	var b strings.Builder
	pieces := titlePieces()
	for range 1 + rnd.IntN(4) {
		b.WriteString(pieces[rnd.IntN(len(pieces))])
	}
	if asset {
		t, err := domain.CheckAssetName("name", b.String())
		return t.Name, err == nil
	}
	t, err := domain.CheckTitle("title", b.String())
	return t.Name, err == nil
}

// Every node's path finds it, the path a node gives and the path the
// lookups read being two implementations (v0.1 design 13.4 items 10, 13;
// M9/P1 design 3.9.2): on random trees in the store, each alive node's
// PathOf is in NFC, as its names are, is the path Paths writes of it (M9/P2
// design 3.7), and parses and finds it; written in
// capitals or in NFD it finds it too, or is too long, its segments' or its
// bytes past their bounds; as a glob without wildcards it lists it alone,
// within MaxGlobBytes; its parent's path with a trailing "/" lists the
// parent's children as the store's Children has them. A path past
// MaxGlobBytes, which a node four levels down may have, is refused as a
// glob, 422 too_long, the message pointing at its parent's children,
// which list it, and at read, which finds it (M9/P1 S2 review, A-M3).
func TestEveryNodesPathFindsIt(t *testing.T) {
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: uuid.NewV7(), SessionID: uuid.NewV7()})
	f := newFixture(t)
	paths := app.NewNodePaths(f.s)
	list := app.NewListByPattern(inWorkspace{uuid.NewV7()}, f.s, f.s, anyInfo{}, everyone{})
	rnd := rand.New(rand.NewPCG(11, 13))
	keys := map[[3]string]bool{} // a notebook's, a parent's and a key
	for tree := range 4 {
		notebook := f.eng
		if tree%2 == 1 {
			notebook = f.ops
		}
		var nodes []domain.Node
		parents := []*domain.Node{nil}
		for range 80 {
			parent := parents[rnd.IntN(len(parents))]
			if parent != nil && depthOf(nodes, *parent) >= 6 {
				continue
			}
			asset := rnd.IntN(3) == 0
			name, ok := randomName(rnd, asset)
			var under string
			var parentID *uuid.UUID
			if parent != nil {
				under, parentID = parent.ID.String(), &parent.ID
			}
			key := [3]string{notebook.String(), under, shared.TitleKey(name)}
			if !ok || keys[key] {
				continue
			}
			keys[key] = true
			var n domain.Node
			if asset {
				n = f.asset(t, notebook, parentID, name, rnd.Float64())
			} else {
				n = f.page(t, notebook, parentID, name, rnd.Float64())
				parents = append(parents, &n)
			}
			nodes = append(nodes, n)
		}
		byID := map[uuid.UUID]domain.Node{}
		for _, n := range nodes {
			byID[n.ID] = n
		}
		written, err := paths.Paths(ctx, notebook, nodeIDs(nodes))
		if err != nil || len(written) != len(nodes) {
			t.Fatalf("Paths of %d nodes = %d, %v", len(nodes), len(written), err)
		}
		for _, n := range nodes {
			var ancestors []domain.Node
			for p := n.ParentID; p != nil; p = byID[*p].ParentID {
				ancestors = append([]domain.Node{byID[*p]}, ancestors...)
			}
			path := domain.PathOf(ancestors, n)
			if !norm.NFC.IsNormalString(path) {
				t.Fatalf("the path %q of %s is not in NFC, as its names are", path, n.Name)
			}
			if written[n.ID] != path {
				t.Fatalf("Paths writes %q for %s, PathOf %q", written[n.ID], n.Name, path)
			}
			findsIt(t, ctx, paths, notebook, path, n, false)
			findsIt(t, ctx, paths, notebook, strings.ToUpper(path), n, true)
			findsIt(t, ctx, paths, notebook, norm.NFD.String(path), n, true)
			got, err := list.Execute(ctx, notebook, path, nil, nil)
			if err != nil || len(got.Items) != 1 || got.Items[0].Node.ID != n.ID || got.Items[0].Path != path {
				t.Fatalf("list(%q) = %+v, %v; want it alone", path, got.Items, err)
			}
			if len(path) > domain.MaxGlobBytes {
				t.Fatalf("the path of %d bytes is listed as a glob", len(path))
			}
			dir := ""
			if len(ancestors) > 0 {
				dir = domain.PathOf(ancestors[:len(ancestors)-1], ancestors[len(ancestors)-1])
				dir = strings.TrimSuffix(dir, ".md") + "/"
			}
			children, err := f.s.Children(context.Background(), notebook, n.ParentID)
			if err != nil {
				t.Fatal(err)
			}
			limit := domain.MaxListLimit
			got, err = list.Execute(ctx, notebook, dir, &limit, nil)
			if err != nil || !slices.Equal(listedIDs(got), nodeIDs(children)) {
				t.Fatalf("list(%q) = %v, %v; want the store's children %v", dir, listedIDs(got), err, nodeIDs(children))
			}
		}
	}
	// Five levels of titles of 255 bytes: a path of 1,283 bytes.
	var chain []domain.Node
	var parent *uuid.UUID
	for i := range 5 {
		n := f.page(t, f.eng, parent, strings.Repeat("漢", 84)+string(rune('a'+i))+"x", 0)
		chain, parent = append(chain, n), &n.ID
	}
	deep := chain[len(chain)-1]
	path := domain.PathOf(chain[:len(chain)-1], deep)
	findsIt(t, ctx, paths, f.eng, path, deep, false)
	_, err := list.Execute(ctx, f.eng, path, nil, nil)
	var e *shared.Error
	if !errors.As(err, &e) || len(e.Fields) != 1 || e.Fields[0].Field != "pattern" || e.Fields[0].Code != shared.FieldTooLong ||
		!strings.Contains(e.Fields[0].Message, `("A/B/")`) || !strings.Contains(e.Fields[0].Message, "read") {
		t.Fatalf("list of a path of %d bytes = %v, want 422 too_long pointing at its parent's children", len(path), err)
	}
	dir := strings.TrimSuffix(domain.PathOf(chain[:len(chain)-2], chain[len(chain)-2]), ".md") + "/"
	got, err := list.Execute(ctx, f.eng, dir, nil, nil)
	if err != nil || !slices.Equal(listedIDs(got), []uuid.UUID{deep.ID}) {
		t.Fatalf("list(its parent's children) = %v, %v; want it", listedIDs(got), err)
	}
}

// findsIt checks that path finds node; a variant, when variant is set,
// may be too long instead.
func findsIt(t *testing.T, ctx context.Context, paths app.NodePaths, notebook uuid.UUID, path string, node domain.Node, variant bool) {
	t.Helper()
	p, err := domain.ParsePath("path", path, false)
	var e *shared.Error
	if variant && errors.As(err, &e) && len(e.Fields) == 1 && e.Fields[0].Code == shared.FieldTooLong {
		return
	}
	if err != nil {
		t.Fatalf("ParsePath(%q): %v", path, err)
	}
	found, err := paths.Find(ctx, notebook, p)
	if err != nil || found.Node.ID != node.ID {
		t.Fatalf("Find(%q) = %+v, %v; want %s", path, found.Node, err, node.Name)
	}
}

func depthOf(nodes []domain.Node, n domain.Node) int {
	depth := 1
	for p := n.ParentID; p != nil; depth++ {
		i := slices.IndexFunc(nodes, func(m domain.Node) bool { return m.ID == *p })
		p = nodes[i].ParentID
	}
	return depth
}

func listedIDs(l app.Listing) []uuid.UUID {
	out := make([]uuid.UUID, len(l.Items))
	for i, item := range l.Items {
		out[i] = item.Node.ID
	}
	return out
}

func nodeIDs(nodes []domain.Node) []uuid.UUID {
	out := make([]uuid.UUID, len(nodes))
	for i, n := range nodes {
		out[i] = n.ID
	}
	return out
}
