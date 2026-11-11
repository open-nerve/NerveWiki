package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// upgradePlaybookRows are the upgrade's rows as the prompt reads them:
// Current alone; any other method is nil's, a panic.
type upgradePlaybookRows struct {
	UpgradeRows
	current func(context.Context, uuid.UUID) (domain.Upgrade, bool, error)
}

func (r upgradePlaybookRows) Current(ctx context.Context, notebook uuid.UUID) (domain.Upgrade, bool, error) {
	return r.current(ctx, notebook)
}

// upgradePlaybookOf is the use case over ports that record their calls,
// the notebook found as found says, the authorizer answering auth, the
// upgrade read as up, ok and failure say. Its notebook is lintExecID(2),
// in the workspace lintExecID(1).
func upgradePlaybookOf(calls *playbookCalls, found bool, auth error, up domain.Upgrade, ok bool, failure error) *UpgradePlaybook {
	workspace := lintExecID(1)
	return NewUpgradePlaybook(lintExecNotebooks{
		workspace: func(_ context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
			*calls = append(*calls, "workspace "+id.String())
			return workspace, found, nil
		},
		state: func(context.Context, uuid.UUID) (NotebookState, bool, error) {
			*calls = append(*calls, "state")
			return NotebookState{Mode: domain.ModeStandard}, true, nil
		},
	}, lintExecAuth(func(_ context.Context, actor shared.Actor, action shared.Action, target shared.Target) (shared.Grant, error) {
		*calls = append(*calls, "authorize "+string(action))
		if actor.UserID != lintExecID(3) || target.WorkspaceID != workspace || target.NotebookID != lintExecID(2) {
			return shared.Grant{}, fmt.Errorf("the actor %+v of the target %+v", actor, target)
		}
		return shared.Grant{}, auth
	}), upgradePlaybookRows{current: func(_ context.Context, id uuid.UUID) (domain.Upgrade, bool, error) {
		*calls = append(*calls, "current "+id.String())
		return up, ok, failure
	}})
}

// The upgrade playbook is the reader's of a notebook upgrading alone
// (M11/P3 design 3.16): lint.read decided as the other prompts decide it,
// one who cannot see the notebook told it is not found; then the upgrade
// under way, read once, no mode read; none, one in review or rolling back
// is wiki.upgrade_state; an upgrading one answers the text. Each refusal
// comes before the next port is asked.
func TestUpgradePlaybookAuthorizesAndRequiresUpgrading(t *testing.T) {
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: lintExecID(3), SessionID: lintExecID(4)})
	failure := errors.New("the database is gone")
	nb := lintExecID(2).String()
	read := playbookCalls{"workspace " + nb, "authorize lint.read", "current " + nb}
	upgrade := func(state domain.UpgradeState) domain.Upgrade {
		return domain.Upgrade{ID: lintExecID(5), NotebookID: lintExecID(2), State: state}
	}
	for _, tt := range []struct {
		name    string
		ctx     context.Context
		found   bool
		auth    error
		up      domain.Upgrade
		ok      bool
		failure error
		want    error
		calls   playbookCalls
	}{
		{name: "no actor", ctx: context.Background(), found: true, ok: true, up: upgrade(domain.UpgradeUpgrading), want: shared.Unauthenticated()},
		{name: "no notebook", ctx: ctx, ok: true, up: upgrade(domain.UpgradeUpgrading), want: domain.ErrNotebookNotFound,
			calls: playbookCalls{"workspace " + nb}},
		{name: "not visible", ctx: ctx, found: true, auth: fmt.Errorf("wrapped: %w", shared.ErrNotVisible), ok: true,
			up: upgrade(domain.UpgradeUpgrading), want: domain.ErrNotebookNotFound, calls: read[:2]},
		{name: "forbidden", ctx: ctx, found: true, auth: shared.Forbidden(), ok: true, up: upgrade(domain.UpgradeUpgrading),
			want: shared.Forbidden(), calls: read[:2]},
		{name: "an authorizer that fails", ctx: ctx, found: true, auth: failure, ok: true, up: upgrade(domain.UpgradeUpgrading),
			want: failure, calls: read[:2]},
		{name: "a read of the upgrade that fails", ctx: ctx, found: true, ok: true, up: upgrade(domain.UpgradeUpgrading),
			failure: failure, want: failure, calls: read},
		{name: "no upgrade", ctx: ctx, found: true, want: domain.ErrUpgradeState, calls: read},
		{name: "reviewing", ctx: ctx, found: true, ok: true, up: upgrade(domain.UpgradeReviewing), want: domain.ErrUpgradeState, calls: read},
		{name: "rolling back", ctx: ctx, found: true, ok: true, up: upgrade(domain.UpgradeRollingBack), want: domain.ErrUpgradeState, calls: read},
		{name: "an upgrade of the row's another state", ctx: ctx, found: true, ok: true, up: upgrade(domain.UpgradeApproved),
			want: domain.ErrUpgradeState, calls: read},
		{name: "upgrading", ctx: ctx, found: true, ok: true, up: upgrade(domain.UpgradeUpgrading), calls: read},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls playbookCalls
			text, err := upgradePlaybookOf(&calls, tt.found, tt.auth, tt.up, tt.ok, tt.failure).Execute(tt.ctx, lintExecID(2))
			if !samePlaybookError(err, tt.want) {
				t.Errorf("Execute() = %v, want %v", err, tt.want)
			}
			if !reflect.DeepEqual(calls, tt.calls) {
				t.Errorf("the calls = %v, want %v", calls, tt.calls)
			}
			want := ""
			if tt.want == nil {
				want = domain.UpgradePlaybookText()
			}
			if text != want {
				t.Errorf("Execute() text = %.60q with the error %v, want %.60q", text, err, want)
			}
			// wiki.upgrade_state's plain detail: never the submission's
			// "still being checked", which the same code has.
			if errors.Is(tt.want, domain.ErrUpgradeState) && (err == nil || err.Error() != domain.ErrUpgradeState.Error()) {
				t.Errorf("Execute() = %v, want the detail %q", err, domain.ErrUpgradeState.Error())
			}
		})
	}

	t.Run("a workspace read that fails", func(t *testing.T) {
		p := NewUpgradePlaybook(lintExecNotebooks{
			workspace: func(context.Context, uuid.UUID) (uuid.UUID, bool, error) { return uuid.UUID{}, false, failure },
		}, lintExecAuth(func(context.Context, shared.Actor, shared.Action, shared.Target) (shared.Grant, error) {
			return shared.Grant{}, nil
		}), upgradePlaybookRows{})
		if _, err := p.Execute(ctx, lintExecID(2)); !errors.Is(err, failure) {
			t.Errorf("Execute() = %v, want its cause", err)
		}
	})
}
