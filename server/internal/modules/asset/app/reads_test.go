package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/mac"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// authorizer shows the notebooks it sees to every caller, and hides the
// rest; it keeps what it was asked, and fails with err when set.
type authorizer struct {
	sees   map[uuid.UUID]bool
	asked  []shared.Action
	target []shared.Target
	err    error
}

func (a *authorizer) Authorize(_ context.Context, _ shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	a.asked, a.target = append(a.asked, action), append(a.target, t)
	if a.err != nil {
		return shared.Grant{}, a.err
	}
	if !a.sees[t.NotebookID] {
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{NotebookRole: shared.NotebookReader}, nil
}

// notebooks maps each notebook not deleted to its workspace; it fails
// with err when set, answering nothing as the notebook module does.
type notebooks struct {
	of  map[uuid.UUID]uuid.UUID
	err error
}

func (n *notebooks) WorkspaceOf(_ context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	if n.err != nil {
		return uuid.UUID{}, false, n.err
	}
	w, ok := n.of[id]
	return w, ok, nil
}

// treeNodes are the nodes not deleted, the pages among them; the node
// vanishing is deleted once read. Node fails past failsAfter reads when
// it is set; Parent and Assets with parentErr and assetsErr.
type treeNodes struct {
	nodes      map[uuid.UUID]app.Node
	vanishing  uuid.UUID
	reads      int
	failsAfter int
	parentErr  error
	assetsErr  error
}

func (t *treeNodes) Node(_ context.Context, id uuid.UUID) (app.Node, bool, error) {
	t.reads++
	if t.failsAfter > 0 && t.reads > t.failsAfter {
		return app.Node{}, false, errPort
	}
	n, ok := t.nodes[id]
	if id == t.vanishing {
		delete(t.nodes, id)
	}
	return n, ok, nil
}

func (t *treeNodes) Parent(_ context.Context, notebookID, parentID uuid.UUID) (bool, error) {
	if t.parentErr != nil {
		return false, t.parentErr
	}
	n, ok := t.nodes[parentID]
	return ok && !n.Asset && n.NotebookID == notebookID, nil
}

func (t *treeNodes) Assets(_ context.Context, notebookID uuid.UUID, parentID *uuid.UUID, after *app.Cursor, limit int) ([]app.Node, error) {
	if t.assetsErr != nil {
		return nil, t.assetsErr
	}
	var out []app.Node
	for _, n := range t.nodes {
		if n.Asset && n.NotebookID == notebookID && (parentID == nil) == (n.ParentID == nil) &&
			(parentID == nil || *parentID == *n.ParentID) &&
			(after == nil || n.NameKey > after.NameKey || n.NameKey == after.NameKey && n.ID.Compare(after.ID) > 0) {
			out = append(out, n)
		}
	}
	slices.SortFunc(out, func(a, b app.Node) int {
		if c := strings.Compare(a.NameKey, b.NameKey); c != 0 {
			return c
		}
		return a.ID.Compare(b.ID)
	})
	return out[:min(limit, len(out))], nil
}

// reader is Reads over the fakes: alice reads in eng, of acme, whose root
// holds a page, intro, with attachments.
type reader struct {
	auth  *authorizer
	books *notebooks
	nodes *treeNodes
	rows  *memRows
	links *links
	clock *clock
	logs  *bytes.Buffer
	reads *app.Reads
	eng   uuid.UUID
	acme  uuid.UUID
	intro uuid.UUID
	ctx   context.Context
}

func newReader() *reader {
	r := &reader{eng: uuid.NewV7(), acme: uuid.NewV7(), intro: uuid.NewV7(), rows: newRows(), links: &links{}, clock: &clock{},
		logs: &bytes.Buffer{}}
	r.auth, r.books = &authorizer{sees: map[uuid.UUID]bool{r.eng: true}}, &notebooks{of: map[uuid.UUID]uuid.UUID{r.eng: r.acme}}
	r.nodes = &treeNodes{nodes: map[uuid.UUID]app.Node{r.intro: {ID: r.intro, NotebookID: r.eng, Name: "Intro", NameKey: "intro"}}}
	r.reads = app.NewReads(app.ReadsDeps{Authorizer: r.auth, Notebooks: r.books, Nodes: r.nodes, Rows: r.rows,
		Signer: macadapter.New(signKey()), Links: r.links, Clock: r.clock, Logger: slog.New(slog.NewTextHandler(r.logs, nil))})
	r.ctx = shared.WithActor(context.Background(), shared.Actor{UserID: uuid.NewV7(), SessionID: uuid.NewV7()})
	return r
}

// hidden is a notebook of acme that alice cannot see.
func (r *reader) hidden() uuid.UUID {
	id := uuid.NewV7()
	r.books.of[id] = r.acme
	return id
}

// attach adds an attachment named name under parent (nil: the root) of
// notebook, with its row unless rowless.
func (r *reader) attach(notebook uuid.UUID, parent *uuid.UUID, name string, rowless bool) app.Node {
	n := app.Node{ID: uuid.NewV7(), NotebookID: notebook, ParentID: parent, Asset: true, Name: name, NameKey: strings.ToLower(name)}
	r.nodes.nodes[n.ID] = n
	if !rowless {
		r.rows.rows[n.ID] = domain.Blob{ID: uuid.NewV7(), NodeID: n.ID, NotebookID: notebook, MIME: "image/png", Bytes: 3}
	}
	return n
}

// Get answers the attachment and its row, signed, with its link, decided
// on asset.read in its notebook's workspace.
func TestGetReadsTheAttachmentSigned(t *testing.T) {
	r := newReader()
	n := r.attach(r.eng, &r.intro, "a.png", false)
	a, err := r.reads.Get(r.ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	b := r.rows.rows[n.ID]
	if a.Node != n || a.Blob.ID != b.ID || a.Signed != macadapter.New(signKey()).Sign(now(), n.ID, b.ID) || a.Link != linkOf(n.ID) {
		t.Errorf("Get() = %+v, want the node, its row, their address signed, its link", a)
	}
	if len(r.links.asked) != 1 || !slices.Equal(r.links.asked[0], []uuid.UUID{n.ID}) || r.links.notebooks[0] != r.eng {
		t.Errorf("links asked %v of %v, want the node's of eng", r.links.asked, r.links.notebooks)
	}
	if !slices.Equal(r.auth.asked, []shared.Action{domain.ActionRead}) || r.auth.target[0] != (shared.Target{WorkspaceID: r.acme, NotebookID: r.eng}) {
		t.Errorf("asked %v on %v, want asset.read on eng of acme", r.auth.asked, r.auth.target)
	}
}

// A node that is none, a page, of a notebook gone or hidden, or without
// its row is asset.not_found, decided on only for an attachment of a
// notebook; a node without its row is logged as an error, with its ids,
// unless it was deleted, with its row, since it was read: a read again
// that fails logs it.
func TestGetAnswersNotFound(t *testing.T) {
	for _, tt := range []struct {
		name            string
		id              func(r *reader) uuid.UUID
		decided, logged bool
	}{
		{"no node", func(*reader) uuid.UUID { return uuid.NewV7() }, false, false},
		{"a page", func(r *reader) uuid.UUID { return r.intro }, false, false},
		{"a notebook gone", func(r *reader) uuid.UUID { return r.attach(uuid.NewV7(), nil, "a.png", false).ID }, false, false},
		{"a notebook hidden", func(r *reader) uuid.UUID { return r.attach(r.hidden(), nil, "a.png", false).ID }, true, false},
		{"a node without its row", func(r *reader) uuid.UUID { return r.attach(r.eng, nil, "a.png", true).ID }, true, true},
		{"a node deleted since it was read", func(r *reader) uuid.UUID {
			r.nodes.vanishing = r.attach(r.eng, nil, "a.png", true).ID
			return r.nodes.vanishing
		}, true, false},
		{"a node without its row, not read again", func(r *reader) uuid.UUID {
			r.nodes.failsAfter = 1
			return r.attach(r.eng, nil, "a.png", true).ID
		}, true, true},
		{"a node deleted before its link is read", func(r *reader) uuid.UUID {
			id := r.attach(r.eng, nil, "a.png", false).ID
			r.links.missing = map[uuid.UUID]bool{id: true}
			return id
		}, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newReader()
			id := tt.id(r)
			if _, err := r.reads.Get(r.ctx, id); !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("Get() = %v, want asset.not_found", err)
			}
			if decided := len(r.auth.asked) > 0; decided != tt.decided {
				t.Errorf("decided: %v, want %v", decided, tt.decided)
			}
			l := r.logs.String()
			if logged := strings.Contains(l, "level=ERROR"); logged != tt.logged ||
				logged && (!strings.Contains(l, "node_id="+id.String()) || !strings.Contains(l, "notebook_id="+r.eng.String())) {
				t.Errorf("logs %q, want an error logged with its ids: %v", l, tt.logged)
			}
		})
	}
}

