package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// changeMode is the notebook module's notebook.change_mode, which
// bootstrap hands the freeze guard.
const changeMode = shared.Action("notebook.change_mode")

// upgradeNodes are an upgrade's rows in memory: protected the ids that have
// one, of any upgrade; baseline the contents now of its baseline's pages;
// reads the reads made, each its upgrade's id and the ids asked, contents
// those of a baseline page's content; err fails each read.
type upgradeNodes struct {
	protected map[uuid.UUID]bool
	baseline  map[uuid.UUID]string
	reads     [][]uuid.UUID
	contents  [][2]uuid.UUID
	err       error
}

func (n *upgradeNodes) BaselineContent(_ context.Context, upgradeID, id uuid.UUID) (string, bool, error) {
	n.contents = append(n.contents, [2]uuid.UUID{upgradeID, id})
	if n.err != nil {
		return "", false, n.err
	}
	content, ok := n.baseline[id]
	return content, ok, nil
}

// frontmatters tells where a content's frontmatter ends as fakeMarkdown
// does, recording nothing.
type frontmatters struct{}

func (frontmatters) FrontmatterEnd(content string) int {
	if !strings.HasPrefix(content, "---\n") {
		return 0
	}
	if i := strings.Index(content[3:], "\n---\n"); i >= 0 {
		return 3 + i + len("\n---\n")
	}
	return 0
}

// bodyHash is the body's hash of content as the unit hashes it for the
// guard, its frontmatter ending where frontmatters tells.
func bodyHash(content string) []byte {
	return domain.UpgradeBodyHash(content, frontmatters{}.FrontmatterEnd(content))
}

func (n *upgradeNodes) Protected(_ context.Context, upgradeID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	n.reads = append(n.reads, append([]uuid.UUID{upgradeID}, ids...))
	if n.err != nil {
		return nil, n.err
	}
	out := map[uuid.UUID]bool{}
	for _, id := range ids {
		if n.protected[id] {
			out[id] = true
		}
	}
	return out, nil
}

// upgrading is eng of f being upgraded by alice, in state, the upgrade id.
func (f *fixture) upgrading(state domain.UpgradeState, id uuid.UUID) domain.NotebookState {
	return domain.NotebookState{Mode: domain.ModeStandard, Upgrade: domain.NotebookUpgrade{ID: id, State: state, By: f.alice}}
}

// freezeStep is a step of eng of f by alice from client in n, of a unit
// whose memo is grants.
func (f *fixture) freezeStep(n domain.NotebookState, client domain.Client, grants *app.Grants, op domain.Operation,
	changes ...domain.Change,
) app.Step {
	return app.Step{Write: app.Write{WorkspaceID: f.acme, NotebookID: f.eng, By: f.alice, Client: client, Kind: domain.ChangesetEdit,
		Notebook: n, Grants: grants}, Operation: op, Changes: changes}
}

