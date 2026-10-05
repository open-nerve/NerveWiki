package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// library is what the reads of the index read, faked: notebooks and their
// workspaces, deleted ones absent; the notebooks the caller sees; pages and
// their notebooks; the index's answers and the pages' contents. calls
// records the reads made, in order.
type library struct {
	workspaces map[uuid.UUID]uuid.UUID
	visible    map[uuid.UUID]bool
	notebooks  map[uuid.UUID]uuid.UUID
	nodes      []domain.Node
	backlinks  []app.Backlink
	properties *app.Properties
	tags       []app.Tag
	tagPages   []uuid.UUID
	aliases    map[uuid.UUID][]string
	contents   map[uuid.UUID]revised
	calls      []string
}

// revised is a page's content and its revision.
type revised struct {
	content  string
	revision int
}

func (l *library) call(c string) { l.calls = append(l.calls, c) }

func (l *library) WorkspaceOf(_ context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	l.call("WorkspaceOf")
	w, ok := l.workspaces[id]
	return w, ok, nil
}

func (l *library) Authorize(_ context.Context, _ shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	l.call("Authorize " + string(action))
	if !l.visible[t.NotebookID] || l.workspaces[t.NotebookID] != t.WorkspaceID {
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{NotebookRole: shared.NotebookReader}, nil
}

func (l *library) NotebookOf(_ context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	l.call("NotebookOf")
	nb, ok := l.notebooks[id]
	return nb, ok, nil
}

func (l *library) All(context.Context, uuid.UUID) ([]domain.Node, error) {
	l.call("All")
	return l.nodes, nil
}

func (l *library) Backlinks(_ context.Context, target, after uuid.UUID, size, contexts int) ([]app.Backlink, error) {
	l.call("Backlinks")
	var out []app.Backlink
	for _, b := range l.backlinks {
		if b.SourceID.Compare(after) > 0 && len(out) < size {
			b.Ranges = b.Ranges[:min(len(b.Ranges), contexts)]
			out = append(out, b)
		}
	}
	return out, nil
}

func (l *library) Properties(context.Context, uuid.UUID) (app.Properties, bool, error) {
	l.call("Properties")
	if l.properties == nil {
		return app.Properties{}, false, nil
	}
	return *l.properties, true, nil
}

func (l *library) Tags(context.Context, uuid.UUID) ([]app.Tag, error) {
	l.call("Tags")
	return l.tags, nil
}

func (l *library) TagPages(_ context.Context, _ uuid.UUID, key string) ([]uuid.UUID, error) {
	l.call("TagPages " + key)
	return l.tagPages, nil
}

func (l *library) NotebookAliases(context.Context, uuid.UUID) (map[uuid.UUID][]string, error) {
	l.call("NotebookAliases")
	return l.aliases, nil
}

func (l *library) Content(_ context.Context, id uuid.UUID) (string, int, bool, error) {
	l.call("Content")
	c, ok := l.contents[id]
	return c.content, c.revision, ok, nil
}

// lib is a library with a notebook the caller sees, nb, with the page p in
// it; and another, hidden, with the page q.
type lib struct {
	*library
	nb, hidden, p, q uuid.UUID
}

func newLibrary() lib {
	l := lib{library: &library{}, nb: uuid.NewV7(), hidden: uuid.NewV7(), p: uuid.NewV7(), q: uuid.NewV7()}
	l.workspaces = map[uuid.UUID]uuid.UUID{l.nb: uuid.NewV7(), l.hidden: uuid.NewV7()}
	l.visible = map[uuid.UUID]bool{l.nb: true}
	l.notebooks = map[uuid.UUID]uuid.UUID{l.p: l.nb, l.q: l.hidden}
	return l
}

func (l lib) access() app.Access {
	return app.Access{Notebooks: l.library, Pages: l.library, Auth: l.library}
}

func (l lib) listBacklinks() app.ListBacklinks {
	return app.ListBacklinks{Access: l.access(), Reads: l.library, Contents: l.library}
}

func (l lib) getProperties() app.GetPageProperties {
	return app.GetPageProperties{Access: l.access(), Reads: l.library}
}

func (l lib) getTag() app.GetTag {
	return app.GetTag{Access: l.access(), Reads: l.library, TagKey: func(name string) (string, bool) {
		return strings.ToLower(name), name != "" && !strings.Contains(name, " ")
	}}
}

// reader is a context with a signed-in caller.
func reader() context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: uuid.NewV7(), SessionID: uuid.NewV7()})
}