// A read without an actor is unauthenticated.
func TestReadsRequireAnActor(t *testing.T) {
	r := newReader()
	n := r.attach(r.eng, nil, "a.png", false)
	var se *shared.Error
	if _, err := r.reads.Get(context.Background(), n.ID); !errors.As(err, &se) || se.Kind != shared.KindUnauthenticated {
		t.Errorf("Get(no actor) = %v, want unauthenticated", err)
	}
	if _, err := r.reads.List(context.Background(), r.eng, nil, nil, nil); !errors.As(err, &se) || se.Kind != shared.KindUnauthenticated {
		t.Errorf("List(no actor) = %v, want unauthenticated", err)
	}
}

// Each port's failure is the read's, not a not_found nor an empty page.
func TestReadsAnswerEachPortsFailure(t *testing.T) {
	for _, tt := range []struct {
		name string
		fail func(r *reader)
		get  bool
	}{
		{"the notebook's read", func(r *reader) { r.books.err = errPort }, true},
		{"the decision", func(r *reader) { r.auth.err = errPort }, true},
		{"the rows' read", func(r *reader) { r.rows.readErr = errPort }, true},
		{"the parent's read", func(r *reader) { r.nodes.parentErr = errPort }, false},
		{"the attachments' read", func(r *reader) { r.nodes.assetsErr = errPort }, false},
		{"the links' read", func(r *reader) { r.links.err = errPort }, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newReader()
			n := r.attach(r.eng, &r.intro, "a.png", false)
			tt.fail(r)
			if _, err := r.reads.Get(r.ctx, n.ID); tt.get && !errors.Is(err, errPort) {
				t.Errorf("Get() = %v, want the port's failure", err)
			}
			if page, err := r.reads.List(r.ctx, r.eng, &r.intro, nil, nil); !errors.Is(err, errPort) {
				t.Errorf("List() = %v, %v; want the port's failure", names(page), err)
			}
		})
	}
}

