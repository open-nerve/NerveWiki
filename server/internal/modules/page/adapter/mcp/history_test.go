package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// versionsFunc is a Versions of a function, and so are the history's
// others.
type (
	versionsFunc func(notebookID uuid.UUID, path string, limit *int, cursor *string) (app.PathVersionList, error)
	versionFunc  func(notebookID uuid.UUID, path string, revision int, offset, column, limit *int, m app.TextMeasure) (
		app.PathVersion, error)
	logFunc       func(notebookID uuid.UUID, q app.PathLogQuery) (app.PathLog, error)
	changesetFunc func(notebookID, id uuid.UUID, limit *int, cursor *string) (app.ChangesetPage, error)
)

func (f versionsFunc) AtPath(_ context.Context, id uuid.UUID, path string, limit *int, cursor *string) (app.PathVersionList, error) {
	return f(id, path, limit, cursor)
}

func (f versionFunc) AtPath(_ context.Context, id uuid.UUID, path string, revision int, offset, column, limit *int,
	m app.TextMeasure,
) (app.PathVersion, error) {
	return f(id, path, revision, offset, column, limit, m)
}

func (f logFunc) AtPath(_ context.Context, id uuid.UUID, q app.PathLogQuery) (app.PathLog, error) {
	return f(id, q)
}

func (f changesetFunc) InNotebook(_ context.Context, notebookID, id uuid.UUID, limit *int, cursor *string) (app.ChangesetPage, error) {
	return f(notebookID, id, limit, cursor)
}

// worstValues are the history's worst values (M9/P2 design 3.16.4): a
// message of 4,096 bytes each written in seven, a display name of 100
// characters and a title of 255 each written in six, an MCP client's
// longest name, a path of eleven such segments, a time to the
// microsecond.
type worstValues struct {
	message, name, title, path string
	client                     domain.Client
	at                         time.Time
}

func worst() worstValues {
	title := strings.Repeat("&", 255)
	return worstValues{message: strings.Repeat("\x01", domain.MaxMessageBytes), name: strings.Repeat("&", 100), title: title,
		path: strings.Repeat(title+"/", 10) + title + ".md", client: domain.Client("mcp:" + strings.Repeat("&", 128)),
		at: time.Date(2026, 10, 18, 12, 34, 56, 123456000, time.UTC)}
}

// worstChangeset is a changeset of the worst values, with three changes:
// each a move of a page of the longest names, between parents of the
// longest names, writing a version too.
func worstChangeset() app.LoggedChangeset {
	reverted := make([]uuid.UUID, domain.MaxRevertedBy)
	for i := range reverted {
		reverted[i] = uuid.NewV7()
	}
	changes := make([]domain.ChangeRow, 3)
	for i := range changes {
		changes[i] = worstChange()
	}
	return app.LoggedChangeset{Changeset: domain.Changeset{ID: uuid.NewV7(), NotebookID: uuid.NewV7(),
		Kind: domain.ChangesetVersionRestore, Client: worst().client, Message: new(worst().message), By: uuid.NewV7(), ByName: worst().name,
		CreatedAt: worst().at, UpdatedAt: worst().at, Trimmed: true, Reverts: &domain.ChangesetBrief{ID: uuid.NewV7(),
			Kind: domain.ChangesetVersionRestore, By: uuid.NewV7(), ByName: worst().name, CreatedAt: worst().at}},
		Facts: domain.ChangesetFacts{Nodes: domain.MaxLogCount, Contents: domain.MaxLogCount, RevertedBy: reverted}, Changes: changes}
}

// worstChange is a move of a page of the longest names, between parents
// of the longest names, that wrote a version too.
func worstChange() domain.ChangeRow {
	place := func() *domain.ChangePlace {
		return &domain.ChangePlace{ParentID: new(uuid.NewV7()), ParentName: new(worst().title), Name: worst().title}
	}
	return domain.ChangeRow{NodeID: uuid.NewV7(), NodeKind: domain.KindPage, Name: worst().title, Alive: true, Before: place(),
		After: place(), Content: &domain.ChangeVersion{Base: new(domain.MaxRevision - 1), Revision: domain.MaxRevision}}
}

// textOf is v's text, as an answer's text block counts it.
func textOf(t *testing.T, v any) int {
	t.Helper()
	text, err := line(v)
	if err != nil {
		t.Fatal(err)
	}
	return mcpserver.TextBytes(text)
}

