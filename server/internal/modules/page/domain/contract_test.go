package domain_test

import (
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The reserved keys are title keys (M10/P1 design 3.21): each is its own
// key, and the names that fold to them, in any case, are reserved; a
// full-width index is not one of them.
func TestTheReservedKeysAreTitleKeys(t *testing.T) {
	for _, k := range domain.ReservedKeys() {
		if shared.TitleKey(k) != k || !domain.Reserved(k) {
			t.Errorf("reserved key %q: its key is %q, Reserved = %v; want itself, reserved", k, shared.TitleKey(k), domain.Reserved(k))
		}
	}
	for _, name := range []string{"Index", "LOG", "Log.MD", "INDEX.md"} {
		if !domain.Reserved(shared.TitleKey(name)) {
			t.Errorf("%q is not reserved, want it: its key is %q", name, shared.TitleKey(name))
		}
	}
	for _, name := range []string{"ｉｎｄｅｘ", "index 2", "logs", "index.markdown", "Log.png"} {
		if domain.Reserved(shared.TitleKey(name)) {
			t.Errorf("%q is reserved, want it not", name)
		}
	}
}

// contractTree is an LLM Wiki's nodes as the contract's tests name them:
// at the root AGENTS, raw (the source root), Notes and Other; under Notes,
// Inner; under Other, a page named AGENTS and a page named index.
type contractTree struct {
	agents, raw, notes, other, inner, deepAgents, deepIndex uuid.UUID
}

// at is a tree state under parent, nil for the root, named name at order.
func at(parent *uuid.UUID, name string, order float64) *domain.TreeState {
	return &domain.TreeState{ParentID: parent, Name: name, SortOrder: order}
}

// Each cell of the step × node table of M10/P1 design 3.21, by the changes
// a unit makes, in their order, each judged once for an LLM Wiki and once
// for a standard notebook of the same source root, which every step lets
// by: the rules look at the mode first.
func TestCheckContract(t *testing.T) {
	var x contractTree
	for _, id := range []*uuid.UUID{&x.agents, &x.raw, &x.notes, &x.other, &x.inner, &x.deepAgents, &x.deepIndex} {
		*id = uuid.NewV7()
	}
	wiki := domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: x.raw}
	standard := domain.NotebookState{Mode: domain.ModeStandard, SourceRootID: x.raw}
	protected := func(id uuid.UUID) domain.Verdict { return domain.Verdict{Err: domain.ErrProtected, NodeID: id} }
	virtual := func(id uuid.UUID) domain.Verdict { return domain.Verdict{Err: domain.ErrVirtualFile, NodeID: id} }
	needs := domain.Verdict{Needs: domain.ActionManageSource, NodeID: x.raw}
	let := domain.Verdict{}
	// The changes of each kind of step, as the units make them.
	create := func(id uuid.UUID, parent *uuid.UUID, name string) []domain.Change {
		return []domain.Change{{NodeID: id, After: at(parent, name, 9), Revision: 1}}
	}
	rename := func(id uuid.UUID, parent *uuid.UUID, from, to string) []domain.Change {
		return []domain.Change{{NodeID: id, Before: at(parent, from, 1), After: at(parent, to, 1)}}
	}
	content := func(id uuid.UUID, parent *uuid.UUID, name string) []domain.Change {
		return []domain.Change{{NodeID: id, Before: at(parent, name, 1), After: at(parent, name, 1), Revision: 2}}
	}
	// move moves id from under from to under to, named name there; the
	// nodes carried go with it, each in its place, after it.
	move := func(id uuid.UUID, from *uuid.UUID, fromName string, to *uuid.UUID, toName string, carried ...uuid.UUID) []domain.Change {
		out := []domain.Change{{NodeID: id, Before: at(from, fromName, 1), After: at(to, toName, 7)}}
		for _, c := range carried {
			out = append(out, domain.Change{NodeID: c, Before: at(&id, "Carried", 1), After: at(&id, "Carried", 1)})
		}
		return out
	}
	// reorder puts id at another order under its parent.
	reorder := func(id uuid.UUID, parent *uuid.UUID, name string, carried ...uuid.UUID) []domain.Change {
		out := []domain.Change{{NodeID: id, Before: at(parent, name, 1), After: at(parent, name, 0.5)}}
		for _, c := range carried {
			out = append(out, domain.Change{NodeID: c, Before: at(&id, "Carried", 1), After: at(&id, "Carried", 1)})
		}
		return out
	}
	// remove deletes id and the nodes under it, after it.
	remove := func(id uuid.UUID, parent *uuid.UUID, name string, under ...uuid.UUID) []domain.Change {
		out := []domain.Change{{NodeID: id, Before: at(parent, name, 1)}}
		for _, u := range under {
			out = append(out, domain.Change{NodeID: u, Before: at(&id, "Under", 1)})
		}
		return out
	}
	restore := func(id uuid.UUID, parent *uuid.UUID, name string) []domain.Change {
		return []domain.Change{{NodeID: id, After: at(parent, name, 3), Restored: &domain.Restored{}}}
	}
	fresh := uuid.NewV7()
	for _, tt := range []struct {
		name    string
		op      domain.Operation
		kind    domain.ChangesetKind
		changes []domain.Change
		want    domain.Verdict
	}{
		// A new page (REST, MCP's write, a batch's, a parent made on the way)
		// and an upload: a reserved title at the root.
		{"a page created at the root as index", domain.OpCreate, domain.ChangesetEdit, create(fresh, nil, "index"), virtual(fresh)},
		{"a page created at the root as Index.md", domain.OpCreate, domain.ChangesetEdit, create(fresh, nil, "Index.md"), virtual(fresh)},
		{"an attachment uploaded at the root as LOG", domain.OpCreate, domain.ChangesetEdit, create(fresh, nil, "LOG"), virtual(fresh)},
		{"a page created at the root as log.md", domain.OpCreate, domain.ChangesetEdit, create(fresh, nil, "log.md"), virtual(fresh)},
		{"a page created under Notes as index", domain.OpCreate, domain.ChangesetEdit, create(fresh, &x.notes, "index"), let},
		{"a page created at the root as Indexes", domain.OpCreate, domain.ChangesetEdit, create(fresh, nil, "Indexes"), let},
		{"an attachment uploaded at the root as log.png", domain.OpCreate, domain.ChangesetEdit, create(fresh, nil, "log.png"), let},
		// An import's creations: numbered, not refused (M10/P1/S3).
		{"an import's page at the root as index", domain.OpCreate, domain.ChangesetImport, create(fresh, nil, "index"), let},
		{"an import's page under raw", domain.OpCreate, domain.ChangesetImport, create(fresh, &x.raw, "Paper"), let},
		// A content write: AGENTS and the source root are written as any page.
		{"AGENTS's content", domain.OpContent, domain.ChangesetEdit, content(x.agents, nil, "AGENTS"), let},
		{"raw's content", domain.OpContent, domain.ChangesetEdit, content(x.raw, nil, "raw"), let},
		{"a version of AGENTS restored", domain.OpContent, domain.ChangesetVersionRestore, content(x.agents, nil, "AGENTS"), let},
		{"another page's content", domain.OpContent, domain.ChangesetEdit, content(x.notes, nil, "Notes"), let},
		// A rename (REST, MCP's move under the same parent, a batch's).
		{"AGENTS renamed", domain.OpRename, domain.ChangesetEdit, rename(x.agents, nil, "AGENTS", "Rules"), protected(x.agents)},
		{"AGENTS renamed in case alone", domain.OpRename, domain.ChangesetEdit, rename(x.agents, nil, "AGENTS", "Agents"), protected(x.agents)},
		{"a root's Agents renamed", domain.OpRename, domain.ChangesetEdit, rename(x.agents, nil, "Agents", "Rules"), protected(x.agents)},
		{"a page named AGENTS under Other renamed", domain.OpRename, domain.ChangesetEdit,
			rename(x.deepAgents, &x.other, "AGENTS", "Rules"), let},
		{"raw renamed", domain.OpRename, domain.ChangesetEdit, rename(x.raw, nil, "raw", "sources"), needs},
		{"raw's ancestor renamed", domain.OpRename, domain.ChangesetEdit, rename(x.notes, nil, "Notes", "Papers"), let},
		{"a page renamed to index.md at the root", domain.OpRename, domain.ChangesetEdit, rename(x.notes, nil, "Notes", "index.md"),
			virtual(x.notes)},
		{"a page renamed to log under Other", domain.OpRename, domain.ChangesetEdit, rename(x.inner, &x.other, "Inner", "log"), let},
		{"a root's index renamed in case alone", domain.OpRename, domain.ChangesetEdit, rename(fresh, nil, "index", "Index"), let},
		{"another page renamed", domain.OpRename, domain.ChangesetEdit, rename(x.other, nil, "Other", "Others"), let},
		// raw renamed to index at the root breaks R3 and R4: R3's code first.
		{"raw renamed to index", domain.OpRename, domain.ChangesetEdit, rename(x.raw, nil, "raw", "index"), virtual(x.raw)},
		// A move to another parent.
		{"AGENTS moved under Notes", domain.OpMove, domain.ChangesetEdit, move(x.agents, nil, "AGENTS", &x.notes, "AGENTS"),
			protected(x.agents)},
		{"raw moved under Notes", domain.OpMove, domain.ChangesetEdit, move(x.raw, nil, "raw", &x.notes, "raw"), needs},
		{"Notes, raw under it, moved under Other", domain.OpMove, domain.ChangesetEdit,
			move(x.notes, nil, "Notes", &x.other, "Notes", x.inner, x.raw), needs},
		{"a page moved to the root as log", domain.OpMove, domain.ChangesetEdit, move(x.inner, &x.notes, "log", nil, "log"),
			virtual(x.inner)},
		{"a page named index moved under Notes", domain.OpMove, domain.ChangesetEdit,
			move(x.deepIndex, &x.other, "index", &x.notes, "index"), let},
		{"another page moved under Other", domain.OpMove, domain.ChangesetEdit, move(x.notes, nil, "Notes", &x.other, "Notes", x.inner),
			let},
		// A reorder under the same parent.
		{"AGENTS reordered", domain.OpMove, domain.ChangesetEdit, reorder(x.agents, nil, "AGENTS"), protected(x.agents)},
		{"raw reordered", domain.OpMove, domain.ChangesetEdit, reorder(x.raw, nil, "raw"), needs},
		{"Notes, raw under it, reordered", domain.OpMove, domain.ChangesetEdit, reorder(x.notes, nil, "Notes", x.inner, x.raw), let},
		{"another page reordered", domain.OpMove, domain.ChangesetEdit, reorder(x.other, nil, "Other"), let},
		// A move and a rename in one step (Relocate to another parent).
		{"AGENTS relocated", domain.OpMove, domain.ChangesetEdit, move(x.agents, nil, "AGENTS", &x.notes, "Rules"),
			protected(x.agents)},
		{"raw relocated", domain.OpMove, domain.ChangesetEdit, move(x.raw, nil, "raw", &x.notes, "sources"), needs},
		{"Notes, raw under it, relocated", domain.OpMove, domain.ChangesetEdit,
			move(x.notes, nil, "Notes", &x.other, "Papers", x.raw), needs},
		{"a page relocated to the root as Index.md", domain.OpMove, domain.ChangesetEdit,
			move(x.inner, &x.notes, "Inner", nil, "Index.md"), virtual(x.inner)},
		{"another page relocated", domain.OpMove, domain.ChangesetEdit, move(x.inner, &x.notes, "Inner", &x.other, "Outer"), let},
		// A deletion.
		{"AGENTS deleted", domain.OpDelete, domain.ChangesetEdit, remove(x.agents, nil, "AGENTS"), protected(x.agents)},
		{"raw deleted", domain.OpDelete, domain.ChangesetEdit, remove(x.raw, nil, "raw", fresh), protected(x.raw)},
		{"Notes, raw under it, deleted", domain.OpDelete, domain.ChangesetEdit, remove(x.notes, nil, "Notes", x.inner, x.raw),
			protected(x.raw)},
		{"a page named AGENTS under Other deleted", domain.OpDelete, domain.ChangesetEdit, remove(x.deepAgents, &x.other, "AGENTS"), let},
		{"another page deleted", domain.OpDelete, domain.ChangesetEdit, remove(x.other, nil, "Other", x.deepAgents), let},
		// A restore from the trash: numbered, not refused (M10/P1/S3).
		{"a page restored at the root as index", domain.OpRestore, domain.ChangesetTrashRestore, restore(fresh, nil, "index"), let},
		{"another page restored", domain.OpRestore, domain.ChangesetTrashRestore, restore(fresh, &x.notes, "Gone"), let},
		// A revert's put back, as a rename or a move is judged: its names
		// aside are names as any other.
		{"a revert renames AGENTS aside", domain.OpRename, domain.ChangesetRevert, rename(x.agents, nil, "AGENTS", "AGENTS ~1a2b3c4d"),
			protected(x.agents)},
		{"a revert renames raw aside", domain.OpRename, domain.ChangesetRevert, rename(x.raw, nil, "raw", "raw ~1a2b3c4d"), needs},
		{"a revert puts raw back", domain.OpRename, domain.ChangesetRevert, rename(x.raw, nil, "sources", "raw"), needs},
		{"a revert puts Notes, raw under it, under another parent", domain.OpMove, domain.ChangesetRevert,
			move(x.notes, &x.other, "Notes", nil, "Notes", x.raw), needs},
		{"a revert puts Notes, raw under it, back under its parent", domain.OpMove, domain.ChangesetRevert,
			reorder(x.notes, nil, "Notes", x.raw), let},
		{"a revert's put back of a page to the root as log", domain.OpMove, domain.ChangesetRevert,
			move(x.inner, &x.notes, "log", nil, "log"), virtual(x.inner)},
		{"a revert's put back of another page", domain.OpMove, domain.ChangesetRevert, move(x.inner, &x.other, "Inner", &x.notes, "Inner"),
			let},
		// A revert's deletion and restore.
		{"a revert deletes AGENTS", domain.OpDelete, domain.ChangesetRevert, remove(x.agents, nil, "AGENTS"), protected(x.agents)},
		{"a revert deletes raw", domain.OpDelete, domain.ChangesetRevert, remove(x.raw, nil, "raw"), protected(x.raw)},
		{"a revert deletes Notes, raw under it", domain.OpDelete, domain.ChangesetRevert, remove(x.notes, nil, "Notes", x.raw),
			protected(x.raw)},
		{"a revert restores a page at the root as log", domain.OpRestore, domain.ChangesetRevert, restore(fresh, nil, "log"), let},
	} {
		facts := domain.FactsOf(tt.op, tt.kind, tt.changes)
		if got := domain.CheckContract(wiki, facts); got != tt.want {
			t.Errorf("%s: CheckContract(an LLM Wiki) = %+v, want %+v", tt.name, got, tt.want)
		}
		if got := domain.CheckContract(standard, facts); got != (domain.Verdict{}) {
			t.Errorf("%s: CheckContract(a standard notebook) = %+v, want it let by", tt.name, got)
		}
	}
}

