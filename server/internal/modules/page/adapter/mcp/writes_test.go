package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// writeFunc is a WritePath of a function, and so are the others of the
// writes.
type (
	writeFunc  func(notebookID uuid.UUID, p app.PathWrite, client domain.Client) (app.Written, error)
	editFunc   func(notebookID uuid.UUID, p app.PageEdit, client domain.Client) (app.Edited, error)
	moveFunc   func(notebookID uuid.UUID, p app.PathMove, client domain.Client) (app.Relocated, error)
	deleteFunc func(notebookID uuid.UUID, p app.PathDelete, client domain.Client) (app.Deleted, error)
)

func (f writeFunc) Execute(_ context.Context, id uuid.UUID, p app.PathWrite, c domain.Client) (app.Written, error) {
	return f(id, p, c)
}

func (f editFunc) Execute(_ context.Context, id uuid.UUID, p app.PageEdit, c domain.Client) (app.Edited, error) {
	return f(id, p, c)
}

func (f moveFunc) Execute(_ context.Context, id uuid.UUID, p app.PathMove, c domain.Client) (app.Relocated, error) {
	return f(id, p, c)
}

func (f deleteFunc) Execute(_ context.Context, id uuid.UUID, p app.PathDelete, c domain.Client) (app.Deleted, error) {
	return f(id, p, c)
}

// structuredOf is a structured result as JSON: Fit's encoded text, or the
// answer mcpserver encodes.
func structuredOf(t *testing.T, res mcpserver.Result) string {
	t.Helper()
	if raw, ok := res.Structured.(json.RawMessage); ok {
		return string(raw)
	}
	b, err := json.Marshal(res.Structured)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The writes pass their call's notebook, their arguments with their
// defaults (replace_all and recursive false, update_links true) and the
// agent's client, mcp: and its name, to their use cases (M9/P2 design 3.8,
// 3.11).
func TestTheWritesPassTheirArguments(t *testing.T) {
	notebook, base := uuid.NewV7(), "x:1"
	call := mcpserver.Call{Notebook: notebook, Client: "Claude Code"}
	client := domain.Client("mcp:Claude Code")
	var got []any
	stop := errors.New("stop")
	tools := tools{uc: UseCases{
		Write: writeFunc(func(id uuid.UUID, p app.PathWrite, c domain.Client) (app.Written, error) {
			got = []any{id, p.Path, p.Content, p.Base, c}
			return app.Written{}, stop
		}),
		Edit: editFunc(func(id uuid.UUID, p app.PageEdit, c domain.Client) (app.Edited, error) {
			got = []any{id, p.Path, p.Old, p.New, p.All, p.Base, c}
			return app.Edited{}, stop
		}),
		Move: moveFunc(func(id uuid.UUID, p app.PathMove, c domain.Client) (app.Relocated, error) {
			got = []any{id, p.From, p.To, p.UpdateLinks, c}
			return app.Relocated{}, stop
		}),
		Delete: deleteFunc(func(id uuid.UUID, p app.PathDelete, c domain.Client) (app.Deleted, error) {
			got = []any{id, p.Path, p.Recursive, c}
			return app.Deleted{}, stop
		}),
	}}
	ctx := context.Background()
	yes, no := true, false
	for _, tt := range []struct {
		name string
		call func() error
		want []any
	}{
		{"write", func() error {
			_, err := tools.write(ctx, call, writeIn{Path: "A.md", Content: "a", BaseVersion: &base})
			return err
		}, []any{notebook, "A.md", "a", &base, client}},
		{"edit", func() error {
			_, err := tools.edit(ctx, call, editIn{Path: "A.md", OldString: "a", NewString: "b"})
			return err
		}, []any{notebook, "A.md", "a", "b", false, (*string)(nil), client}},
		{"edit with replace_all", func() error {
			_, err := tools.edit(ctx, call, editIn{Path: "A.md", OldString: "a", NewString: "b", ReplaceAll: &yes, BaseVersion: &base})
			return err
		}, []any{notebook, "A.md", "a", "b", true, &base, client}},
		{"move", func() error {
			_, err := tools.move(ctx, call, moveIn{From: "A.md", To: "B.md"})
			return err
		}, []any{notebook, "A.md", "B.md", true, client}},
		{"move without its links", func() error {
			_, err := tools.move(ctx, call, moveIn{From: "A.md", To: "B.md", UpdateLinks: &no})
			return err
		}, []any{notebook, "A.md", "B.md", false, client}},
		{"delete", func() error {
			_, err := tools.delete(ctx, call, deleteIn{Path: "A.md"})
			return err
		}, []any{notebook, "A.md", false, client}},
		{"delete with recursive", func() error {
			_, err := tools.delete(ctx, call, deleteIn{Path: "A.md", Recursive: &yes})
			return err
		}, []any{notebook, "A.md", true, client}},
	} {
		got = nil
		if err := tt.call(); !errors.Is(err, stop) || len(got) != len(tt.want) {
			t.Errorf("%s = %v, passing %v", tt.name, err, got)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s passed %v, want %v", tt.name, got, tt.want)
				break
			}
		}
	}
}

