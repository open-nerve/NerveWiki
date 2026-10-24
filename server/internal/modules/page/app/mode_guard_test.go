package app_test

import (
	"errors"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// A standard notebook's steps the mode guard lets by at once (M10/P1
// design 3.20), those an LLM Wiki's contract refuses among them, of a
// standard notebook with a source root too (M11's upgrade may give one
// first): AGENTS deleted and reordered, index created at the root, a page
// renamed to index.md there, the source root renamed, which an LLM Wiki
// asks of the authorizer, here denying. The same steps of an LLM Wiki are
// refused, each with its code.
func TestTheModeGuardLetsAStandardNotebookBy(t *testing.T) {
	f := newFixture()
	agents, raw, notes, fresh := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	at := func(name string, order float64) *domain.TreeState {
		return &domain.TreeState{Name: name, SortOrder: order}
	}
	steps := []struct {
		name string
		op   domain.Operation
		c    domain.Change
		code string // an LLM Wiki's
	}{
		{"AGENTS deleted", domain.OpDelete, domain.Change{NodeID: agents, Before: at("AGENTS", 1)}, "page.protected"},
		{"AGENTS reordered", domain.OpMove, domain.Change{NodeID: agents, Before: at("AGENTS", 1), After: at("AGENTS", 0.5)},
			"page.protected"},
		{"index created at the root", domain.OpCreate, domain.Change{NodeID: fresh, After: at("index", 4), Revision: 1},
			"page.virtual_file"},
		{"a page renamed to index.md at the root", domain.OpRename, domain.Change{NodeID: notes, Before: at("Notes", 3),
			After: at("index.md", 3)}, "page.virtual_file"},
		{"the source root renamed", domain.OpRename, domain.Change{NodeID: raw, Before: at("raw", 2), After: at("sources", 2)},
			"page.source_admin_only"},
	}
	guard := app.NewModeGuard(f.auth)
	ctx := f.asAlice()
	for _, mode := range []domain.Mode{domain.ModeStandard, domain.ModeLLMWiki} {
		for _, s := range steps {
			step := app.Step{Write: app.Write{WorkspaceID: f.acme, NotebookID: f.eng, By: f.alice, Kind: domain.ChangesetEdit,
				Notebook: domain.NotebookState{Mode: mode, SourceRootID: raw}, Grants: &app.Grants{}}, Operation: s.op,
				Changes: []domain.Change{s.c}}
			err := guard.GuardWrite(ctx, step)
			if mode == domain.ModeStandard && err != nil || mode == domain.ModeLLMWiki && codeOf(err) != s.code {
				t.Errorf("%s in a %s notebook = %v, want %s", s.name, mode, err, map[domain.Mode]string{
					domain.ModeStandard: "it let by", domain.ModeLLMWiki: s.code}[mode])
			}
		}
	}
}

// The mode guard marks its refusal with the node it is about, not the
// step's first (M10/P1 design 3.20; review C-M1): Notes deleted with raw
// under it is page.protected of raw, R2's; Notes moved under Other with
// raw under it, page.manage_source denied, page.source_admin_only of raw,
// R4's. Unmarked, the unit would mark them with Notes.
func TestTheModeGuardMarksTheNodeItRefuses(t *testing.T) {
	f := newFixture()
	f.auth.forbidden[domain.ActionManageSource] = true
	notes, other, raw := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	guard := app.NewModeGuard(f.auth)
	for _, s := range []struct {
		name    string
		op      domain.Operation
		changes []domain.Change
		code    string
	}{
		{"Notes deleted, raw under it", domain.OpDelete, []domain.Change{{NodeID: notes, Before: &domain.TreeState{Name: "Notes"}},
			{NodeID: raw, Before: &domain.TreeState{ParentID: &notes, Name: "raw"}}}, "page.protected"},
		{"Notes moved under Other, raw under it", domain.OpMove, []domain.Change{
			{NodeID: notes, Before: &domain.TreeState{Name: "Notes"}, After: &domain.TreeState{ParentID: &other, Name: "Notes"}},
			{NodeID: raw, Before: &domain.TreeState{ParentID: &notes, Name: "raw"}, After: &domain.TreeState{ParentID: &notes, Name: "raw"}}},
			"page.source_admin_only"},
	} {
		step := app.Step{Write: app.Write{WorkspaceID: f.acme, NotebookID: f.eng, By: f.alice, Kind: domain.ChangesetEdit,
			Notebook: domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, Grants: &app.Grants{}}, Operation: s.op,
			Changes: s.changes}
		err := guard.GuardWrite(f.asAlice(), step)
		var r *domain.Refused
		if codeOf(err) != s.code || !errors.As(err, &r) || r.NodeID != raw {
			t.Errorf("%s = %v (%+v), want %s marked with raw", s.name, err, r, s.code)
		}
	}
}

// The mode guard asks page.manage_source once a unit (M10/P1 design 3.19):
// a batch that moves the source root twice, the guard's need granted, asks
// it once in its unit's transaction and writes both moves; two batches,
// two units, ask it twice. Denied once the plan, which asks apart, let the
// batch by (the role changed in between), the first move is refused in the
// unit, page.source_admin_only at ops[0], of the source root, and the unit
// rolls back.
func TestTheModeGuardAsksOnceAUnit(t *testing.T) {
	newWiki := func(grant bool) (*fixture, domain.Node) {
		f := newFixture()
		raw := f.page("raw", nil, 1)
		f.page("Notes", nil, 2)
		f.page("Other", nil, 3)
		f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
		f.guards = []app.WriteGuard{app.NewModeGuard(f.auth)}
		if grant {
			f.grant(domain.ActionManageSource)
		}
		return f, raw
	}
	asked := func(f *fixture) int { return f.count("Authorize page.manage_source in tx") }

	f, raw := newWiki(true)
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, moveOp("raw.md", "Notes/raw.md"), moveOp("Notes/raw.md", "Other/raw.md")); err != nil {
		t.Fatal(err)
	}
	if n := asked(f); n != 1 || f.store.nodes[raw.ID].ParentID == nil || f.store.nodes[*f.store.nodes[raw.ID].ParentID].Name != "Other" {
		t.Errorf("a batch of two moves of raw asked page.manage_source %d times in its unit, raw under %v; want once, raw under Other",
			n, f.store.nodes[raw.ID].ParentID)
	}

	f, _ = newWiki(true)
	for _, op := range []domain.BatchArgs{moveOp("raw.md", "Notes/raw.md"), moveOp("Notes/raw.md", "Other/raw.md")} {
		if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, op); err != nil {
			t.Fatal(err)
		}
	}
	if n := asked(f); n != 2 {
		t.Errorf("two batches of a move of raw each asked page.manage_source %d times in their units, want twice", n)
	}

	f, raw = newWiki(true)
	f.rec.before = map[string]func(){"LockNotebook in tx": func() {
		f.auth.grants[domain.ActionManageSource], f.auth.forbidden[domain.ActionManageSource] = false, true
	}}
	_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, moveOp("raw.md", "Notes/raw.md"), moveOp("Notes/raw.md", "Other/raw.md"))
	var r *domain.Refused
	if opOf(err) != "0 page.source_admin_only" || !errors.As(err, &r) || r.NodeID != raw.ID || asked(f) != 1 || !f.tx.rolledBack ||
		f.count("Authorize page.manage_source") != 1 {
		t.Errorf("a batch of two moves of raw by an admin made a writer = %s (%v) asking %d times in the unit; want ops[0]'s "+
			"page.source_admin_only of raw, asked once there and once by the plan, rolled back", opOf(err), err, asked(f))
	}
}