// The freeze guard (M11/P1 design 3.4): a notebook with no upgrade it lets
// by asking nothing, reading no row; the initiator's web write while upgrading it refuses
// without asking; his MCP write asks notebook.change_mode once a unit, two
// steps of one unit asking it once and two units twice, and is let by
// when granted; refused or not visible, the write is notebook.upgrading;
// the authorizer's own error is answered as it is; a unit of another
// upgrade is the defect's ErrStaleUpgradeWrite, no problem. The vetoer
// refuses any opening while an upgrade is under way, its initiator's too,
// and lets one by in a notebook with none.
func TestTheFreezeGuard(t *testing.T) {
	agent := domain.MCPClient("claude-code")
	page := domain.Change{NodeID: uuid.NewV7(), After: &domain.TreeState{Name: "Notes"}}
	current := uuid.NewV7()
	asked := func(f *fixture) int { return f.count("Authorize " + string(changeMode)) }

	t.Run("no upgrade", func(t *testing.T) {
		f := newFixture()
		nodes := &upgradeNodes{}
		guard := app.NewFreezeGuard(f.auth, changeMode, nodes, frontmatters{})
		for _, op := range []domain.Operation{domain.OpCreate, domain.OpDelete} {
			err := guard.GuardWrite(f.asAlice(), f.freezeStep(domain.NotebookState{Mode: domain.ModeStandard}, domain.ClientWeb, &app.Grants{},
				op, page))
			if err != nil || len(f.rec.calls) != 0 || len(nodes.reads) != 0 {
				t.Errorf("a %s of a notebook with no upgrade = %v after %v, %d reads; want nil asking nothing, reading nothing", op, err,
					f.rec.calls, len(nodes.reads))
			}
		}
	})
	t.Run("the initiator's web write", func(t *testing.T) {
		f := newFixture()
		f.grant(changeMode)
		guard := app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{}, frontmatters{})
		err := guard.GuardWrite(f.asAlice(), f.freezeStep(f.upgrading(domain.UpgradeUpgrading, current), domain.ClientWeb, &app.Grants{},
			domain.OpCreate, page))
		if codeOf(err) != "notebook.upgrading" || asked(f) != 0 {
			t.Errorf("the initiator's web write = %v, asked %d; want notebook.upgrading, asking nothing", err, asked(f))
		}
	})
	t.Run("the initiator's MCP writes ask once a unit", func(t *testing.T) {
		f := newFixture()
		f.grant(changeMode)
		guard := app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{}, frontmatters{})
		n := f.upgrading(domain.UpgradeUpgrading, current)
		unit := &app.Grants{}
		for range 2 {
			if err := guard.GuardWrite(f.asAlice(), f.freezeStep(n, agent, unit, domain.OpCreate, page)); err != nil {
				t.Fatalf("the initiator's MCP write, granted = %v, want nil", err)
			}
		}
		if got := asked(f); got != 1 {
			t.Errorf("two steps of one unit asked notebook.change_mode %d times, want once", got)
		}
		if err := guard.GuardWrite(f.asAlice(), f.freezeStep(n, agent, &app.Grants{}, domain.OpCreate, page)); err != nil {
			t.Fatal(err)
		}
		if got := asked(f); got != 2 {
			t.Errorf("a second unit asked: %d times in all, want twice", got)
		}
		if target := f.auth.targets[0]; target != (shared.Target{WorkspaceID: f.acme, NotebookID: f.eng}) {
			t.Errorf("asked of %+v, want eng of acme", target)
		}
	})
	for _, denial := range []struct {
		name string
		set  func(f *fixture)
	}{
		{"forbidden", func(f *fixture) { f.auth.forbidden[changeMode] = true }},
		{"not visible", func(*fixture) {}},
	} {
		t.Run("the initiator's MCP write, "+denial.name, func(t *testing.T) {
			f := newFixture()
			denial.set(f)
			guard := app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{}, frontmatters{})
			unit := &app.Grants{}
			n := f.upgrading(domain.UpgradeUpgrading, current)
			for range 2 {
				if err := guard.GuardWrite(f.asAlice(), f.freezeStep(n, agent, unit, domain.OpCreate, page)); codeOf(err) != "notebook.upgrading" {
					t.Errorf("the initiator's MCP write, %s = %v, want notebook.upgrading", denial.name, err)
				}
			}
			if got := asked(f); got != 1 {
				t.Errorf("asked %d times in one unit, want once", got)
			}
		})
	}
	t.Run("the authorizer's error", func(t *testing.T) {
		f := newFixture()
		failure := errors.New("the database is gone")
		f.auth.failing = map[shared.Action]error{changeMode: failure}
		guard := app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{}, frontmatters{})
		err := guard.GuardWrite(f.asAlice(), f.freezeStep(f.upgrading(domain.UpgradeUpgrading, current), agent, &app.Grants{},
			domain.OpCreate, page))
		if !errors.Is(err, failure) {
			t.Errorf("the initiator's MCP write, the authorizer failing = %v, want its error", err)
		}
	})
	t.Run("a stale unit", func(t *testing.T) {
		f := newFixture()
		f.grant(changeMode)
		guard := app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{}, frontmatters{})
		s := f.freezeStep(f.upgrading(domain.UpgradeRollingBack, current), domain.ClientWeb, &app.Grants{}, domain.OpCreate, page)
		s.UpgradeOwn = uuid.NewV7()
		err := guard.GuardWrite(f.asAlice(), s)
		var problem *shared.Error
		if !errors.Is(err, domain.ErrStaleUpgradeWrite) || errors.As(err, &problem) || len(f.rec.calls) != 0 {
			t.Errorf("a unit of another upgrade = %v after %v, want ErrStaleUpgradeWrite, no problem, asking nothing", err, f.rec.calls)
		}
	})
	// A freeze the guard does not know is a defect's error, no problem's:
	// it asks nothing and reads no row, and its unit writes nothing, though
	// every action is granted.
	t.Run("a freeze it does not know", func(t *testing.T) {
		f := newFixture()
		f.grant(changeMode, domain.ActionCreate)
		nodes := &upgradeNodes{}
		unknown := func(domain.NotebookState, uuid.UUID, domain.Client, uuid.UUID) domain.Freeze {
			return domain.FreezeAsk + 1
		}
		guard := app.NewFreezeGuard(f.auth, changeMode, nodes, frontmatters{}).WithAdmits(unknown)
		err := guard.GuardWrite(f.asAlice(), f.freezeStep(f.upgrading(domain.UpgradeUpgrading, current), agent, &app.Grants{},
			domain.OpCreate, page))
		var problem *shared.Error
		if err == nil || errors.As(err, &problem) || len(f.rec.calls) != 0 || len(nodes.reads) != 0 {
			t.Errorf("an unknown freeze = %v after %v, %d reads; want a defect's error, asking and reading nothing", err, f.rec.calls,
				len(nodes.reads))
		}
		f.guards = []app.WriteGuard{guard}
		if _, err := f.writer().Run(f.asAlice(), app.UnitSpec{NotebookID: f.eng, Action: domain.ActionCreate, Tree: true, Client: agent,
			NotFound: domain.ErrNotebookNotFound}, create("Notes")); err == nil || len(f.store.nodes) != 0 || len(f.store.changesets) != 0 {
			t.Errorf("a unit of an unknown freeze = %v, %d nodes, %d changesets; want its error, nothing written", err,
				len(f.store.nodes), len(f.store.changesets))
		}
	})
	// The deletion guardrail: a subtree whose root the agent created, a
	// baseline page under it, and the protected source root under that, is
	// refused, page.upgrade_protected marked with the baseline page, the
	// first protected in the subtree's order, not with its root; the
	// upgrade's rows read once, of the notebook's upgrade, the subtree's
	// ids. A subtree all created since goes; a step not a delete reads no
	// row; the reader's error is answered as it is.
	t.Run("the deletion guardrail", func(t *testing.T) {
		created, baseline, root, fresh := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
		subtree := []domain.Change{{NodeID: created, Before: &domain.TreeState{Name: "Made"}},
			{NodeID: baseline, Before: &domain.TreeState{ParentID: &created, Name: "Guide"}},
			{NodeID: root, Before: &domain.TreeState{ParentID: &baseline, Name: "raw"}}}
		f := newFixture()
		f.grant(changeMode)
		nodes := &upgradeNodes{protected: map[uuid.UUID]bool{baseline: true, root: true}}
		guard := app.NewFreezeGuard(f.auth, changeMode, nodes, frontmatters{})
		n := f.upgrading(domain.UpgradeUpgrading, current)
		err := guard.GuardWrite(f.asAlice(), f.freezeStep(n, agent, &app.Grants{}, domain.OpDelete, subtree...))
		var r *domain.Refused
		if codeOf(err) != "page.upgrade_protected" || !errors.As(err, &r) || r.NodeID != baseline {
			t.Errorf("the deletion of a subtree holding baseline nodes = %v (%+v), want page.upgrade_protected marked with Guide", err, r)
		}
		if want := [][]uuid.UUID{{current, created, baseline, root}}; !slices.EqualFunc(nodes.reads, want, slices.Equal) {
			t.Errorf("read %v, want the upgrade's rows of the subtree once, %v", nodes.reads, want)
		}
		nodes.reads = nil
		if err := guard.GuardWrite(f.asAlice(), f.freezeStep(n, agent, &app.Grants{}, domain.OpDelete,
			domain.Change{NodeID: fresh, Before: &domain.TreeState{Name: "New"}})); err != nil {
			t.Errorf("the deletion of a page created since = %v, want nil", err)
		}
		nodes.reads = nil
		for _, op := range []domain.Operation{domain.OpMove, domain.OpRename, domain.OpCreate} {
			s := f.freezeStep(n, agent, &app.Grants{}, op, subtree[1])
			s.Options.UpdateLinks = true
			if err := guard.GuardWrite(f.asAlice(), s); err != nil {
				t.Errorf("a %s of a baseline page = %v, want nil (P2/S1's guardrails)", op, err)
			}
		}
		if len(nodes.reads) != 0 {
			t.Errorf("steps not a delete read %v, want nothing", nodes.reads)
		}
		failure := errors.New("the database is gone")
		nodes.err = failure
		if err := guard.GuardWrite(f.asAlice(), f.freezeStep(n, agent, &app.Grants{}, domain.OpDelete, subtree...)); !errors.Is(err, failure) {
			t.Errorf("the deletion, the reader failing = %v, want its error", err)
		}
	})
	// The moves' and renames' guardrail (M11/P2 design 3.2): a move or a
	// rename of a baseline page with what it carries that leaves the links
	// to them as they are, a single move's (Options.UpdateLinks false) or a
	// batch's (KeepLinks, its unit's UpdateLinks true), is
	// page.upgrade_keep_links marked with the step's root; one that
	// rewrites them goes. Neither reads a row.
	t.Run("the moves' and renames' guardrail", func(t *testing.T) {
		f := newFixture()
		f.grant(changeMode)
		root, carried := uuid.NewV7(), uuid.NewV7()
		nodes := &upgradeNodes{protected: map[uuid.UUID]bool{root: true, carried: true}}
		guard := app.NewFreezeGuard(f.auth, changeMode, nodes, frontmatters{})
		n := f.upgrading(domain.UpgradeUpgrading, current)
		changes := []domain.Change{{NodeID: root, Before: &domain.TreeState{Name: "Guide"}, After: &domain.TreeState{Name: "Map"}},
			{NodeID: carried, Before: &domain.TreeState{ParentID: &root, Name: "Kept"}, After: &domain.TreeState{ParentID: &root, Name: "Kept"}}}
		for _, op := range []domain.Operation{domain.OpMove, domain.OpRename} {
			for _, tt := range []struct {
				name         string
				update, keep bool
				refused      bool
			}{
				{"a single one with update_links false", false, false, true},
				{"a batch's with update_links false", true, true, true},
				{"one that rewrites the links", true, false, false},
			} {
				s := f.freezeStep(n, agent, &app.Grants{}, op, changes...)
				s.Options.UpdateLinks, s.KeepLinks = tt.update, tt.keep
				err := guard.GuardWrite(f.asAlice(), s)
				var r *domain.Refused
				switch {
				case tt.refused && (codeOf(err) != "page.upgrade_keep_links" || !errors.As(err, &r) || r.NodeID != root):
					t.Errorf("%s, %s = %v (%+v), want page.upgrade_keep_links marked with Guide", op, tt.name, err, r)
				case !tt.refused && err != nil:
					t.Errorf("%s, %s = %v, want nil", op, tt.name, err)
				}
			}
		}
		if len(nodes.reads) != 0 {
			t.Errorf("the moves and renames read %v, want nothing", nodes.reads)
		}
	})
	// The body's guardrail (M11/P2 design 3.3): a write of a baseline page
	// whose body, past its frontmatter, is not the one it holds now is
	// page.upgrade_immutable marked with the page; one that changes its
	// frontmatter alone goes; each reads the page's content once, of the
	// notebook's upgrade, and no other row. The system's rewrite (Rewrite)
	// goes unread, though its body changed and it carries no hash; a page
	// not of the baseline goes, its one read answering none; a step of the
	// initiator's without the body's hash is a defect's error, no problem,
	// reading nothing; the reader's error is answered as it is.
	t.Run("the body's guardrail", func(t *testing.T) {
		f := newFixture()
		f.grant(changeMode)
		plan, fresh := uuid.NewV7(), uuid.NewV7()
		nodes := &upgradeNodes{baseline: map[uuid.UUID]string{plan: "---\na: 1\n---\nText"}}
		guard := app.NewFreezeGuard(f.auth, changeMode, nodes, frontmatters{})
		n := f.upgrading(domain.UpgradeUpgrading, current)
		step := func(id uuid.UUID, content string, rewrite bool) app.Step {
			s := f.freezeStep(n, agent, &app.Grants{}, domain.OpContent, domain.Change{NodeID: id})
			s.Content, s.Rewrite = &app.StepContent{BodyHash: bodyHash(content)}, rewrite
			return s
		}
		for _, tt := range []struct {
			name, content string
			refused       bool
		}{
			{"its frontmatter alone changed", "---\na: 2\nb: 3\n---\nText", false},
			{"a frontmatter added before the old one", "---\nb: 3\n---\n---\na: 1\n---\nText", true},
			{"its body changed", "---\na: 1\n---\nTexT", true},
		} {
			nodes.contents = nil
			err := guard.GuardWrite(f.asAlice(), step(plan, tt.content, false))
			var r *domain.Refused
			switch {
			case tt.refused && (codeOf(err) != "page.upgrade_immutable" || !errors.As(err, &r) || r.NodeID != plan):
				t.Errorf("a write of Plan, %s = %v (%+v), want page.upgrade_immutable marked with Plan", tt.name, err, r)
			case !tt.refused && err != nil:
				t.Errorf("a write of Plan, %s = %v, want nil", tt.name, err)
			}
			if want := [][2]uuid.UUID{{current, plan}}; !slices.Equal(nodes.contents, want) || len(nodes.reads) != 0 {
				t.Errorf("a write of Plan, %s, read the contents %v and the rows %v; want %v alone", tt.name, nodes.contents, nodes.reads,
					want)
			}
		}
		nodes.contents = nil
		rewrite := step(plan, "---\na: 1\n---\n[[Moved]]", true)
		rewrite.Content.BodyHash = nil
		if err := guard.GuardWrite(f.asAlice(), rewrite); err != nil || len(nodes.contents) != 0 {
			t.Errorf("the system's rewrite of Plan's links = %v, read %v; want nil, reading nothing", err, nodes.contents)
		}
		if err := guard.GuardWrite(f.asAlice(), step(fresh, "new", false)); err != nil ||
			!slices.Equal(nodes.contents, [][2]uuid.UUID{{current, fresh}}) {
			t.Errorf("a write of a page created since = %v, read %v; want nil, its one read", err, nodes.contents)
		}
		nodes.contents = nil
		hashless := step(plan, "---\na: 2\n---\nText", false)
		hashless.Content.BodyHash = nil
		var problem *shared.Error
		if err := guard.GuardWrite(f.asAlice(), hashless); err == nil || errors.As(err, &problem) || len(nodes.contents) != 0 {
			t.Errorf("a write of Plan without its body's hash = %v, read %v; want a defect's error, reading nothing", err,
				nodes.contents)
		}
		failure := errors.New("the database is gone")
		nodes.err = failure
		if err := guard.GuardWrite(f.asAlice(), step(plan, "---\na: 2\n---\nText", false)); !errors.Is(err, failure) {
			t.Errorf("a write of Plan, the reader failing = %v, want its error", err)
		}
	})
	t.Run("the vetoer", func(t *testing.T) {
		f := newFixture()
		guard := app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{}, frontmatters{})
		for _, state := range []domain.UpgradeState{domain.UpgradeUpgrading, domain.UpgradeReviewing, domain.UpgradeRollingBack} {
			o := app.SessionOpening{Write: app.Write{NotebookID: f.eng, By: f.alice, Client: domain.ClientWeb,
				Notebook: f.upgrading(state, current)}, PageID: page.NodeID}
			if err := guard.VetoEditSession(context.Background(), o); codeOf(err) != "notebook.upgrading" {
				t.Errorf("the initiator's opening while %s = %v, want notebook.upgrading", state, err)
			}
		}
		o := app.SessionOpening{Write: app.Write{NotebookID: f.eng, By: f.alice, Client: domain.ClientWeb,
			Notebook: domain.NotebookState{Mode: domain.ModeStandard}}, PageID: page.NodeID}
		if err := guard.VetoEditSession(context.Background(), o); err != nil {
			t.Errorf("an opening with no upgrade = %v, want nil", err)
		}
	})
}