// A list reads the clock once: each attachment is signed as of that time.
func TestListSignsAsOfOneTime(t *testing.T) {
	r := newReader()
	r.attach(r.eng, &r.intro, "a.png", false)
	r.attach(r.eng, &r.intro, "b.png", false)
	r.clock.step = time.Hour
	page, err := r.reads.List(r.ctx, r.eng, &r.intro, nil, nil)
	if err != nil || len(page.Assets) != 2 || r.clock.reads != 1 || !page.Assets[0].Signed.Expires.Equal(page.Assets[1].Signed.Expires) {
		t.Errorf("List() = %+v, %v after %d reads of the clock; want both signed as of one read", page.Assets, err, r.clock.reads)
	}
}

// List answers the attachments under a parent, by name key and id, a page
// at a time, each with its link, read once a page; a node without its row
// is left out and logged, one deleted before the links were read is left
// out.
func TestListPagesTheAttachments(t *testing.T) {
	r := newReader()
	c := r.attach(r.eng, &r.intro, "C.png", false)
	r.attach(r.eng, &r.intro, "a.png", false)
	b := r.attach(r.eng, &r.intro, "b.png", true)
	d := r.attach(r.eng, &r.intro, "d.png", false)
	gone := r.attach(r.eng, &r.intro, "e.png", false)
	r.attach(r.eng, nil, "root.png", false)
	r.links.missing = map[uuid.UUID]bool{gone.ID: true}
	two := 2
	first, err := r.reads.List(r.ctx, r.eng, &r.intro, &two, nil)
	if err != nil || first.NextCursor == "" || !slices.Equal(names(first), []string{"a.png"}) {
		t.Fatalf("List() = %v, %q, %v; want a.png, b.png left out, and a cursor", names(first), first.NextCursor, err)
	}
	second, err := r.reads.List(r.ctx, r.eng, &r.intro, &two, &first.NextCursor)
	if err != nil || second.NextCursor == "" || !slices.Equal(names(second), []string{"C.png", "d.png"}) {
		t.Fatalf("List(the cursor) = %v, %q, %v; want C.png, d.png, and a cursor", names(second), second.NextCursor, err)
	}
	if s := second.Assets[0]; s.Node != c || s.Signed != macadapter.New(signKey()).Sign(now(), c.ID, r.rows.rows[c.ID].ID) ||
		s.Link != linkOf(c.ID) {
		t.Errorf("listed %+v, want C.png signed, with its link", s)
	}
	if asked := r.links.asked[len(r.links.asked)-1]; !slices.Equal(asked, []uuid.UUID{c.ID, d.ID}) || r.links.notebooks[0] != r.eng {
		t.Errorf("links asked %v of %v, want C.png's and d.png's of eng, at once", asked, r.links.notebooks)
	}
	third, err := r.reads.List(r.ctx, r.eng, &r.intro, &two, &second.NextCursor)
	if err != nil || third.NextCursor != "" || len(third.Assets) != 0 {
		t.Errorf("List(the second cursor) = %v, %q, %v; want e.png, deleted since, left out", names(third), third.NextCursor, err)
	}
	if l := r.logs.String(); !strings.Contains(l, "level=ERROR") || !strings.Contains(l, b.ID.String()) {
		t.Errorf("logs %q, want b.png's node logged as an error", l)
	}
	root, err := r.reads.List(r.ctx, r.eng, nil, nil, nil)
	if err != nil || !slices.Equal(names(root), []string{"root.png"}) {
		t.Errorf("List(the root) = %v, %v; want root.png", names(root), err)
	}
}