// The writes answer what their use cases did (M9/P2 design 3.11): the
// page's path by its names, its node and version, changed, and the
// changeset and its time in UTC, both null when nothing changed; write
// the nodes it created, edit how many it replaced, delete what went to
// the trash.
func TestTheWritesAnswerWhatTheyDid(t *testing.T) {
	node, changeset := domain.Node{ID: uuid.NewV7()}, uuid.NewV7()
	at := time.Date(2026, 10, 18, 12, 0, 0, 123456000, time.FixedZone("x", 3600))
	done := app.Outcome{ChangesetID: changeset, At: at}
	version := domain.VersionToken{Node: node.ID, Revision: 2}
	parent := domain.Node{ID: uuid.NewV7()}
	when := `"changeset_id":"` + changeset.String() + `","at":"2026-10-18T11:00:00.123456Z"`
	tools := tools{uc: UseCases{
		Write: writeFunc(func(_ uuid.UUID, p app.PathWrite, _ domain.Client) (app.Written, error) {
			w := app.Written{Path: "A/B.md", Node: node, Version: version}
			if p.Base == nil {
				w.Created, w.Outcome = []app.Placed{{Path: "A.md", Node: parent}, {Path: "A/B.md", Node: node}}, done
			}
			return w, nil
		}),
		Edit: editFunc(func(uuid.UUID, app.PageEdit, domain.Client) (app.Edited, error) {
			return app.Edited{Path: "A/B.md", Node: node, Version: version, Replacements: 3, Outcome: done}, nil
		}),
		Move: moveFunc(func(uuid.UUID, app.PathMove, domain.Client) (app.Relocated, error) {
			return app.Relocated{Path: "C/B.md", Node: node}, nil
		}),
		Delete: deleteFunc(func(uuid.UUID, app.PathDelete, domain.Client) (app.Deleted, error) {
			return app.Deleted{Path: "A/B.md", Node: node, Pages: 2, Assets: 1, Outcome: done}, nil
		}),
	}}
	ctx, base := context.Background(), "x:1"
	answer := func(res mcpserver.Result, err error) string {
		if err != nil {
			return err.Error()
		}
		return structuredOf(t, res)
	}
	for _, tt := range []struct {
		name, got, want string
	}{
		{"a write that created", answer(tools.write(ctx, mcpserver.Call{}, writeIn{Path: "a/b.md"})),
			`{"path":"A/B.md","node_id":"` + node.ID.String() + `","version":"` + version.String() + `","changed":true,"created":[` +
				`{"path":"A.md","node_id":"` + parent.ID.String() + `"},{"path":"A/B.md","node_id":"` + node.ID.String() + `"}],` +
				`"created_count":2,` + when + `}`},
		{"a write that changed nothing", answer(tools.write(ctx, mcpserver.Call{}, writeIn{Path: "a/b.md", BaseVersion: &base})),
			`{"path":"A/B.md","node_id":"` + node.ID.String() + `","version":"` + version.String() + `","changed":false,"created":[],` +
				`"created_count":0,"changeset_id":null,"at":null}`},
		{"an edit", answer(tools.edit(ctx, mcpserver.Call{}, editIn{Path: "a/b.md", OldString: "a", NewString: "b"})),
			`{"path":"A/B.md","node_id":"` + node.ID.String() + `","version":"` + version.String() + `","replacements":3,"changed":true,` +
				when + `}`},
		{"a move that moved nothing", answer(tools.move(ctx, mcpserver.Call{}, moveIn{From: "C/B.md", To: "c/b.md"})),
			`{"path":"C/B.md","node_id":"` + node.ID.String() + `","changed":false,"changeset_id":null,"at":null}`},
		{"a deletion", answer(tools.delete(ctx, mcpserver.Call{}, deleteIn{Path: "a/b.md"})),
			`{"path":"A/B.md","node_id":"` + node.ID.String() + `","pages":2,"attachments":1,` + when + `}`},
	} {
		if tt.got != tt.want {
			t.Errorf("%s answered %s, want %s", tt.name, tt.got, tt.want)
		}
	}
}

