package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// directory uses the ordinary index fakes, with independently supplied full
// values for the new batched port. Expected catalogue values below are literal.
type directory struct {
	indexLib
	state     app.IndexNotebookState
	source    []app.IndexSourceNode
	heads     int
	batches   [][]uuid.UUID
	failBatch int
	failure   error
}

func newDirectory() *directory {
	l := newIndexLib()
	l.reads.held = nil
	l.reads.facts = map[uuid.UUID]app.IndexFacts{}
	l.paths = newPaths(l.library)
	return &directory{indexLib: l, state: app.IndexNotebookState{Mode: "standard"}}
}
func directoryID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-4000-8000-%012x", n))
}
func (d *directory) add(n int, path string, typ *string) uuid.UUID {
	id := directoryID(n)
	d.paths.add(id, path)
	d.reads.held = append(d.reads.held, id)
	d.reads.facts[id] = app.IndexFacts{Type: typ}
	return id
}
func textPtr(s string) *string { return &s }
func (d *directory) StateOf(context.Context, uuid.UUID) (app.IndexNotebookState, bool, error) {
	return d.state, true, nil
}
func (d *directory) Nodes(context.Context, uuid.UUID, uuid.UUID) ([]app.IndexSourceNode, error) {
	return d.source, nil
}
func (d *directory) CatalogueHeads(_ context.Context, _ uuid.UUID, ids []uuid.UUID) ([]app.IndexCatalogueHead, error) {
	d.heads++
	d.batches = append(d.batches, slices.Clone(ids))
	if len(ids) > 512 {
		return nil, fmt.Errorf("oversized batch: %d", len(ids))
	}
	if d.heads == d.failBatch {
		return nil, d.failure
	}
	out := make([]app.IndexCatalogueHead, len(ids))
	for i, id := range ids {
		f := d.reads.facts[id]
		out[i] = app.IndexCatalogueHead{ID: id, Typed: f.Type != nil}
		if f.Type != nil && len(*f.Type) <= 1024 {
			out[i].FilterType = f.Type
		}
	}
	return out, nil
}
func (d *directory) use() app.ListIndex {
	l := d.listIndex()
	l.Notebooks = d
	l.Sources = d
	l.Catalogue = d
	return l
}
func (d *directory) rest(t *testing.T, q app.IndexQuery) app.IndexPage {
	t.Helper()
	q.Catalogue, q.ExcludeArea = true, true
	p, err := d.use().Execute(reader(), d.nb, q)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func expectTypes(t *testing.T, p app.IndexPage, want []app.IndexType, complete bool, long int64) {
	t.Helper()
	if !reflect.DeepEqual(p.Types, want) || p.TypesComplete != complete || p.UnfilterableTypePages != long {
		t.Fatalf("catalogue=%+v complete=%v long=%d; want %+v %v %d", p.Types, p.TypesComplete, p.UnfilterableTypePages, want, complete, long)
	}
}
func scopeDirectory() (*directory, uuid.UUID) {
	d := newDirectory()
	root := d.add(1, "raw.md", nil)
	d.add(2, "Topic.md", textPtr("topic"))
	s := d.add(3, "raw/Source.md", textPtr("source"))
	empty := d.add(4, "Empty.md", nil)
	d.state = app.IndexNotebookState{Mode: "llm_wiki", SourceRootID: &root}
	d.source = []app.IndexSourceNode{{ID: s, Bytes: 20}}
	return d, empty
}

func TestDirectoryIndex(t *testing.T) {
	t.Run("exclude", func(t *testing.T) {
		d := newDirectory()
		root := d.add(1, "raw.md", textPtr("root"))
		s := d.add(2, "raw/Source.md", textPtr("source"))
		wiki := d.add(3, "Wiki.md", textPtr("topic"))
		d.state = app.IndexNotebookState{Mode: "llm_wiki", SourceRootID: &root}
		d.source = []app.IndexSourceNode{{ID: s, Bytes: 10}, {ID: directoryID(4), Asset: true}, {ID: directoryID(5)}}
		p := d.rest(t, app.IndexQuery{})
		if !slices.Equal(idsOf(p), []uuid.UUID{root, wiki}) {
			t.Fatalf("REST IDs=%v", idsOf(p))
		}
		expectTypes(t, p, []app.IndexType{{Type: "root", Count: 1}, {Type: "topic", Count: 1}}, true, 0)
		if p.SourceArea == nil || p.SourceArea.RootID != root || p.SourceArea.Sources != 2 || p.Mode != "llm_wiki" {
			t.Fatalf("source metadata=%+v mode=%q", p.SourceArea, p.Mode)
		}
		calls := d.heads
		m, err := d.use().Execute(reader(), d.nb, app.IndexQuery{})
		if err != nil || !slices.Equal(idsOf(m), []uuid.UUID{root, s, wiki}) || d.heads != calls {
			t.Fatalf("MCP=%v %v, catalogue calls=%d to %d", idsOf(m), err, calls, d.heads)
		}
	})
	t.Run("types_scope", func(t *testing.T) {
		cases := []struct {
			name string
			q    app.IndexQuery
		}{
			{"a_type", app.IndexQuery{Type: textPtr("absent")}},
			{"b_empty_children", app.IndexQuery{ParentID: new(directoryID(4))}},
			{"c_invalid_tag", app.IndexQuery{Tag: textPtr("a b")}},
			{"d_empty_tag", app.IndexQuery{Tag: textPtr("absent")}},
			{"e_no_type_matches", app.IndexQuery{Type: textPtr("source")}},
			{"f_past_cursor", app.IndexQuery{}},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				d, _ := scopeDirectory()
				q := c.q
				if c.name == "f_past_cursor" {
					cursor, err := shared.EncodeCursor(domain.IndexCursor{ID: uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")})
					if err != nil {
						t.Fatal(err)
					}
					q.Cursor = &cursor
				}
				p := d.rest(t, q)
				if len(p.Items) != 0 || p.Items == nil || p.Next != "" {
					t.Fatalf("empty items=%+v", p)
				}
				expectTypes(t, p, []app.IndexType{{Type: "topic", Count: 1}}, true, 0)
				if p.Mode != "llm_wiki" || p.SourceArea == nil || p.SourceArea.RootID != directoryID(1) || p.SourceArea.Sources != 1 {
					t.Fatalf("lost metadata: %+v", p)
				}
			})
		}
		d, _ := scopeDirectory()
		q := app.IndexQuery{Tag: textPtr("a b")}
		m, err := d.use().Execute(reader(), d.nb, q)
		if err != nil || len(m.Items) != 0 || d.heads != 0 {
			t.Fatalf("MCP empty=%+v %v calls=%d", m, err, d.heads)
		}
		// A matching item filter also leaves the catalogue independent.
		d, _ = scopeDirectory()
		d.reads.tagged["one"] = []uuid.UUID{directoryID(2)}
		p := d.rest(t, app.IndexQuery{Type: textPtr("topic"), Tag: textPtr("one"), AtRoot: true})
		expectTypes(t, p, []app.IndexType{{Type: "topic", Count: 1}}, true, 0)
		if len(p.Items) != 1 {
			t.Fatalf("combined filters: %+v", p.Items)
		}
	})
	t.Run("exact_collision", func(t *testing.T) {
		d := newDirectory()
		prefix := strings.Repeat("p", 128)
		values := []string{"A", "a", prefix + "X", prefix + "Y"}
		counts := []int{2, 3, 2, 3}
		want := []app.IndexType{}
		n := 1
		owned := map[string][]uuid.UUID{}
		for i, v := range values {
			want = append(want, app.IndexType{Type: v, Count: counts[i]})
			for range counts[i] {
				id := d.add(n, fmt.Sprintf("P%d.md", n), textPtr(v))
				owned[v] = append(owned[v], id)
				n++
			}
		}
		p := d.rest(t, app.IndexQuery{})
		expectTypes(t, p, want, true, 0)
		for _, v := range values {
			p = d.rest(t, app.IndexQuery{Type: &v})
			if !slices.Equal(idsOf(p), owned[v]) {
				t.Fatalf("exact filter %q=%v want=%v", v, idsOf(p), owned[v])
			}
			expectTypes(t, p, want, true, 0)
		}
	})
	t.Run("type_bytes", func(t *testing.T) {
		d := newDirectory()
		at := strings.Repeat("界", 341) + "a"
		past := at + "b"
		id := d.add(1, "At.md", &at)
		d.add(2, "Past.md", &past)
		d.add(3, "PastAgain.md", &past)
		p := d.rest(t, app.IndexQuery{Type: &at})
		if !slices.Equal(idsOf(p), []uuid.UUID{id}) {
			t.Fatalf("1024-byte exact=%v", idsOf(p))
		}
		expectTypes(t, p, []app.IndexType{{Type: at, Count: 1}}, false, 2)
		_, err := d.use().Execute(reader(), d.nb, app.IndexQuery{Type: &past, Catalogue: true})
		var e *shared.Error
		if !errors.As(err, &e) || e.Code != shared.CodeValidationFailed || e.Fields[0].Field != "type" {
			t.Fatalf("1025 bytes=%v", err)
		}
	})
	t.Run("catalogue_bounds", func(t *testing.T) {
		t.Run("items", func(t *testing.T) {
			d := newDirectory()
			want := []app.IndexType{}
			for n := 100; n >= 0; n-- {
				s := fmt.Sprintf("t%03d", n)
				d.add(101-n, s+".md", &s)
			}
			// Evicted values repeat later; retained values count across the entire input.
			d.add(102, "AgainLargest.md", textPtr("t100"))
			d.add(103, "AgainSmallest.md", textPtr("t000"))
			for n := 0; n < 100; n++ {
				count := 1
				if n == 0 {
					count = 2
				}
				want = append(want, app.IndexType{Type: fmt.Sprintf("t%03d", n), Count: count})
			}
			p := d.rest(t, app.IndexQuery{})
			expectTypes(t, p, want, false, 0)
			p = d.rest(t, app.IndexQuery{Type: textPtr("t100")})
			if len(p.Items) != 2 {
				t.Fatalf("evicted exact type inaccessible: %+v", p.Items)
			}
		})
		t.Run("json_bytes", func(t *testing.T) {
			d := newDirectory()
			want := []app.IndexType{}
			for n := 0; n < 8; n++ {
				s := fmt.Sprintf("%02d", n) + strings.Repeat("<", 1000)
				d.add(n+1, fmt.Sprintf("P%d.md", n), &s)
				if n < 5 {
					want = append(want, app.IndexType{Type: s, Count: 1})
				}
			}
			p := d.rest(t, app.IndexQuery{})
			expectTypes(t, p, want, false, 0)
			b, err := json.Marshal(p.Types)
			if err != nil || len(b) > 32*1024 {
				t.Fatalf("types actual JSON=%d %v", len(b), err)
			}
			extra := append(slices.Clone(p.Types), app.IndexType{Type: "05" + strings.Repeat("<", 1000), Count: 1})
			b, _ = json.Marshal(extra)
			if len(b) <= 32*1024 {
				t.Fatalf("fixture did not cross JSON byte boundary: %d", len(b))
			}
		})
	})

	t.Run("read_errors", func(t *testing.T) {
		d, _ := scopeDirectory()
		d.failure = errors.New("catalogue after empty items")
		d.failBatch = 1
		_, err := d.use().Execute(reader(), d.nb, app.IndexQuery{Tag: textPtr("a b"), Catalogue: true, ExcludeArea: true})
		if !errors.Is(err, d.failure) {
			t.Fatalf("empty items swallowed metadata error: %v", err)
		}
		u := d.use()
		u.Reads = directoryReadErrors{IndexReads: d.reads, tag: errors.New("tag read"), pages: errors.New("head read")}
		_, err = u.Execute(reader(), d.nb, app.IndexQuery{Tag: textPtr("known"), Catalogue: true})
		if err == nil || err.Error() != "tag read" {
			t.Fatalf("tag error precedence=%v", err)
		}
		_, err = u.Execute(reader(), d.nb, app.IndexQuery{ParentID: new(directoryID(99)), Tag: textPtr("known"), Catalogue: true})
		if !errors.Is(err, domain.ErrPageNotFound) {
			t.Fatalf("parent error precedence=%v", err)
		}
		_, err = u.Execute(reader(), d.nb, app.IndexQuery{Catalogue: true})
		if err == nil || err.Error() != "head read" {
			t.Fatalf("head read error=%v", err)
		}
	})
	t.Run("catalogue_complete", func(t *testing.T) {
		d := newDirectory()
		d.add(1, "A.md", textPtr("A"))
		d.add(2, "B.md", textPtr("B"))
		d.add(3, "C.md", textPtr("C"))
		p := d.rest(t, app.IndexQuery{})
		expectTypes(t, p, []app.IndexType{{Type: "A", Count: 1}, {Type: "B", Count: 1}, {Type: "C", Count: 1}}, true, 0)
		if p.Mode != "standard" || p.SourceArea != nil {
			t.Fatalf("standard metadata=%+v", p)
		}
		t.Run("long", func(t *testing.T) {
			d.add(4, "Long.md", textPtr(strings.Repeat("x", 1025)))
			p := d.rest(t, app.IndexQuery{})
			expectTypes(t, p, []app.IndexType{{Type: "A", Count: 1}, {Type: "B", Count: 1}, {Type: "C", Count: 1}}, false, 1)
		})
		t.Run("whole_input", func(t *testing.T) {
			d := newDirectory()
			for n := 1; n <= 513; n++ {
				d.add(n, fmt.Sprintf("P%d.md", n), textPtr("A"))
			}
			p := d.rest(t, app.IndexQuery{})
			expectTypes(t, p, []app.IndexType{{Type: "A", Count: 513}}, true, 0)
			if len(d.batches) != 2 || len(d.batches[0]) != 512 || len(d.batches[1]) != 1 {
				t.Fatalf("finite batches=%v", d.batches)
			}
		})
		t.Run("late_error", func(t *testing.T) {
			d := newDirectory()
			for n := 1; n <= 513; n++ {
				d.add(n, fmt.Sprintf("P%d.md", n), textPtr("A"))
			}
			d.failBatch = 2
			d.failure = errors.New("late catalogue read")
			_, err := d.use().Execute(reader(), d.nb, app.IndexQuery{Catalogue: true})
			if !errors.Is(err, d.failure) || d.heads != 2 {
				t.Fatalf("late failure=%v, calls=%d", err, d.heads)
			}
		})
	})
	t.Run("judges_in_turn", func(t *testing.T) {
		d := newDirectory()
		d.add(1, "A.md", textPtr("A"))
		zero := 0
		for _, c := range []struct {
			name string
			q    app.IndexQuery
			code string
		}{{"cursor", app.IndexQuery{Cursor: textPtr("bad"), Type: textPtr("")}, "bad_request"}, {"filter", app.IndexQuery{Type: textPtr(""), Limit: &zero}, "validation_failed"}, {"limit", app.IndexQuery{Limit: &zero}, "validation_failed"}, {"hidden", app.IndexQuery{}, "notebook.not_found"}, {"parent_conflict", app.IndexQuery{ParentID: new(uuid.UUID), AtRoot: true}, "validation_failed"}, {"internal_parent_conflict", app.IndexQuery{Parent: textPtr("/"), AtRoot: true}, "validation_failed"}} {
			t.Run(c.name, func(t *testing.T) {
				_, err := d.use().Execute(reader(), d.hidden, c.q)
				var e *shared.Error
				if !errors.As(err, &e) || e.Code != c.code {
					t.Fatalf("error=%v want=%s", err, c.code)
				}
			})
		}
		missing := directoryID(98)
		_, err := d.use().Execute(reader(), d.nb, app.IndexQuery{ParentID: &missing, Catalogue: true})
		if !errors.Is(err, domain.ErrPageNotFound) {
			t.Fatalf("parent missing=%v", err)
		}
	})
	t.Run("zero_parent", func(t *testing.T) {
		d := newDirectory()
		zero := uuid.UUID{}
		d.paths.add(zero, "Zero.md")
		d.reads.held = append(d.reads.held, zero)
		d.reads.facts[zero] = app.IndexFacts{}
		child := d.add(1, "Zero/Child.md", textPtr("topic"))
		root := d.add(2, "Other.md", nil)
		children := d.rest(t, app.IndexQuery{ParentID: &zero})
		if !slices.Equal(idsOf(children), []uuid.UUID{child}) {
			t.Errorf("real zero parent children=%v; want only %v", idsOf(children), child)
		}
		roots := d.rest(t, app.IndexQuery{AtRoot: true})
		if !slices.Equal(idsOf(roots), []uuid.UUID{zero, root}) {
			t.Errorf("nil root children=%v; want %v", idsOf(roots), []uuid.UUID{zero, root})
		}
		expectTypes(t, children, []app.IndexType{{Type: "topic", Count: 1}}, true, 0)
		expectTypes(t, roots, []app.IndexType{{Type: "topic", Count: 1}}, true, 0)
	})
	t.Run("rest_mcp", func(t *testing.T) {
		d := newDirectory()
		d.add(4, "Untyped.md", nil)
		d.add(2, "Alpha2.md", textPtr("alpha"))
		d.add(3, "Beta.md", textPtr("Beta"))
		d.add(1, "Alpha1.md", textPtr("alpha"))
		want := []uuid.UUID{directoryID(1), directoryID(2), directoryID(3), directoryID(4)}
		p := d.rest(t, app.IndexQuery{})
		if !slices.Equal(idsOf(p), want) {
			t.Fatalf("REST=%v want=%v", idsOf(p), want)
		}
		calls := d.heads
		m, err := d.use().Execute(reader(), d.nb, app.IndexQuery{})
		if err != nil || !slices.Equal(idsOf(m), want) || d.heads != calls {
			t.Fatalf("MCP=%v %v catalogue %d -> %d", idsOf(m), err, calls, d.heads)
		}
	})
}

type directoryReadErrors struct {
	app.IndexReads
	tag, pages error
}

func (r directoryReadErrors) TagPages(context.Context, uuid.UUID, string) ([]uuid.UUID, error) {
	return nil, r.tag
}
func (r directoryReadErrors) IndexPages(context.Context, uuid.UUID, *string) ([]app.IndexedPage, error) {
	return nil, r.pages
}
