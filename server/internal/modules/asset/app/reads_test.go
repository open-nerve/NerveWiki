package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// authorizer shows the notebooks it sees to every caller, and hides the
// rest; it keeps what it was asked.
type authorizer struct {
	sees   map[uuid.UUID]bool
	asked  []shared.Action
	target []shared.Target
}

func (a *authorizer) Authorize(_ context.Context, _ shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	a.asked, a.target = append(a.asked, action), append(a.target, t)
	if !a.sees[t.NotebookID] {
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{NotebookRole: shared.NotebookReader}, nil
}

// notebooks maps each notebook not deleted to its workspace.
type notebooks map[uuid.UUID]uuid.UUID

func (n notebooks) WorkspaceOf(_ context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	w, ok := n[id]
	return w, ok, nil
}

// treeNodes are the nodes not deleted, the pages among them.
type treeNodes struct {
	nodes map[uuid.UUID]app.Node
	reads int
}

func (t *treeNodes) Node(_ context.Context, id uuid.UUID) (app.Node, bool, error) {
	t.reads++
	n, ok := t.nodes[id]
	return n, ok, nil
}

func (t *treeNodes) Parent(_ context.Context, notebookID, parentID uuid.UUID) (bool, error) {
	n, ok := t.nodes[parentID]
	return ok && !n.Asset && n.NotebookID == notebookID, nil
}

func (t *treeNodes) Assets(_ context.Context, notebookID uuid.UUID, parentID *uuid.UUID, after *app.Cursor, limit int) ([]app.Node, error) {
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
	books notebooks
	nodes *treeNodes
	rows  *memRows
	logs  *bytes.Buffer
	reads *app.Reads
	eng   uuid.UUID
	acme  uuid.UUID
	intro uuid.UUID
	ctx   context.Context
}

func newReader() *reader {
	r := &reader{eng: uuid.NewV7(), acme: uuid.NewV7(), intro: uuid.NewV7(), rows: newRows(), logs: &bytes.Buffer{}}
	r.auth, r.books = &authorizer{sees: map[uuid.UUID]bool{r.eng: true}}, notebooks{r.eng: r.acme}
	r.nodes = &treeNodes{nodes: map[uuid.UUID]app.Node{r.intro: {ID: r.intro, NotebookID: r.eng, Name: "Intro", NameKey: "intro"}}}
	r.reads = app.NewReads(app.ReadsDeps{Authorizer: r.auth, Notebooks: r.books, Nodes: r.nodes, Rows: r.rows,
		Signer: app.NewSigner(signKey(), fixedClock{now()}), Logger: slog.New(slog.NewTextHandler(r.logs, nil))})
	r.ctx = shared.WithActor(context.Background(), shared.Actor{UserID: uuid.NewV7(), SessionID: uuid.NewV7()})
	return r
}

// hidden is a notebook of acme that alice cannot see.
func (r *reader) hidden() uuid.UUID {
	id := uuid.NewV7()
	r.books[id] = r.acme
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

// Get answers the attachment and its row, signed, decided on asset.read
// in its notebook's workspace.
func TestGetReadsTheAttachmentSigned(t *testing.T) {
	r := newReader()
	n := r.attach(r.eng, &r.intro, "a.png", false)
	a, err := r.reads.Get(r.ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	b := r.rows.rows[n.ID]
	if a.Node != n || a.Blob.ID != b.ID || a.Signed != app.NewSigner(signKey(), fixedClock{now()}).Sign(n.ID, b.ID) {
		t.Errorf("Get() = %+v, want the node, its row, their address signed", a)
	}
	if !slices.Equal(r.auth.asked, []shared.Action{domain.ActionRead}) || r.auth.target[0] != (shared.Target{WorkspaceID: r.acme, NotebookID: r.eng}) {
		t.Errorf("asked %v on %v, want asset.read on eng of acme", r.auth.asked, r.auth.target)
	}
}

// A node that is none, a page, of a notebook gone or hidden, or without
// its row is asset.not_found; a node without its row is logged as an
// error.
func TestGetAnswersNotFound(t *testing.T) {
	for _, tt := range []struct {
		name   string
		id     func(r *reader) uuid.UUID
		logged bool
	}{
		{"no node", func(*reader) uuid.UUID { return uuid.NewV7() }, false},
		{"a page", func(r *reader) uuid.UUID { return r.intro }, false},
		{"a notebook gone", func(r *reader) uuid.UUID { return r.attach(uuid.NewV7(), nil, "a.png", false).ID }, false},
		{"a notebook hidden", func(r *reader) uuid.UUID { return r.attach(r.hidden(), nil, "a.png", false).ID }, false},
		{"a node without its row", func(r *reader) uuid.UUID { return r.attach(r.eng, nil, "a.png", true).ID }, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newReader()
			if _, err := r.reads.Get(r.ctx, tt.id(r)); !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("Get() = %v, want asset.not_found", err)
			}
			if logged := strings.Contains(r.logs.String(), "level=ERROR"); logged != tt.logged {
				t.Errorf("logs %q, want an error logged: %v", r.logs, tt.logged)
			}
		})
	}
}

// List answers the attachments under a parent, by name key and id, a page
// at a time; a node without its row is left out and logged.
func TestListPagesTheAttachments(t *testing.T) {
	r := newReader()
	c := r.attach(r.eng, &r.intro, "C.png", false)
	r.attach(r.eng, &r.intro, "a.png", false)
	b := r.attach(r.eng, &r.intro, "b.png", true)
	r.attach(r.eng, &r.intro, "d.png", false)
	r.attach(r.eng, nil, "root.png", false)
	two := 2
	first, err := r.reads.List(r.ctx, r.eng, &r.intro, &two, nil)
	if err != nil || first.NextCursor == "" || !slices.Equal(names(first), []string{"a.png"}) {
		t.Fatalf("List() = %v, %q, %v; want a.png, b.png left out, and a cursor", names(first), first.NextCursor, err)
	}
	second, err := r.reads.List(r.ctx, r.eng, &r.intro, &two, &first.NextCursor)
	if err != nil || second.NextCursor != "" || !slices.Equal(names(second), []string{"C.png", "d.png"}) {
		t.Errorf("List(the cursor) = %v, %q, %v; want C.png, d.png, the last page", names(second), second.NextCursor, err)
	}
	if s := second.Assets[0]; s.Node != c || s.Signed != app.NewSigner(signKey(), fixedClock{now()}).Sign(c.ID, r.rows.rows[c.ID].ID) {
		t.Errorf("listed %+v, want C.png signed", s)
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