func names(p app.AssetPage) []string {
	var out []string
	for _, a := range p.Assets {
		out = append(out, a.Node.Name)
	}
	return out
}

// List judges the cursor first, then the decision, the parent, the
// limit.
func TestListAnswersItsFailuresInOrder(t *testing.T) {
	bad, zero := "nope", 0
	for _, tt := range []struct {
		name     string
		notebook func(r *reader) uuid.UUID
		parent   func(r *reader) *uuid.UUID
		limit    *int
		cursor   *string
		want     error
		decided  bool
	}{
		{"a cursor it cannot read, in a hidden notebook", func(*reader) uuid.UUID { return uuid.NewV7() }, nil, &zero, &bad,
			shared.InvalidCursor(), false},
		{"a notebook gone", func(*reader) uuid.UUID { return uuid.NewV7() }, nil, nil, nil, domain.ErrNotebookNotFound, false},
		{"a hidden notebook, under no page", func(r *reader) uuid.UUID { return r.hidden() }, func(*reader) *uuid.UUID { p := uuid.NewV7(); return &p },
			&zero, nil, domain.ErrNotebookNotFound, true},
		{"a parent that is no page, a limit of 0", func(r *reader) uuid.UUID { return r.eng }, func(*reader) *uuid.UUID { p := uuid.NewV7(); return &p },
			&zero, nil, domain.ErrParentNotFound, true},
		{"a parent that is an attachment", func(r *reader) uuid.UUID { return r.eng }, func(r *reader) *uuid.UUID {
			id := r.attach(r.eng, nil, "a.png", false).ID
			return &id
		}, nil, nil, domain.ErrParentNotFound, true},
		{"a limit of 0", func(r *reader) uuid.UUID { return r.eng }, nil, &zero, nil, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newReader()
			var parent *uuid.UUID
			if tt.parent != nil {
				parent = tt.parent(r)
			}
			_, err := r.reads.List(r.ctx, tt.notebook(r), parent, tt.limit, tt.cursor)
			var se *shared.Error
			switch {
			case tt.want == nil && (!errors.As(err, &se) || se.Kind != shared.KindInvalid):
				t.Errorf("List() = %v, want validation_failed", err)
			case tt.want != nil && !errors.Is(err, tt.want):
				t.Errorf("List() = %v, want %v", err, tt.want)
			}
			if decided := len(r.auth.asked) > 0; decided != tt.decided {
				t.Errorf("decided: %v, want %v", decided, tt.decided)
			}
		})
	}
}