// write's answer holds as many of the nodes it created as fit, each a
// path, and counts them all (M9/P2 design 3.11.1): ten paths of 2,818
// bytes of "&" do not fit, the first ones do.
func TestAWriteFitsTheNodesItCreated(t *testing.T) {
	segment := strings.Repeat("&", 255)
	var created []app.Placed
	path := ""
	for i := range 10 {
		if i > 0 {
			path += "/"
		}
		path += segment
		created = append(created, app.Placed{Path: path + ".md", Node: domain.Node{ID: uuid.NewV7()}})
	}
	last := created[len(created)-1]
	tools := tools{uc: UseCases{Write: writeFunc(func(uuid.UUID, app.PathWrite, domain.Client) (app.Written, error) {
		return app.Written{Path: last.Path, Node: last.Node, Created: created, Outcome: app.Outcome{ChangesetID: uuid.NewV7()}}, nil
	})}}
	res, err := tools.write(context.Background(), mcpserver.Call{}, writeIn{Path: last.Path})
	if err != nil {
		t.Fatal(err)
	}
	var a writeAnswer
	if err := json.Unmarshal([]byte(rawOf(t, res)), &a); err != nil {
		t.Fatal(err)
	}
	if a.CreatedCount != 10 || len(a.Created) == 0 || len(a.Created) == 10 || a.Created[0].Path != created[0].Path ||
		mcpserver.TextBytes(rawOf(t, res)) > mcpserver.MaxAnswerBytes {
		t.Errorf("the answer holds %d of %d created, within %d bytes", len(a.Created), a.CreatedCount, mcpserver.TextBytes(rawOf(t, res)))
	}
}

// Each write words its errors with its own path: write's and move's taken
// paths each in their tool's words, delete's recursive and a move's locks
// quoting the path the call names.
func TestTheWritesWordTheirErrors(t *testing.T) {
	taken := &domain.PathTaken{Path: "B.md"}
	tools := tools{uc: UseCases{
		Write: writeFunc(func(uuid.UUID, app.PathWrite, domain.Client) (app.Written, error) { return app.Written{}, taken }),
		Move:  moveFunc(func(uuid.UUID, app.PathMove, domain.Client) (app.Relocated, error) { return app.Relocated{}, taken }),
		Delete: deleteFunc(func(uuid.UUID, app.PathDelete, domain.Client) (app.Deleted, error) {
			return app.Deleted{}, &domain.Recursive{Field: "recursive", Pages: 1, Assets: 1}
		}),
		Edit: editFunc(func(uuid.UUID, app.PageEdit, domain.Client) (app.Edited, error) {
			return app.Edited{}, &domain.LockPaths{Err: domain.Locked(uuid.NewV7(), uuid.NewV7(), "Bob")}
		}),
	}}
	ctx := context.Background()
	_, werr := tools.write(ctx, mcpserver.Call{}, writeIn{Path: "B.md"})
	_, merr := tools.move(ctx, mcpserver.Call{}, moveIn{From: "A.md", To: "B.md"})
	_, derr := tools.delete(ctx, mcpserver.Call{}, deleteIn{Path: "Guides.md"})
	_, eerr := tools.edit(ctx, mcpserver.Call{}, editIn{Path: "Notes.md", OldString: "a", NewString: "b"})
	for _, tt := range []struct {
		err  error
		want string
	}{
		{werr, `page.title_taken: "B.md" is a page already.`},
		{merr, `page.title_taken: "B.md" is taken: a page there has that title.`},
		{derr, `validation_failed: "Guides.md" has 1 page and 1 attachment under it:`},
		{eerr, `page.locked: "Notes.md" is being edited by Bob in the web editor;`},
	} {
		if text, _ := wordsOf(tt.err); !strings.HasPrefix(text, tt.want) {
			t.Errorf("%v worded %q, want it to start with %q", tt.err, text, tt.want)
		}
	}
}