// A read by page answers page.not_found for a page that is not there, one
// in a deleted notebook, and one in a notebook the caller cannot see, the
// index unread; a read by notebook answers notebook.not_found alike (M6/P5
// design 2). A deleted notebook is not found by its workspace, whatever
// the decision on the roles it had would say.
func TestAReadOfWhatTheCallerCannotSeeIsNotFound(t *testing.T) {
	l := newLibrary()
	deleted, gone := uuid.NewV7(), uuid.NewV7()
	l.visible[deleted] = true // the caller had a role in it
	l.notebooks[gone] = deleted
	for _, id := range []uuid.UUID{uuid.NewV7(), gone, l.q} {
		if _, err := l.listBacklinks().Execute(reader(), id, nil, nil); !errors.Is(err, domain.ErrPageNotFound) {
			t.Errorf("backlinks of %v: %v", id, err)
		}
		if _, err := l.getProperties().Execute(reader(), id); !errors.Is(err, domain.ErrPageNotFound) {
			t.Errorf("properties of %v: %v", id, err)
		}
	}
	for _, id := range []uuid.UUID{uuid.NewV7(), deleted, l.hidden} {
		if _, err := (app.ListTags{Access: l.access(), Reads: l.library}).Execute(reader(), id); !errors.Is(err, domain.ErrNotebookNotFound) {
			t.Errorf("tags of %v: %v", id, err)
		}
		if _, err := l.getTag().Execute(reader(), id, "a"); !errors.Is(err, domain.ErrNotebookNotFound) {
			t.Errorf("a tag of %v: %v", id, err)
		}
		if _, err := (app.ListLinkTargets{Access: l.access(), Reads: l.library}).Execute(reader(), id); !errors.Is(err, domain.ErrNotebookNotFound) {
			t.Errorf("link targets of %v: %v", id, err)
		}
	}
	for _, c := range l.calls {
		if !strings.HasPrefix(c, "WorkspaceOf") && !strings.HasPrefix(c, "NotebookOf") && !strings.HasPrefix(c, "Authorize") {
			t.Errorf("read %s of what the caller cannot see", c)
		}
	}
	if _, err := l.listBacklinks().Execute(context.Background(), l.p, nil, nil); !errors.Is(err, shared.Unauthenticated()) {
		t.Errorf("without an actor: %v", err)
	}
}

// The backlinks judge the cursor first, then the page, then the limit
// (M6/P5 design 2).
func TestTheBacklinksJudgeTheCursorThenThePageThenTheLimit(t *testing.T) {
	l := newLibrary()
	bad, zero, big := "x", 0, 101
	if _, err := l.listBacklinks().Execute(reader(), uuid.NewV7(), &zero, &bad); !errors.Is(err, shared.InvalidCursor()) || len(l.calls) > 0 {
		t.Errorf("a bad cursor: %v after %v", err, l.calls)
	}
	if _, err := l.listBacklinks().Execute(reader(), uuid.NewV7(), &zero, nil); !errors.Is(err, domain.ErrPageNotFound) {
		t.Errorf("no page and a bad limit: %v", err)
	}
	l.calls = nil
	for _, limit := range []*int{&zero, &big} {
		var problem *shared.Error
		if _, err := l.listBacklinks().Execute(reader(), l.p, limit, nil); !errors.As(err, &problem) || problem.Code != shared.CodeValidationFailed {
			t.Errorf("limit %d: %v", *limit, err)
		}
	}
	if want := []string{"NotebookOf", "WorkspaceOf", "Authorize backlink.list"}; !slices.Equal(l.calls[:3], want) || slices.Contains(l.calls, "Backlinks") {
		t.Errorf("calls %v", l.calls)
	}
}