// The facts of a step are its changes' nodes, in their order, each place
// by its parent and its name's title key; none for a creation's before
// and a deletion's after.
func TestFactsOfAStep(t *testing.T) {
	a, b, parent := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	s := domain.FactsOf(domain.OpMove, domain.ChangesetRevert, []domain.Change{
		{NodeID: a, Before: at(nil, "Ｎotes Ω", 1), After: at(&parent, "NOTES", 2)},
		{NodeID: b, After: at(&a, "Straße", 1)},
		{NodeID: parent, Before: at(nil, "Index.MD", 1)},
	})
	if s.Operation != domain.OpMove || s.Kind != domain.ChangesetRevert || len(s.Nodes) != 3 {
		t.Fatalf("FactsOf = %+v, want a revert's move of three nodes", s)
	}
	n := s.Nodes
	switch {
	case n[0].ID != a || n[0].Before.ParentID != nil || n[0].Before.Key != shared.TitleKey("Ｎotes Ω") ||
		n[0].After.ParentID == nil || *n[0].After.ParentID != parent || n[0].After.Key != "notes":
		t.Errorf("the first node = %+v (%+v, %+v), want a's from the root to parent, keyed", n[0], n[0].Before, n[0].After)
	case n[1].ID != b || n[1].Before != nil || n[1].After.Key != "strasse":
		t.Errorf("the second node = %+v, want b created, keyed strasse", n[1])
	case n[2].ID != parent || n[2].After != nil || n[2].Before.Key != "index.md":
		t.Errorf("the third node = %+v, want parent deleted, keyed index.md", n[2])
	}
}