// A unit of the upgrade its notebook has now is the upgrade's own (M11/P1
// design 3.4): whoever runs it, from the web, it writes in any state, and
// its changeset records the upgrade; a unit of an upgrade that is not the
// notebook's, or of one when the notebook has none, is stale, refused
// before it writes anything: no node, no changeset. A unit of no upgrade,
// from the web, is notebook.upgrading.
func TestTheFreezeLetsItsOwnUnits(t *testing.T) {
	current := uuid.NewV7()
	unit := func(f *fixture, own uuid.UUID) error {
		_, err := f.writer().Run(f.asAlice(), app.UnitSpec{NotebookID: f.eng, Action: domain.ActionCreate, Tree: true,
			Client: domain.ClientWeb, NotFound: domain.ErrNotebookNotFound, UpgradeOwn: own}, create(fmt.Sprintf("Seeded %d", len(f.store.nodes))))
		return err
	}
	for _, state := range []domain.UpgradeState{domain.UpgradeUpgrading, domain.UpgradeRollingBack} {
		f := newFixture()
		f.grant(domain.ActionCreate)
		f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: f.upgrading(state, current)}
		f.guards = []app.WriteGuard{app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{}, frontmatters{})}
		if err := unit(f, current); err != nil || len(f.store.nodes) != 1 || len(f.store.changesets) != 1 ||
			f.store.changesets[0].UpgradeID != current {
			t.Errorf("%s: the upgrade's own unit = %v, %d nodes, changesets %+v; want a page, its changeset of the upgrade",
				state, err, len(f.store.nodes), f.store.changesets)
		}
		if err := unit(f, uuid.UUID{}); codeOf(err) != "notebook.upgrading" || len(f.store.nodes) != 1 {
			t.Errorf("%s: a unit of no upgrade = %v, %d nodes; want notebook.upgrading, nothing more", state, err, len(f.store.nodes))
		}
	}
	for _, tt := range []struct {
		name  string
		state domain.NotebookState
	}{
		{"another upgrade's", domain.NotebookState{Mode: domain.ModeStandard, Upgrade: domain.NotebookUpgrade{ID: uuid.NewV7(),
			State: domain.UpgradeRollingBack}}},
		{"one the notebook no longer has", domain.NotebookState{Mode: domain.ModeStandard}},
	} {
		f := newFixture()
		f.grant(domain.ActionCreate)
		f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: tt.state}
		f.guards = []app.WriteGuard{app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{}, frontmatters{})}
		if err := unit(f, current); !errors.Is(err, domain.ErrStaleUpgradeWrite) || len(f.store.nodes) != 0 || len(f.store.changesets) != 0 {
			t.Errorf("a unit of %s = %v, %d nodes, %d changesets; want ErrStaleUpgradeWrite, nothing written", tt.name, err,
				len(f.store.nodes), len(f.store.changesets))
		}
	}
}