// The backlinks come a page at a time, by source: the next cursor is the
// last one's id when more follow, "" otherwise, and reads on from it.
// Each page's contexts are its first links', one a line; a page whose
// content is not of the index's revision, or gone, has none (M6/P5
// design 3).
func TestTheBacklinksComeAPageAtATimeWithTheirContexts(t *testing.T) {
	l := newLibrary()
	ids := []uuid.UUID{uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()}
	slices.SortFunc(ids, uuid.UUID.Compare)
	l.contents = map[uuid.UUID]revised{
		ids[0]: {"see [[p]] and [[p]]\n[[p]] again", 3},
		ids[1]: {"[[p]]", 2},
		ids[2]: {"[[p]]", 1},
	}
	var many []domain.Range
	for i := range 12 {
		many = append(many, domain.Range{Start: 6 + i, End: 7 + i})
	}
	l.backlinks = []app.Backlink{
		{SourceID: ids[0], Revision: 3, Links: 3, Ranges: []domain.Range{{Start: 6, End: 7}, {Start: 16, End: 17}, {Start: 22, End: 23}}},
		{SourceID: ids[1], Revision: 1, Links: 1, Ranges: []domain.Range{{Start: 2, End: 3}}}, // written since
		{SourceID: ids[2], Revision: 1, Links: 12, Ranges: many},                              // past its content
		{SourceID: ids[3], Revision: 1, Links: 1, Ranges: []domain.Range{{Start: 2, End: 3}}}, // gone
	}
	two := 2
	first, err := l.listBacklinks().Execute(reader(), l.p, &two, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []app.Backlinking{
		{PageID: ids[0], Links: 3, Contexts: []string{"see [[p]] and [[p]]", "[[p]] again"}},
		{PageID: ids[1], Links: 1, Contexts: []string{}},
	}
	if next, _ := shared.EncodeCursor(ids[1]); !reflect.DeepEqual(first.Pages, want) || first.NextCursor != next {
		t.Errorf("the first page: %+v\nwant %+v, next %q", first, want, next)
	}
	second, err := l.listBacklinks().Execute(reader(), l.p, &two, &first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	want = []app.Backlinking{{PageID: ids[2], Links: 12, Contexts: []string{}}, {PageID: ids[3], Links: 1, Contexts: []string{}}}
	if !reflect.DeepEqual(second.Pages, want) || second.NextCursor != "" {
		t.Errorf("the last page, exactly full: %+v\nwant %+v", second, want)
	}
	if got, err := l.listBacklinks().Execute(reader(), l.p, nil, nil); err != nil || len(got.Pages) != 4 || got.NextCursor != "" {
		t.Errorf("one page of all: %+v, %v", got, err)
	}
}

// A page's properties are the index's; a page the index does not have
// has a valid frontmatter and none (M6/P5 design 4).
func TestAPagesPropertiesAreTheIndexs(t *testing.T) {
	l := newLibrary()
	if got, err := l.getProperties().Execute(reader(), l.p); err != nil || !reflect.DeepEqual(got, app.Properties{Valid: true}) {
		t.Errorf("not indexed: %+v, %v", got, err)
	}
	l.properties = &app.Properties{
		Properties: []app.Property{{Key: "up", Value: json.RawMessage(`"[[a]]"`)}},
		Links:      []app.PropertyLink{{Key: "up", NodeID: l.q}},
	}
	if got, err := l.getProperties().Execute(reader(), l.p); err != nil || !reflect.DeepEqual(got, *l.properties) {
		t.Errorf("indexed: %+v, %v", got, err)
	}
}

// A tag's pages are read by its key; a name no tag has has none, unread
// (M6/P5 design 5).
func TestATagsPagesAreReadByItsKey(t *testing.T) {
	l := newLibrary()
	l.tagPages = []uuid.UUID{l.p}
	if got, err := l.getTag().Execute(reader(), l.nb, "Proj"); err != nil || !slices.Equal(got, l.tagPages) || !slices.Contains(l.calls, "TagPages proj") {
		t.Errorf("a tag: %v, %v after %v", got, err, l.calls)
	}
	l.calls = nil
	if got, err := l.getTag().Execute(reader(), l.nb, "a b"); err != nil || got == nil || len(got) != 0 || slices.ContainsFunc(l.calls, func(c string) bool { return strings.HasPrefix(c, "TagPages") }) {
		t.Errorf("no tag: %v, %v after %v", got, err, l.calls)
	}
	l.tags = []app.Tag{{Tag: "Proj", Pages: 2}}
	if got, err := (app.ListTags{Access: l.access(), Reads: l.library}).Execute(reader(), l.nb); err != nil || !reflect.DeepEqual(got, l.tags) {
		t.Errorf("tags: %v, %v", got, err)
	}
}

// The link targets are the notebook's pages, by id, each with its title,
// its writing, the path where another page shares its title, and its
// aliases, none as an empty list (M6/P5 design 6).
func TestTheLinkTargetsAreThePagesWithTheirWritings(t *testing.T) {
	l := newLibrary()
	var ids [5]uuid.UUID
	for i := range ids {
		ids[i] = uuid.NewV7()
	}
	step := func(i int, name string) domain.Step {
		return domain.Step{ID: ids[i], Key: strings.ToLower(name), Name: name}
	}
	a, b := step(0, "A"), step(1, "B")
	l.nodes = []domain.Node{
		{ID: ids[0], Path: []domain.Step{a}},
		{ID: ids[1], Path: []domain.Step{b}},
		{ID: ids[2], Path: []domain.Step{a, step(2, "Note")}},
		{ID: ids[3], Path: []domain.Step{b, step(3, "note")}},
		{ID: ids[4], Path: []domain.Step{b, step(4, "Solo")}},
	}
	l.aliases = map[uuid.UUID][]string{ids[4]: {"Alone", "S"}}
	got, err := (app.ListLinkTargets{Access: l.access(), Reads: l.library}).Execute(reader(), l.nb)
	want := []app.LinkTarget{
		{ID: ids[0], Name: "A", Link: "A", Aliases: []string{}},
		{ID: ids[1], Name: "B", Link: "B", Aliases: []string{}},
		{ID: ids[2], Name: "Note", Link: "A/Note", Aliases: []string{}},
		{ID: ids[3], Name: "note", Link: "B/note", Aliases: []string{}},
		{ID: ids[4], Name: "Solo", Link: "Solo", Aliases: []string{"Alone", "S"}},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("link targets: %+v, %v\nwant %+v", got, err, want)
	}
}
