package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
// one, of any upgrade; reads the reads made, each its upgrade's id and the
// ids asked; err fails each read.
type upgradeNodes struct {
	protected map[uuid.UUID]bool
	reads     [][]uuid.UUID
	err       error
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
		guard := app.NewFreezeGuard(f.auth, changeMode, nodes)
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
		guard := app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{})
		err := guard.GuardWrite(f.asAlice(), f.freezeStep(f.upgrading(domain.UpgradeUpgrading, current), domain.ClientWeb, &app.Grants{},
			domain.OpCreate, page))
		if codeOf(err) != "notebook.upgrading" || asked(f) != 0 {
			t.Errorf("the initiator's web write = %v, asked %d; want notebook.upgrading, asking nothing", err, asked(f))
		}
	})
	t.Run("the initiator's MCP writes ask once a unit", func(t *testing.T) {
		f := newFixture()
		f.grant(changeMode)
		guard := app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{})
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
			guard := app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{})
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
		guard := app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{})
		err := guard.GuardWrite(f.asAlice(), f.freezeStep(f.upgrading(domain.UpgradeUpgrading, current), agent, &app.Grants{},
			domain.OpCreate, page))
		if !errors.Is(err, failure) {
			t.Errorf("the initiator's MCP write, the authorizer failing = %v, want its error", err)
		}
	})
	t.Run("a stale unit", func(t *testing.T) {
		f := newFixture()
		f.grant(changeMode)
		guard := app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{})
		s := f.freezeStep(f.upgrading(domain.UpgradeRollingBack, current), domain.ClientWeb, &app.Grants{}, domain.OpCreate, page)
		s.UpgradeOwn = uuid.NewV7()
		err := guard.GuardWrite(f.asAlice(), s)
		var problem *shared.Error
		if !errors.Is(err, domain.ErrStaleUpgradeWrite) || errors.As(err, &problem) || len(f.rec.calls) != 0 {
			t.Errorf("a unit of another upgrade = %v after %v, want ErrStaleUpgradeWrite, no problem, asking nothing", err, f.rec.calls)
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
		guard := app.NewFreezeGuard(f.auth, changeMode, nodes)
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
		for _, op := range []domain.Operation{domain.OpMove, domain.OpRename, domain.OpContent, domain.OpCreate} {
			if err := guard.GuardWrite(f.asAlice(), f.freezeStep(n, agent, &app.Grants{}, op, subtree[1])); err != nil {
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
	t.Run("the vetoer", func(t *testing.T) {
		f := newFixture()
		guard := app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{})
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
		f.guards = []app.WriteGuard{app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{})}
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
		f.guards = []app.WriteGuard{app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{})}
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
			f.guards = []app.WriteGuard{app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{})}
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