// A changeset a unit inserts records the upgrade its notebook has (M11/P1
// design 3.4): the initiator's MCP write while upgrading, past the guard;
// a unit's of a notebook with none records none. With no guard, as the
// notebook's freeze might be unregistered, a web write in an upgrading
// notebook would record it too: the record follows the notebook's
// upgrade, the guards decide who writes.
func TestTheChangesetsCarryTheirUpgrade(t *testing.T) {
	current := uuid.NewV7()
	for _, tt := range []struct {
		name   string
		state  domain.NotebookState
		client domain.Client
		guard  bool
		want   uuid.UUID
	}{
		{"the initiator's MCP write", domain.NotebookState{Mode: domain.ModeStandard, Upgrade: domain.NotebookUpgrade{ID: current,
			State: domain.UpgradeUpgrading}}, domain.MCPClient("claude-code"), true, current},
		{"a notebook with no upgrade", domain.NotebookState{Mode: domain.ModeStandard}, domain.ClientWeb, true, uuid.UUID{}},
		{"an unguarded web write", domain.NotebookState{Mode: domain.ModeStandard, Upgrade: domain.NotebookUpgrade{ID: current,
			State: domain.UpgradeReviewing}}, domain.ClientWeb, false, current},
	} {
		f := newFixture()
		f.grant(domain.ActionCreate, changeMode)
		tt.state.Upgrade.By = f.alice
		f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: tt.state}
		if tt.guard {
			f.guards = []app.WriteGuard{app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{}, frontmatters{})}
		}
		_, err := f.writer().Run(f.asAlice(), app.UnitSpec{NotebookID: f.eng, Action: domain.ActionCreate, Tree: true, Client: tt.client,
			NotFound: domain.ErrNotebookNotFound}, create("Notes"))
		if err != nil || len(f.store.changesets) != 1 || f.store.changesets[0].UpgradeID != tt.want {
			t.Errorf("%s = %v, changesets %+v; want one of the upgrade %s", tt.name, err, f.store.changesets, tt.want)
		}
	}
}

