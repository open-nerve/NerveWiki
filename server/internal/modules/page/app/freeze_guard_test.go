package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// changeMode is the notebook module's notebook.change_mode, which
// bootstrap hands the freeze guard.
const changeMode = shared.Action("notebook.change_mode")

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
// by asking nothing; the initiator's web write while upgrading it refuses
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
		guard := app.NewFreezeGuard(f.auth, changeMode)
		err := guard.GuardWrite(f.asAlice(), f.freezeStep(domain.NotebookState{Mode: domain.ModeStandard}, domain.ClientWeb, &app.Grants{},
			domain.OpCreate, page))
		if err != nil || len(f.rec.calls) != 0 {
			t.Errorf("a write of a notebook with no upgrade = %v after %v, want nil asking nothing", err, f.rec.calls)
		}
	})
	t.Run("the initiator's web write", func(t *testing.T) {
		f := newFixture()
		f.grant(changeMode)
		guard := app.NewFreezeGuard(f.auth, changeMode)
		err := guard.GuardWrite(f.asAlice(), f.freezeStep(f.upgrading(domain.UpgradeUpgrading, current), domain.ClientWeb, &app.Grants{},
			domain.OpCreate, page))
		if codeOf(err) != "notebook.upgrading" || asked(f) != 0 {
			t.Errorf("the initiator's web write = %v, asked %d; want notebook.upgrading, asking nothing", err, asked(f))
		}
	})
	t.Run("the initiator's MCP writes ask once a unit", func(t *testing.T) {
		f := newFixture()
		f.grant(changeMode)
		guard := app.NewFreezeGuard(f.auth, changeMode)
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
			guard := app.NewFreezeGuard(f.auth, changeMode)
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
		guard := app.NewFreezeGuard(f.auth, changeMode)
		err := guard.GuardWrite(f.asAlice(), f.freezeStep(f.upgrading(domain.UpgradeUpgrading, current), agent, &app.Grants{},
			domain.OpCreate, page))
		if !errors.Is(err, failure) {
			t.Errorf("the initiator's MCP write, the authorizer failing = %v, want its error", err)
		}
	})
	t.Run("a stale unit", func(t *testing.T) {
		f := newFixture()
		f.grant(changeMode)
		guard := app.NewFreezeGuard(f.auth, changeMode)
		s := f.freezeStep(f.upgrading(domain.UpgradeRollingBack, current), domain.ClientWeb, &app.Grants{}, domain.OpCreate, page)
		s.UpgradeOwn = uuid.NewV7()
		err := guard.GuardWrite(f.asAlice(), s)
		var problem *shared.Error
		if !errors.Is(err, domain.ErrStaleUpgradeWrite) || errors.As(err, &problem) || len(f.rec.calls) != 0 {
			t.Errorf("a unit of another upgrade = %v after %v, want ErrStaleUpgradeWrite, no problem, asking nothing", err, f.rec.calls)
		}
	})
	t.Run("the vetoer", func(t *testing.T) {
		f := newFixture()
		guard := app.NewFreezeGuard(f.auth, changeMode)
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
		f.guards = []app.WriteGuard{app.NewFreezeGuard(f.auth, changeMode)}
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
		f.guards = []app.WriteGuard{app.NewFreezeGuard(f.auth, changeMode)}
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
			f.guards = []app.WriteGuard{app.NewFreezeGuard(f.auth, changeMode)}
		}
		_, err := f.writer().Run(f.asAlice(), app.UnitSpec{NotebookID: f.eng, Action: domain.ActionCreate, Tree: true, Client: tt.client,
			NotFound: domain.ErrNotebookNotFound}, create("Notes"))
		if err != nil || len(f.store.changesets) != 1 || f.store.changesets[0].UpgradeID != tt.want {
			t.Errorf("%s = %v, changesets %+v; want one of the upgrade %s", tt.name, err, f.store.changesets, tt.want)
		}
	}
}
