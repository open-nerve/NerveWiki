package app_test

import (
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// logFixture is eng's log: alice created A and B (made), bob renamed A
// (renamed) and wrote B's content from the API (written), alice reverted
// the rename from an MCP client (reverted); a changeset of another
// notebook and one of a deleted notebook (gone).
type logFixture struct {
	*fixture
	bob                                 uuid.UUID
	a, b                                domain.Node
	made, renamed, written, reverted    *fakeChangeset
	elsewhere, gone                     *fakeChangeset
	madeAt, renamedAt, writtenAt, revAt time.Time
}

func newLogFixture() *logFixture {
	f := &logFixture{fixture: newFixture(), bob: uuid.NewV7()}
	f.grant(domain.ActionHistory)
	f.names[f.alice], f.names[f.bob] = "Alice", "Bob"
	f.a, f.b = f.page("A", nil, 0), f.page("B", nil, 1)
	aState, bState := f.a.State(), f.b.State()
	renamed := domain.TreeState{Name: "A2", SortOrder: 0}
	f.madeAt, f.renamedAt, f.writtenAt, f.revAt = now().Add(-4*time.Hour), now().Add(-3*time.Hour), now().Add(-2*time.Hour),
		now().Add(-time.Hour)
	f.made = f.log.changeset(domain.ChangesetEdit, f.alice, f.madeAt,
		[]domain.ChangesetItem{{NodeID: f.a.ID, After: &aState, Revision: 1}, {NodeID: f.b.ID, After: &bState, Revision: 1}},
		f.log.version(f.a.ID, 0, 1, ""), f.log.version(f.b.ID, 0, 1, ""))
	f.renamed = f.log.changeset(domain.ChangesetEdit, f.bob, f.renamedAt,
		[]domain.ChangesetItem{{NodeID: f.a.ID, Before: &aState, After: &renamed}})
	f.written = f.log.changeset(domain.ChangesetEdit, f.bob, f.writtenAt, nil, f.log.version(f.b.ID, 1, 2, "two"))
	f.written.Client = domain.ClientAPI
	f.reverted = f.log.changeset(domain.ChangesetRevert, f.alice, f.revAt,
		[]domain.ChangesetItem{{NodeID: f.a.ID, Before: &renamed, After: &aState}})
	f.reverted.RevertsID, f.reverted.Client = f.renamed.ID, "mcp:claude"
	f.elsewhere = f.log.changeset(domain.ChangesetEdit, f.alice, now(), nil)
	f.elsewhere.NotebookID = uuid.NewV7()
	f.gone = f.log.changeset(domain.ChangesetEdit, f.alice, now(), nil)
	f.gone.gone = true
	return f
}

func ids(changesets []app.LoggedChangeset) []uuid.UUID {
	out := make([]uuid.UUID, len(changesets))
	for i, c := range changesets {
		out[i] = c.Changeset.ID
	}
	return out
}

// The log lists a notebook's changesets newer first, a page at a time
// from the newest, the cursor of the next after each page but the last;
// each with its counts and reverts, its first changes and its actors'
// names, read once a page; a revert with the brief of what it reverts.
func TestListChangesetsNewerFirst(t *testing.T) {
	f := newLogFixture()
	uc := app.NewListChangesets(f.historyReads())
	limit := 3
	got, err := uc.Execute(f.asAlice(), f.eng, app.LogQuery{Limit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	if want := []uuid.UUID{f.reverted.ID, f.written.ID, f.renamed.ID}; !slices.Equal(ids(got.Changesets), want) || got.NextCursor == "" {
		t.Fatalf("first page = %v, next %q; want %v and a cursor", ids(got.Changesets), got.NextCursor, want)
	}
	if want := []string{"WorkspaceOf", "Authorize page.history", "ListChangesets", "ChangesetFacts 3", "FirstChanges 3 3"}; !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v, want %v", f.rec.calls, want)
	}
	rev := got.Changesets[0]
	if rev.Changeset.ByName != "Alice" || rev.Changeset.Reverts == nil || rev.Changeset.Reverts.ID != f.renamed.ID ||
		rev.Changeset.Reverts.ByName != "Bob" || !rev.Changeset.Reverts.CreatedAt.Equal(f.renamedAt) {
		t.Errorf("the revert = %+v, reverting %+v; want alice's, of bob's rename", rev.Changeset, rev.Changeset.Reverts)
	}
	if renamed := got.Changesets[2]; renamed.Changeset.ByName != "Bob" || !slices.Equal(renamed.Facts.RevertedBy, []uuid.UUID{f.reverted.ID}) ||
		renamed.Facts.Nodes != 1 || renamed.Facts.Contents != 0 || len(renamed.Changes) != 1 || renamed.Changes[0].Change() != domain.ChangeRenamed {
		t.Errorf("the rename = %+v, %+v, %+v; want bob's, reverted, one node renamed", renamed.Changeset, renamed.Facts, renamed.Changes)
	}
	if written := got.Changesets[1]; written.Facts.Contents != 1 || len(written.Changes) != 1 ||
		written.Changes[0].Change() != domain.ChangeContent || written.Changeset.Client != domain.ClientAPI {
		t.Errorf("the write = %+v, %+v; want B's content", written.Facts, written.Changes)
	}
	f.rec.calls = nil
	next, err := uc.Execute(f.asAlice(), f.eng, app.LogQuery{Limit: &limit, Cursor: &got.NextCursor})
	if err != nil || !slices.Equal(ids(next.Changesets), []uuid.UUID{f.made.ID}) || next.NextCursor != "" {
		t.Errorf("second page = %v, %q, %v; want made's alone, the last", ids(next.Changesets), next.NextCursor, err)
	}
	if made := next.Changesets[0]; made.Facts.Nodes != 2 || made.Facts.Contents != 2 || len(made.Changes) != 2 ||
		made.Changes[0].Change() != domain.ChangeCreated {
		t.Errorf("made = %+v, %+v; want two pages created", made.Facts, made.Changes)
	}
	if f.called("ChangesetMoves") {
		t.Error("the log read whether its changesets rename or move a node")
	}
}

// The log's filters reach its statement: one actor's, one client's, every
// MCP client's, the changesets of a node, alive or in the trash.
func TestListChangesetsFilters(t *testing.T) {
	f := newLogFixture()
	uc := app.NewListChangesets(f.historyReads())
	web, mcp, api := "web", "mcp", "api"
	f.trashed(f.b.ID, now(), f.alice)
	for _, tt := range []struct {
		name string
		q    app.LogQuery
		call string
		want []uuid.UUID
	}{
		{"bob's", app.LogQuery{Actor: &f.bob}, "ListChangesets actor", []uuid.UUID{f.written.ID, f.renamed.ID}},
		{"the web's", app.LogQuery{Client: &web}, "ListChangesets client web", []uuid.UUID{f.renamed.ID, f.made.ID}},
		{"the MCP clients'", app.LogQuery{Client: &mcp}, "ListChangesets mcp", []uuid.UUID{f.reverted.ID}},
		{"B's, in the trash", app.LogQuery{PageID: &f.b.ID}, "ListChangesets page", []uuid.UUID{f.written.ID, f.made.ID}},
		{"bob's of B from the API", app.LogQuery{Actor: &f.bob, PageID: &f.b.ID, Client: &api}, "ListChangesets actor client api page",
			[]uuid.UUID{f.written.ID}},
	} {
		f.rec.calls = nil
		got, err := uc.Execute(f.asAlice(), f.eng, tt.q)
		if err != nil || !slices.Equal(ids(got.Changesets), tt.want) || !f.called(tt.call) {
			t.Errorf("%s = %v, %v after %v; want %v through %q", tt.name, ids(got.Changesets), err, f.rec.calls, tt.want, tt.call)
		}
	}
}

// The log's codes come in listAssets' order: the cursor first, then the
// notebook and the decision, then the node, then the client and the limit.
func TestListChangesetsCodes(t *testing.T) {
	f := newLogFixture()
	uc := app.NewListChangesets(f.historyReads())
	bad, badClient, zero := "!", "mobile", 0
	other := f.page("Other", nil, 2)
	other.NotebookID = uuid.NewV7()
	f.store.nodes[other.ID] = other
	missing := uuid.NewV7()
	for _, tt := range []struct {
		name     string
		notebook uuid.UUID
		q        app.LogQuery
		want     string
	}{
		{"a cursor that does not decode, of no notebook", uuid.NewV7(), app.LogQuery{Cursor: &bad, PageID: &missing}, shared.CodeBadRequest + " cursor"},
		{"no notebook, a node of none", uuid.NewV7(), app.LogQuery{PageID: &missing, Client: &badClient}, "notebook.not_found"},
		{"a node of another notebook, a bad client", f.eng, app.LogQuery{PageID: &other.ID, Client: &badClient}, "page.not_found"},
		{"a node of none", f.eng, app.LogQuery{PageID: &missing}, "page.not_found"},
		{"a bad client and limit", f.eng, app.LogQuery{Client: &badClient, Limit: &zero}, shared.CodeValidationFailed + " client"},
		{"a bad limit", f.eng, app.LogQuery{Limit: &zero}, shared.CodeValidationFailed + " limit"},
	} {
		_, err := uc.Execute(f.asAlice(), tt.notebook, tt.q)
		var e *shared.Error
		got := ""
		if errors.As(err, &e) {
			got = e.Code
			if len(e.Fields) == 1 {
				got += " " + e.Fields[0].Field
			}
		}
		if got != tt.want {
			t.Errorf("%s = %v (%s), want %s", tt.name, err, got, tt.want)
		}
	}
	g := newLogFixture()
	delete(g.auth.grants, domain.ActionHistory)
	if _, err := app.NewListChangesets(g.historyReads()).Execute(g.asAlice(), g.eng, app.LogQuery{}); !errors.Is(err, domain.ErrNotebookNotFound) {
		t.Errorf("no role in the notebook = %v, want notebook.not_found", err)
	}
}

// A changeset reads as the log lists it, with whether it renames or moves
// a node; one of a notebook deleted, the caller cannot see, or none at all
// is page.changeset_not_found.
func TestGetChangeset(t *testing.T) {
	f := newLogFixture()
	moved := f.page("Moved", nil, 3)
	from, to := moved.State(), domain.TreeState{ParentID: &f.a.ID, Name: "Moved", SortOrder: 0}
	move := f.log.changeset(domain.ChangesetEdit, f.bob, now(), []domain.ChangesetItem{{NodeID: moved.ID, Before: &from, After: &to}})
	uc := app.NewGetChangeset(f.historyReads())
	for _, tt := range []struct {
		c     *fakeChangeset
		moves domain.ChangesetMoves
	}{
		{f.renamed, domain.ChangesetMoves{Renames: true}},
		{move, domain.ChangesetMoves{Moves: true}},
		{f.made, domain.ChangesetMoves{}},
	} {
		f.rec.calls = nil
		got, err := uc.Execute(f.asAlice(), tt.c.ID)
		if err != nil || got.Changeset.ID != tt.c.ID || got.Moves != tt.moves || got.Changeset.ByName == "" {
			t.Errorf("GetChangeset(%s) = %+v, %v; want it with %+v", tt.c.ID, got, err, tt.moves)
		}
		want := []string{"FindChangeset", "WorkspaceOf", "Authorize page.history", "ChangesetFacts 1", "FirstChanges 1 3", "ChangesetMoves"}
		if !slices.Equal(f.rec.calls, want) {
			t.Errorf("calls = %v, want %v", f.rec.calls, want)
		}
	}
	got, err := uc.Execute(f.asAlice(), f.reverted.ID)
	if err != nil || got.Changeset.Reverts == nil || got.Changeset.Reverts.ByName != "Bob" {
		t.Errorf("the revert = %+v, %v; want bob's rename as what it reverts", got.Changeset, err)
	}
	for _, id := range []uuid.UUID{uuid.NewV7(), f.gone.ID, f.elsewhere.ID} {
		if _, err := uc.Execute(f.asAlice(), id); !errors.Is(err, domain.ErrChangesetNotFound) {
			t.Errorf("GetChangeset(%s) = %v, want page.changeset_not_found", id, err)
		}
	}
	delete(f.auth.grants, domain.ActionHistory)
	if _, err := uc.Execute(f.asAlice(), f.made.ID); !errors.Is(err, domain.ErrChangesetNotFound) {
		t.Errorf("no role in the notebook = %v, want page.changeset_not_found", err)
	}
}

// A changeset's changes come by node id, a page at a time; the cursor is
// judged first, then the changeset, then the limit.
func TestListChanges(t *testing.T) {
	f := newLogFixture()
	uc := app.NewListChanges(f.historyReads())
	one := 1
	first, err := uc.Execute(f.asAlice(), f.made.ID, &one, nil)
	lower := min(f.a.ID.String(), f.b.ID.String())
	if err != nil || len(first.Changes) != 1 || first.Changes[0].NodeID.String() != lower || first.NextCursor == "" {
		t.Fatalf("first page = %+v, %v; want the lower id's, a cursor", first, err)
	}
	second, err := uc.Execute(f.asAlice(), f.made.ID, &one, &first.NextCursor)
	if err != nil || len(second.Changes) != 1 || second.Changes[0].NodeID == first.Changes[0].NodeID || second.NextCursor != "" {
		t.Errorf("second page = %+v, %v; want the other, the last", second, err)
	}
	c := f.page("C", nil, 2)
	three := f.log.changeset(domain.ChangesetEdit, f.alice, now(), []domain.ChangesetItem{{NodeID: f.a.ID, After: new(f.a.State())},
		{NodeID: f.b.ID, After: new(f.b.State())}, {NodeID: c.ID, After: new(c.State())}})
	two := 2
	page, err := uc.Execute(f.asAlice(), three.ID, &two, nil)
	if err != nil {
		t.Fatal(err)
	}
	rest, err := uc.Execute(f.asAlice(), three.ID, &two, &page.NextCursor)
	last := slices.MaxFunc([]uuid.UUID{f.a.ID, f.b.ID, c.ID}, uuid.UUID.Compare)
	if err != nil || len(page.Changes) != 2 || len(rest.Changes) != 1 || rest.Changes[0].NodeID != last || rest.NextCursor != "" {
		t.Errorf("three changes two at a time = %+v, then %+v, %v; want the highest id's alone after the first two", page, rest, err)
	}
	bad, zero := "!", 0
	f.rec.calls = nil
	if _, err := uc.Execute(f.asAlice(), uuid.NewV7(), &zero, &bad); !errors.Is(err, shared.InvalidCursor()) || len(f.rec.calls) != 0 {
		t.Errorf("a bad cursor of no changeset = %v after %v, want bad_request first", err, f.rec.calls)
	}
	if _, err := uc.Execute(f.asAlice(), uuid.NewV7(), &zero, nil); !errors.Is(err, domain.ErrChangesetNotFound) {
		t.Errorf("no changeset, a bad limit = %v, want page.changeset_not_found", err)
	}
	var e *shared.Error
	if _, err := uc.Execute(f.asAlice(), f.made.ID, &zero, nil); !errors.As(err, &e) || e.Code != shared.CodeValidationFailed {
		t.Errorf("a bad limit = %v, want 422", err)
	}
}