// A list of versions is REST's PageVersion for each, newest first, times
// in UTC, with the page's path, node id and version now; next_cursor the
// list's, null without one (M9/P2 design 3.16.1). A path that names
// nothing is page.not_found in the module's words: the path, and the hint
// P1's lookup gives (S2 review A-M1).
func TestTheVersionsListAnswersItsShape(t *testing.T) {
	node, v1, v3, c1, c3, alice := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	at := time.Date(2026, 10, 17, 10, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	l := app.PathVersionList{Path: "Guide.md", Node: domain.Node{ID: node}, Current: domain.VersionToken{Node: node, Revision: 4},
		Versions: []domain.Version{
			{ID: v3, Base: new(1), Revision: 4, ByteSize: 12, CreatedAt: at, UpdatedAt: at.Add(time.Minute), Editing: true,
				Changeset: domain.VersionChangeset{ID: c3, Kind: domain.ChangesetEdit, Client: "mcp:claude-code", By: alice, ByName: "Alice",
					CreatedAt: at}},
			{ID: v1, Revision: 1, CreatedAt: at, UpdatedAt: at, Changeset: domain.VersionChangeset{ID: c1, Kind: domain.ChangesetImport,
				Client: domain.ClientWeb, By: alice, ByName: "Alice", CreatedAt: at}},
		}, Cursors: []string{"c4", "c1"}, NextCursor: "c1"}
	var passed []any
	tools := tools{uc: UseCases{Versions: versionsFunc(func(id uuid.UUID, path string, limit *int, cursor *string) (app.PathVersionList, error) {
		passed = []any{id, path, limit, cursor}
		return l, nil
	})}}
	notebook, limit, cursor := uuid.NewV7(), 2, "c9"
	res, err := tools.versions(context.Background(), mcpserver.Call{Notebook: notebook}, versionsIn{Path: "guide.md", Limit: &limit,
		Cursor: &cursor})
	if err != nil {
		t.Fatal(err)
	}
	if passed[0] != notebook || passed[1] != "guide.md" || passed[2] != &limit || passed[3] != &cursor {
		t.Errorf("versions passed %v", passed)
	}
	alices := `"created_by":{"user_id":"` + alice.String() + `","display_name":"Alice"}`
	want := `{"path":"Guide.md","node_id":"` + node.String() + `","current_version":"` + node.String() + `:4","versions":[` +
		`{"id":"` + v3.String() + `","revision":4,"base_revision":1,"byte_size":12,"created_at":"2026-10-17T08:30:00Z",` +
		`"updated_at":"2026-10-17T08:31:00Z","editing":true,"changeset":{"id":"` + c3.String() + `","kind":"edit",` +
		`"client":"mcp:claude-code","created_at":"2026-10-17T08:30:00Z",` + alices + `}},` +
		`{"id":"` + v1.String() + `","revision":1,"base_revision":null,"byte_size":0,"created_at":"2026-10-17T08:30:00Z",` +
		`"updated_at":"2026-10-17T08:30:00Z","editing":false,"changeset":{"id":"` + c1.String() + `","kind":"import",` +
		`"client":"web","created_at":"2026-10-17T08:30:00Z",` + alices + `}}],"next_cursor":"c1"}`
	if raw := rawOf(t, res); raw != want {
		t.Errorf("the answer is\n%s\nwant\n%s", raw, want)
	}
	// The last page: its versions, next_cursor null (S2 review C-M2).
	l.NextCursor = ""
	res, err = tools.versions(context.Background(), mcpserver.Call{}, versionsIn{Path: "Guide.md"})
	if raw := rawOf(t, res); err != nil || !strings.HasSuffix(raw, `"editing":false,"changeset":{"id":"`+c1.String()+`","kind":"import",`+
		`"client":"web","created_at":"2026-10-17T08:30:00Z",`+alices+`}}],"next_cursor":null}`) {
		t.Errorf("the last page = %s, %v", raw, err)
	}
	l.Versions, l.Cursors, l.NextCursor = nil, nil, ""
	res, err = tools.versions(context.Background(), mcpserver.Call{}, versionsIn{Path: "Guide.md"})
	if raw := rawOf(t, res); err != nil || !strings.HasSuffix(raw, `"versions":[],"next_cursor":null}`) {
		t.Errorf("no versions = %s, %v", raw, err)
	}
	if passed[2] != (*int)(nil) || passed[3] != (*string)(nil) {
		t.Errorf("versions without a limit or a cursor passed %v", passed)
	}
	// A path that names nothing is page.not_found at it, with its hint.
	tools.uc.Versions = versionsFunc(func(uuid.UUID, string, *int, *string) (app.PathVersionList, error) {
		return app.PathVersionList{}, &domain.PathNotFound{Path: "Guide.md", Hint: domain.AttachmentHint("Guide")}
	})
	_, err = tools.versions(context.Background(), mcpserver.Call{}, versionsIn{Path: "Guide.md"})
	if text, _ := wordsOf(err); text != `page.not_found: Nothing is at "Guide.md". That is an attachment: its path is "Guide".` {
		t.Errorf("a path that names nothing = %v, %q", err, text)
	}
}

// A list holds as many versions as the answer does, next_cursor the one
// after the last of them; the longest path with a version of the worst
// values fits, about 19,000 bytes (M9/P2 design 3.16.4).
func TestTheVersionsListFitsItsAnswer(t *testing.T) {
	node := uuid.NewV7()
	versions, cursors := make([]domain.Version, 100), make([]string, 100)
	for i := range versions {
		versions[i] = domain.Version{ID: uuid.NewV7(), Base: new(domain.MaxRevision - 1), Revision: domain.MaxRevision,
			ByteSize: domain.MaxContentBytes, CreatedAt: worst().at, UpdatedAt: worst().at, Editing: true,
			Changeset: domain.VersionChangeset{ID: uuid.NewV7(), Kind: domain.ChangesetVersionRestore, Client: worst().client, By: uuid.NewV7(),
				ByName: worst().name, CreatedAt: worst().at}}
		cursors[i] = "cursor-" + strconv.Itoa(i)
	}
	l := app.PathVersionList{Path: worst().path, Node: domain.Node{ID: node}, Current: domain.VersionToken{Node: node, Revision: domain.MaxRevision},
		Versions: versions, Cursors: cursors, NextCursor: cursors[99]}
	tools := tools{uc: UseCases{Versions: versionsFunc(func(uuid.UUID, string, *int, *string) (app.PathVersionList, error) { return l, nil })}}
	res, err := tools.versions(context.Background(), mcpserver.Call{}, versionsIn{Path: worst().path})
	if err != nil {
		t.Fatal(err)
	}
	var a versionsAnswer
	if err := json.Unmarshal([]byte(rawOf(t, res)), &a); err != nil {
		t.Fatal(err)
	}
	k := len(a.Versions)
	if n := mcpserver.TextBytes(rawOf(t, res)); n > mcpserver.MaxAnswerBytes || k == 0 || k >= 100 {
		t.Fatalf("%d versions in %d bytes, want some within %d", k, n, mcpserver.MaxAnswerBytes)
	}
	if a.NextCursor == nil || *a.NextCursor != cursors[k-1] {
		t.Errorf("next_cursor %v after %d versions, want the cursor after the %dth", a.NextCursor, k, k)
	}
	one := textOf(t, versionsAnswer{Path: worst().path, NodeID: node.String(), CurrentVersion: l.Current.String(),
		Versions: []versionRow{versionOf(versions[0])}, NextCursor: new(strings.Repeat("e", 120))})
	t.Logf("the longest path with one version of the worst values: %d bytes; %d versions in an answer", one, k)
	if one > mcpserver.MaxAnswerBytes/2 {
		t.Errorf("the longest path with one version is %d bytes, want about 19,000", one)
	}
}

// With revision, versions reads the version as read reads a page: its
// first block, a line of the version's revision and the page's version
// now beside read's members, then its lines; the slice's room leaves the
// widest such block (M9/P2 design 3.16.1). A path that names nothing is
// page.not_found at the path, with its hint, as for the list.
func TestTheVersionsReadAVersionAsReadReadsAPage(t *testing.T) {
	node, notebook := uuid.NewV7(), uuid.NewV7()
	var passed []any
	var room int
	v := app.PathVersion{Path: "A.md", Node: domain.Node{ID: node}, Current: domain.VersionToken{Node: node, Revision: 7}, Revision: 5,
		Window: domain.Window{Offset: 2, Column: 3, Limit: 2}, Slice: domain.Slice{Text: "ab\ncd\n", Lines: 2, TotalLines: 9, NextOffset: 4},
		Bytes: 40}
	tools := tools{uc: UseCases{Version: versionFunc(func(id uuid.UUID, path string, revision int, o, c, l *int, m app.TextMeasure) (
		app.PathVersion, error,
	) {
		passed, room = []any{id, path, revision, o, c, l}, m.Room("A.md")
		return v, nil
	})}}
	revision, offset, column, limit := 4, 2, 3, 2
	res, err := tools.versions(context.Background(), mcpserver.Call{Notebook: notebook}, versionsIn{Path: "a.md", Revision: &revision,
		Offset: &offset, Column: &column, Limit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	if passed[0] != notebook || passed[1] != "a.md" || passed[2] != 4 || passed[3] != &offset || passed[4] != &column || passed[5] != &limit {
		t.Errorf("versions passed %v", passed)
	}
	want := `{"path":"A.md","kind":"page","revision":5,"current_version":"` + node.String() + `:7","offset":2,"column":3,"lines":2,` +
		`"total_lines":9,"byte_size":40,"next_offset":4,"next_column":null}`
	if len(res.Blocks) != 2 || res.Blocks[0].Text != want || res.Blocks[1].Text != "ab\ncd\n" {
		t.Errorf("the answer is %+v, want\n%s\nand the lines", res.Blocks, want)
	}
	widestText, err := line(widestVersion("A.md"))
	if err != nil {
		t.Fatal(err)
	}
	if room != mcpserver.MaxAnswerBytes-mcpserver.TextBytes(widestText) || room >= measure().Room("A.md") {
		t.Errorf("the room is %d, want the bound less the widest first block, below read's %d", room, measure().Room("A.md"))
	}
	v.Slice = domain.Slice{Text: "x", Lines: 1, TotalLines: 1, NextColumn: 9, NextOffset: 1}
	res, err = tools.versions(context.Background(), mcpserver.Call{}, versionsIn{Path: "A.md", Revision: &revision})
	if err != nil || !strings.HasSuffix(res.Blocks[0].Text, `"next_offset":1,"next_column":9}`) {
		t.Errorf("a line cut = %+v, %v", res.Blocks, err)
	}
	// A path that names nothing is page.not_found at it, with its hint.
	tools.uc.Version = versionFunc(func(uuid.UUID, string, int, *int, *int, *int, app.TextMeasure) (app.PathVersion, error) {
		return app.PathVersion{}, &domain.PathNotFound{Path: "Guide.md", Hint: domain.TitleHint("Guide.md.md")}
	})
	_, err = tools.versions(context.Background(), mcpserver.Call{}, versionsIn{Path: "Guide.md", Revision: &revision})
	if text, _ := wordsOf(err); text != `page.not_found: Nothing is at "Guide.md". A page there is titled with the .md: its path is "Guide.md.md".` {
		t.Errorf("a path that names nothing = %v, %q", err, text)
	}
}

// The arguments of versions' other call are refused before anything else,
// the cursor's 400 too: neither use case is called (M9/P2 design 3.16.1).
func TestTheVersionsRefuseTheOtherCallsArgumentsFirst(t *testing.T) {
	called := 0
	tools := tools{uc: UseCases{
		Versions: versionsFunc(func(uuid.UUID, string, *int, *string) (app.PathVersionList, error) {
			called++
			return app.PathVersionList{}, shared.InvalidCursor()
		}),
		Version: versionFunc(func(uuid.UUID, string, int, *int, *int, *int, app.TextMeasure) (app.PathVersion, error) {
			called++
			return app.PathVersion{}, shared.InvalidCursor()
		}),
	}}
	one, bad := 1, "x"
	for _, tt := range []struct {
		name   string
		in     versionsIn
		fields []string
	}{
		{"a read's cursor", versionsIn{Path: "A.md", Revision: &one, Cursor: &bad}, []string{"cursor"}},
		{"the list's offset and a bad cursor", versionsIn{Path: "A.md", Offset: &one, Cursor: &bad}, []string{"offset"}},
		{"the list's column", versionsIn{Path: "A.md", Column: &one}, []string{"column"}},
		{"the list's offset and column", versionsIn{Path: "A.md", Offset: &one, Column: &one}, []string{"offset", "column"}},
	} {
		called = 0
		_, err := tools.versions(context.Background(), mcpserver.Call{}, tt.in)
		var e *shared.Error
		if !errors.As(err, &e) || e.Code != shared.CodeValidationFailed || len(e.Fields) != len(tt.fields) || called != 0 {
			t.Errorf("%s: %v after %d calls, want 422 on %v before any", tt.name, err, called, tt.fields)
			continue
		}
		for i, f := range e.Fields {
			if f.Field != tt.fields[i] || f.Code != shared.FieldNotAllowed {
				t.Errorf("%s: field %+v, want %s not_allowed", tt.name, f, tt.fields[i])
			}
		}
	}
	// Each call's own arguments reach its use case, a bad cursor its 400.
	for name, in := range map[string]versionsIn{"the list": {Path: "A.md", Cursor: &bad}, "a read": {Path: "A.md", Revision: &one, Offset: &one}} {
		called = 0
		var e *shared.Error
		if _, err := tools.versions(context.Background(), mcpserver.Call{}, in); !errors.As(err, &e) || e.Code != shared.CodeBadRequest ||
			called != 1 {
			t.Errorf("%s: %v after %d calls, want its use case's answer", name, err, called)
		}
	}
}

// A changeset as log lists it is REST's Changeset, times in UTC, a node's
// kind "attachment" where REST's is "asset", reverted_by never null, its
// first changes without paths and no changes_cut; next_cursor the page's,
// null without one; the arguments pass as they are (M9/P2 design 3.16.2).
func TestTheLogAnswersItsShape(t *testing.T) {
	cs, other, notebook, alice, bob, guide, png, plan, reverted := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(),
		uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	at := time.Date(2026, 10, 17, 10, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	l := app.PathLog{Changesets: []app.LoggedChangeset{
		{Changeset: domain.Changeset{ID: cs, NotebookID: notebook, Kind: domain.ChangesetRevert, Client: domain.ClientAPI, Message: new("why"),
			By: bob, ByName: "Bob", CreatedAt: at, UpdatedAt: at.Add(time.Second), Reverts: &domain.ChangesetBrief{ID: other,
				Kind: domain.ChangesetEdit, By: alice, ByName: "Alice", CreatedAt: at}},
			Facts: domain.ChangesetFacts{Nodes: 1, Contents: 1, RevertedBy: []uuid.UUID{reverted}},
			Changes: []domain.ChangeRow{
				{NodeID: png, NodeKind: domain.KindAsset, Name: "flow-1.png", Alive: true,
					Before: &domain.ChangePlace{ParentID: &guide, ParentName: new("Guide"), Name: "flow.png"},
					After:  &domain.ChangePlace{ParentID: &guide, ParentName: new("Guide"), Name: "flow-1.png"}},
				{NodeID: plan, NodeKind: domain.KindPage, Name: "Plan", Content: &domain.ChangeVersion{Base: new(2), Revision: 3}},
			}},
		{Changeset: domain.Changeset{ID: other, NotebookID: notebook, Kind: domain.ChangesetEdit, Client: "mcp:claude-code", By: alice,
			ByName: "Alice", CreatedAt: at, UpdatedAt: at, Trimmed: true},
			Changes: []domain.ChangeRow{{NodeID: plan, NodeKind: domain.KindPage, Name: "Plan",
				After: &domain.ChangePlace{Name: "Plan"}, Content: &domain.ChangeVersion{Revision: 1}}}},
	}, Cursors: []string{"c1", "c2"}, NextCursor: "c2"}
	var passed []any
	tools := tools{uc: UseCases{Log: logFunc(func(id uuid.UUID, q app.PathLogQuery) (app.PathLog, error) {
		passed = []any{id, q}
		return l, nil
	})}}
	path, actor, client, limit, cursor := "Guide.md", "me", "mcp", 2, "c0"
	in := logIn{Path: &path, Actor: &actor, Client: &client, Limit: &limit, Cursor: &cursor}
	res, err := tools.log(context.Background(), mcpserver.Call{Notebook: notebook}, in)
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := passed[1].(app.PathLogQuery); passed[0] != notebook || !ok || q.Path != &path || q.Actor != &actor || q.Client != &client ||
		q.Limit != &limit || q.Cursor != &cursor {
		t.Errorf("log passed %v", passed)
	}
	user := func(id uuid.UUID, name string) string {
		return `{"user_id":"` + id.String() + `","display_name":"` + name + `"}`
	}
	guides := `"parent_id":"` + guide.String() + `","parent_name":"Guide"`
	want := `{"changesets":[{"id":"` + cs.String() + `","notebook_id":"` + notebook.String() + `","kind":"revert","client":"api",` +
		`"message":"why","created_by":` + user(bob, "Bob") + `,"created_at":"2026-10-17T08:30:00Z","updated_at":"2026-10-17T08:30:01Z",` +
		`"reverts":{"id":"` + other.String() + `","kind":"edit","created_by":` + user(alice, "Alice") + `,"created_at":"2026-10-17T08:30:00Z"},` +
		`"reverted_by":["` + reverted.String() + `"],"trimmed":false,"node_count":1,"content_count":1,"changes":[` +
		`{"node":{"id":"` + png.String() + `","kind":"attachment","name":"flow-1.png","alive":true},"kind":"renamed",` +
		`"before":{` + guides + `,"name":"flow.png"},"after":{` + guides + `,"name":"flow-1.png"},"content":null},` +
		`{"node":{"id":"` + plan.String() + `","kind":"page","name":"Plan","alive":false},"kind":"content","before":null,"after":null,` +
		`"content":{"base_revision":2,"revision":3}}]},` +
		`{"id":"` + other.String() + `","notebook_id":"` + notebook.String() + `","kind":"edit","client":"mcp:claude-code","message":null,` +
		`"created_by":` + user(alice, "Alice") + `,"created_at":"2026-10-17T08:30:00Z","updated_at":"2026-10-17T08:30:00Z","reverts":null,` +
		`"reverted_by":[],"trimmed":true,"node_count":0,"content_count":0,"changes":[{"node":{"id":"` + plan.String() + `","kind":"page",` +
		`"name":"Plan","alive":false},"kind":"created","before":null,"after":{"parent_id":null,"parent_name":null,"name":"Plan"},` +
		`"content":{"base_revision":null,"revision":1}}]}],"next_cursor":"c2"}`
	if raw := rawOf(t, res); raw != want {
		t.Errorf("the answer is\n%s\nwant\n%s", raw, want)
	}
	l = app.PathLog{}
	res, err = tools.log(context.Background(), mcpserver.Call{}, logIn{})
	if raw := rawOf(t, res); err != nil || raw != `{"changesets":[],"next_cursor":null}` {
		t.Errorf("an empty log = %s, %v", raw, err)
	}
	if q := passed[1].(app.PathLogQuery); q != (app.PathLogQuery{}) {
		t.Errorf("log without arguments passed %+v", q)
	}
	// A path that names nothing is page.not_found at it, with its hint.
	tools.uc.Log = logFunc(func(uuid.UUID, app.PathLogQuery) (app.PathLog, error) {
		return app.PathLog{}, &domain.PathNotFound{Path: path, Hint: "a hint"}
	})
	_, err = tools.log(context.Background(), mcpserver.Call{}, in)
	if text, _ := wordsOf(err); text != `page.not_found: Nothing is at "Guide.md". a hint` {
		t.Errorf("a path that names nothing = %v, %q", err, text)
	}
}

// A log answers as many rows as fit, next_cursor the cursor after the last
// of them, none with changes_cut. A first row that does not fit whole
// comes alone, with as many of its changes as fit and changes_cut true,
// next_cursor the cursor after it, not the page's: the second row is not
// skipped (M9/P2 design 3.16.2, 6.3 #43, #44). Each answer is within the
// bound.
func TestTheLogFitsItsAnswer(t *testing.T) {
	rows, cursors := make([]app.LoggedChangeset, 100), make([]string, 100)
	for i := range rows {
		rows[i] = worstChangeset()
		rows[i].Changes = nil
		cursors[i] = "cursor-" + strconv.Itoa(i)
	}
	page := app.PathLog{Changesets: rows, Cursors: cursors, NextCursor: "next"}
	tools := tools{uc: UseCases{Log: logFunc(func(uuid.UUID, app.PathLogQuery) (app.PathLog, error) { return page, nil })}}
	answer := func() (logAnswer, string) {
		t.Helper()
		res, err := tools.log(context.Background(), mcpserver.Call{}, logIn{})
		if err != nil {
			t.Fatal(err)
		}
		raw := rawOf(t, res)
		if n := mcpserver.TextBytes(raw); n > mcpserver.MaxAnswerBytes {
			t.Errorf("the answer is %d bytes, over %d", n, mcpserver.MaxAnswerBytes)
		}
		var a logAnswer
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			t.Fatal(err)
		}
		return a, raw
	}
	a, raw := answer()
	if k := len(a.Changesets); k == 0 || k >= 100 || a.NextCursor == nil || *a.NextCursor != cursors[k-1] ||
		strings.Contains(raw, "changes_cut") {
		t.Errorf("%d rows of 100, next_cursor %v; want some, the cursor after the last, no changes_cut", k, a.NextCursor)
	}
	// The worst row first, then another: the worst alone, cut.
	largest := worstChangeset()
	page = app.PathLog{Changesets: []app.LoggedChangeset{largest, rows[0]}, Cursors: []string{"c1", "c2"}, NextCursor: "c2"}
	a, _ = answer()
	switch {
	case len(a.Changesets) != 1 || a.Changesets[0].ID != largest.Changeset.ID:
		t.Errorf("the rows are %+v, want the worst alone", a.Changesets)
	case !a.Changesets[0].ChangesCut || len(a.Changesets[0].Changes) != 1 || a.Changesets[0].Changes[0].Node.ID != largest.Changes[0].NodeID:
		t.Errorf("the worst row has %d changes, changes_cut %v; want its first and true", len(a.Changesets[0].Changes),
			a.Changesets[0].ChangesCut)
	case a.NextCursor == nil || *a.NextCursor != "c1":
		t.Errorf("next_cursor %v, want the cursor after the worst row, c1", a.NextCursor)
	}
	// The worst row alone on its page: next_cursor the page's.
	for _, next := range []string{"", "c1"} {
		page = app.PathLog{Changesets: []app.LoggedChangeset{largest}, Cursors: []string{"c1"}, NextCursor: next}
		if a, _ = answer(); len(a.Changesets) != 1 || !a.Changesets[0].ChangesCut || (next == "") != (a.NextCursor == nil) ||
			(a.NextCursor != nil && *a.NextCursor != next) {
			t.Errorf("the worst row alone, the page's next %q: next_cursor %v", next, a.NextCursor)
		}
	}
	// What it measures (M9/P2 design 3.16.4): a row without changes fits,
	// with three it is over.
	cursor := new(strings.Repeat("e", 120))
	bare, whole := changesetRowOf(largest), changesetRowOf(largest)
	bare.Changes = []change[changeNode]{}
	none, three := textOf(t, logAnswer{Changesets: []changesetRow{bare}, NextCursor: cursor}),
		textOf(t, logAnswer{Changesets: []changesetRow{whole}, NextCursor: cursor})
	t.Logf("the worst row: %d bytes without changes, %d with three", none, three)
	if none > mcpserver.MaxAnswerBytes || three <= mcpserver.MaxAnswerBytes {
		t.Errorf("the worst row is %d bytes without changes and %d with three, want within and over %d", none, three,
			mcpserver.MaxAnswerBytes)
	}
}

// changeset answers the changeset on its first page, REST's getChangeset
// without its changes and with renames and moves, null with a cursor; its
// changes are REST's, each node's kind "attachment" for REST's "asset",
// with its path now, null in the trash or gone (M9/P2 design 3.16.3).
func TestTheChangesetAnswersItsShape(t *testing.T) {
	cs, notebook, alice, guide, png, plan := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	at := time.Date(2026, 10, 17, 10, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	logged := app.LoggedChangeset{Changeset: domain.Changeset{ID: cs, NotebookID: notebook, Kind: domain.ChangesetEdit,
		Client: "mcp:claude-code", By: alice, ByName: "Alice", CreatedAt: at, UpdatedAt: at},
		Facts: domain.ChangesetFacts{Nodes: 2, Contents: 0}, Changes: []domain.ChangeRow{{NodeID: png}}}
	changes := []domain.ChangeRow{
		{NodeID: png, NodeKind: domain.KindAsset, Name: "flow-1.png", Alive: true,
			Before: &domain.ChangePlace{ParentID: &guide, ParentName: new("Guide"), Name: "flow.png"},
			After:  &domain.ChangePlace{ParentID: &guide, ParentName: new("Guide"), Name: "flow-1.png"}},
		{NodeID: plan, NodeKind: domain.KindPage, Name: "Plan", Before: &domain.ChangePlace{Name: "Plan"}},
	}
	p := app.ChangesetPage{Detail: &app.ChangesetDetail{LoggedChangeset: logged, Moves: domain.ChangesetMoves{Renames: true}},
		Changes: changes, Cursors: []string{"c1", "c2"}, Paths: map[uuid.UUID]string{png: "Guide/flow-1.png"}, NextCursor: "c2",
		Start: "start"}
	var passed []any
	tools := tools{uc: UseCases{Changeset: changesetFunc(func(nb, id uuid.UUID, limit *int, cursor *string) (app.ChangesetPage, error) {
		passed = []any{nb, id, limit, cursor}
		return p, nil
	})}}
	limit := 2
	res, err := tools.changeset(context.Background(), mcpserver.Call{Notebook: notebook}, changesetIn{ID: cs.String(), Limit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	if passed[0] != notebook || passed[1] != cs || passed[2] != &limit || passed[3] != (*string)(nil) {
		t.Errorf("changeset passed %v", passed)
	}
	guides := `"parent_id":"` + guide.String() + `","parent_name":"Guide"`
	changesJSON := `"changes":[{"node":{"id":"` + png.String() + `","kind":"attachment","name":"flow-1.png","alive":true,` +
		`"path":"Guide/flow-1.png"},"kind":"renamed","before":{` + guides + `,"name":"flow.png"},"after":{` + guides +
		`,"name":"flow-1.png"},"content":null},{"node":{"id":"` + plan.String() + `","kind":"page","name":"Plan","alive":false,` +
		`"path":null},"kind":"deleted","before":{"parent_id":null,"parent_name":null,"name":"Plan"},"after":null,"content":null}],` +
		`"next_cursor":"c2"}`
	want := `{"changeset":{"id":"` + cs.String() + `","notebook_id":"` + notebook.String() + `","kind":"edit","client":"mcp:claude-code",` +
		`"message":null,"created_by":{"user_id":"` + alice.String() + `","display_name":"Alice"},"created_at":"2026-10-17T08:30:00Z",` +
		`"updated_at":"2026-10-17T08:30:00Z","reverts":null,"reverted_by":[],"trimmed":false,"node_count":2,"content_count":0,` +
		`"renames":true,"moves":false},` + changesJSON
	if raw := rawOf(t, res); raw != want {
		t.Errorf("the answer is\n%s\nwant\n%s", raw, want)
	}
	// A changeset that moves a node across parents (S2 review C-M2).
	p.Detail.Moves = domain.ChangesetMoves{Moves: true}
	res, err = tools.changeset(context.Background(), mcpserver.Call{}, changesetIn{ID: cs.String()})
	if raw := rawOf(t, res); err != nil || !strings.Contains(raw, `"content_count":0,"renames":false,"moves":true},`) {
		t.Errorf("a changeset that moves = %s, %v", raw, err)
	}
	// With a cursor, no changeset.
	p.Detail = nil
	cursor := "c0"
	res, err = tools.changeset(context.Background(), mcpserver.Call{}, changesetIn{ID: cs.String(), Cursor: &cursor})
	if raw := rawOf(t, res); err != nil || raw != `{"changeset":null,`+changesJSON || passed[3] != &cursor {
		t.Errorf("a page with a cursor = %s, %v", raw, err)
	}
	p = app.ChangesetPage{Start: "start"}
	res, err = tools.changeset(context.Background(), mcpserver.Call{}, changesetIn{ID: cs.String(), Cursor: &cursor})
	if raw := rawOf(t, res); err != nil || raw != `{"changeset":null,"changes":[],"next_cursor":null}` {
		t.Errorf("a page past the last change = %s, %v", raw, err)
	}
}

// changeset's id is read as uuid.Parse reads one; anything else is the
// zero id, which the use case finds no changeset of; its errors are as
// they are (M9/P2 design 3.16.3). Each id reaches the use case, whose
// checks of the cursor, the limit and the notebook come first: what the
// use case was given is forgotten before each call (S2 review C-M3).
func TestTheChangesetsIDIsAUUIDOrNone(t *testing.T) {
	id := uuid.MustParse("0199f3a2-7c4d-7e21-9a3b-5f6e7d8c9b0a")
	var got uuid.UUID
	tools := tools{uc: UseCases{Changeset: changesetFunc(func(_, id uuid.UUID, _ *int, _ *string) (app.ChangesetPage, error) {
		got = id
		return app.ChangesetPage{}, domain.ErrChangesetNotFound
	})}}
	for in, want := range map[string]uuid.UUID{
		id.String(): id, strings.ToUpper(id.String()): id, "{" + id.String() + "}": id, "not-a-uuid": {}, "": {}, id.String() + " ": {},
	} {
		got = uuid.Max() // no id the use case is given: still Max, it was not called
		switch _, err := tools.changeset(context.Background(), mcpserver.Call{}, changesetIn{ID: in}); {
		case got == uuid.Max():
			t.Errorf("changeset(%q) answered %v without its use case, want the use case given %s", in, err, want)
		case got != want || !errors.Is(err, domain.ErrChangesetNotFound):
			t.Errorf("changeset(%q) passed %s and answered %v, want %s and its use case's error", in, got, err, want)
		}
	}
}

// changeset answers as many changes as fit, next_cursor the cursor after
// the last of them. A first page whose changeset leaves its first change
// no room answers the changeset alone, next_cursor the first change's
// page, Start; a page with a cursor holds a change of the worst values.
// Each answer is within the bound (M9/P2 design 3.16.3, 3.16.4).
func TestTheChangesetFitsItsAnswer(t *testing.T) {
	largest := worstChangeset()
	changes, cursors, paths := make([]domain.ChangeRow, 100), make([]string, 100), map[uuid.UUID]string{}
	for i := range changes {
		changes[i] = worstChange()
		cursors[i] = "cursor-" + strconv.Itoa(i)
		paths[changes[i].NodeID] = worst().path
	}
	p := app.ChangesetPage{Changes: changes, Cursors: cursors, Paths: paths, NextCursor: "next", Start: "start"}
	tools := tools{uc: UseCases{Changeset: changesetFunc(func(uuid.UUID, uuid.UUID, *int, *string) (app.ChangesetPage, error) {
		return p, nil
	})}}
	answer := func() changesetAnswer {
		t.Helper()
		res, err := tools.changeset(context.Background(), mcpserver.Call{}, changesetIn{ID: largest.Changeset.ID.String()})
		if err != nil {
			t.Fatal(err)
		}
		raw := rawOf(t, res)
		if n := mcpserver.TextBytes(raw); n > mcpserver.MaxAnswerBytes {
			t.Errorf("the answer is %d bytes, over %d", n, mcpserver.MaxAnswerBytes)
		}
		var a changesetAnswer
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	// A page with a cursor: some of the changes, each of the worst values.
	a := answer()
	if k := len(a.Changes); a.Changeset != nil || k == 0 || k >= 100 || a.NextCursor == nil || *a.NextCursor != cursors[k-1] {
		t.Errorf("%d changes of 100, next_cursor %v; want some, the cursor after the last", k, a.NextCursor)
	}
	// The first page: the worst changeset leaves no room.
	p.Detail = &app.ChangesetDetail{LoggedChangeset: largest, Moves: domain.ChangesetMoves{Renames: true, Moves: true}}
	if a = answer(); a.Changeset == nil || a.Changeset.ID != largest.Changeset.ID || len(a.Changes) != 0 || a.NextCursor == nil ||
		*a.NextCursor != "start" {
		t.Errorf("the worst first page = %+v, %d changes, next_cursor %v; want the changeset alone and start", a.Changeset,
			len(a.Changes), a.NextCursor)
	}
	// A first page whose changeset leaves room: its changes go on.
	p.Detail.Changeset.Message = nil
	if a = answer(); a.Changeset == nil || len(a.Changes) == 0 || *a.NextCursor != cursors[len(a.Changes)-1] {
		t.Errorf("a first page with room = %d changes, next_cursor %v", len(a.Changes), a.NextCursor)
	}
	// A change of a page with a cursor that does not fit is the tool's
	// fault, not a page of no change that would start over.
	p.Detail = nil
	p.Changes = []domain.ChangeRow{{NodeID: uuid.NewV7(), NodeKind: domain.KindPage, Name: strings.Repeat("&", mcpserver.MaxAnswerBytes)}}
	if _, err := tools.changeset(context.Background(), mcpserver.Call{}, changesetIn{}); !errors.Is(err, errNoChangeFits) {
		t.Errorf("a change that does not fit = %v, want errNoChangeFits", err)
	}
	// What it measures (M9/P2 design 3.16.4).
	cursor := new(strings.Repeat("e", 120))
	header := &changesetHeader{changesetFields: fieldsOf(largest), Renames: true, Moves: true}
	one := []change[pathNode]{changeOf(worstChange(), pathNode{changeNode: nodeOf(worstChange()), Path: new(worst().path)})}
	alone := textOf(t, changesetAnswer{Changeset: header, Changes: []change[pathNode]{}, NextCursor: cursor})
	first := textOf(t, changesetAnswer{Changeset: header, Changes: one, NextCursor: cursor})
	page := textOf(t, changesetAnswer{Changes: one, NextCursor: cursor})
	t.Logf("the worst changeset: %d bytes alone, %d with a change; a page of a change: %d", alone, first, page)
	if alone > mcpserver.MaxAnswerBytes || first <= mcpserver.MaxAnswerBytes || page > mcpserver.MaxAnswerBytes {
		t.Errorf("the worst changeset is %d bytes alone, %d with a change, a page of a change %d: want within, over, within %d",
			alone, first, page, mcpserver.MaxAnswerBytes)
	}
}

// The widest first block of a version is wider than read's by the
// version's revision and the page's version now, and no real one is wider.
func TestTheWidestVersionIsTheWidest(t *testing.T) {
	widestText, err := line(widestVersion("A.md"))
	if err != nil {
		t.Fatal(err)
	}
	most := math.MaxInt32
	actual := versionMeta{Path: "A.md", Kind: "page", Revision: most, CurrentVersion: domain.VersionToken{Node: uuid.Max(), Revision: most}.String(),
		Offset: math.MaxInt64, Column: math.MaxInt64, Lines: math.MaxInt64, TotalLines: math.MaxInt64, Bytes: math.MaxInt64,
		NextOffset: new(math.MaxInt64), NextColumn: new(math.MaxInt64)}
	actualText, err := line(actual)
	if err != nil {
		t.Fatal(err)
	}
	if mcpserver.TextBytes(actualText) > mcpserver.TextBytes(widestText) {
		t.Errorf("a version's block of %d bytes is wider than the widest, %d", mcpserver.TextBytes(actualText),
			mcpserver.TextBytes(widestText))
	}
}