// The prechecks of the REST starts that write (M11/P1 design 3.4): an
// upload's, a background restore's and a background revert's answer
// notebook.upgrading right after their decision, a reader still
// forbidden, before any check that would answer otherwise (a name
// invalid, a page not in the trash, a changeset not found), reading
// nothing after the state; the revert admits nothing.
func TestThePrechecksRefuseAnUpgradingNotebook(t *testing.T) {
	for _, tt := range []struct {
		name   string
		action shared.Action
		check  func(f *fixture) error
	}{
		{"an upload's, of a name invalid", upload, func(f *fixture) error {
			return f.checkAsset(f.newAsset("index.md", nil))
		}},
		{"a restore's, of a page not in the trash", domain.ActionRestore, func(f *fixture) error {
			box := f.page("Box", nil, 0)
			_, err := f.jobs(now(), 0, 0).CheckRestore(f.asAlice(), f.eng, box.ID, nil)
			return err
		}},
		{"a revert's, of a changeset not found", domain.ActionRevert, func(f *fixture) error {
			_, err := f.jobs(now(), 1, 0).CheckRevert(f.asAlice(), f.eng, uuid.NewV7(), nil, func(context.Context) error {
				t.Error("a revert of a notebook being upgraded admitted")
				return nil
			}, allJobBatches)
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(tt.action)
			f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: f.upgrading(domain.UpgradeReviewing, uuid.NewV7())}
			if err := tt.check(f); codeOf(err) != "notebook.upgrading" {
				t.Errorf("Check() = %v, want notebook.upgrading", err)
			}
			if calls := f.rec.calls; len(calls) == 0 || calls[len(calls)-1] != "StateOf" {
				t.Errorf("calls = %v, want none after the state's read", calls)
			}
			f.auth.grants[tt.action], f.auth.forbidden[tt.action] = false, true
			if err := tt.check(f); codeOf(err) != shared.CodeForbidden {
				t.Errorf("a reader's Check() = %v, want forbidden", err)
			}
		})
	}
}
