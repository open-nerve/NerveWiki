package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Two registrants of each extension point (M2 design 8): the dispatch
// reaches both, in the order registered, and the first error, a refusal or
// a failure, stops it and is the use case's, for its transaction to roll
// back what came before. The module's extension tests show the rollback on
// a real database, with one registrant; these show the loops.

// registrant vetoes membership ends and follows every extension point,
// named in the store's calls; it refuses with refusal, and fails what it
// follows with failure.
type registrant struct {
	name             string
	store            *fakeStore
	refusal, failure error
}

func (r registrant) VetoMembershipEnd(ctx context.Context, _ app.MembershipEnd) error {
	r.store.record(ctx, "veto "+r.name)
	return r.refusal
}

func (r registrant) MembershipEnded(ctx context.Context, _ app.MembershipEnd) error {
	return r.follow(ctx, "ended")
}

func (r registrant) MembershipRestored(ctx context.Context, _ app.MembershipRestore) error {
	return r.follow(ctx, "restored")
}

func (r registrant) WorkspaceDeleted(ctx context.Context, _ app.WorkspaceDeletion) error {
	return r.follow(ctx, "deleted")
}

func (r registrant) follow(ctx context.Context, what string) error {
	r.store.record(ctx, what+" "+r.name)
	return r.failure
}

func TestTwoRegistrantsOfEachExtensionPoint(t *testing.T) {
	for _, point := range []struct {
		name     string
		run      func(tm *team, a, b registrant) error
		vetoes   bool
		followed string // what the subscribers follow
		write    string // the write the vetoers precede
	}{
		{"a membership's end", func(tm *team, a, b registrant) error {
			ender := app.MembershipEnder{Members: tm.store, Invitations: tm.store, Profiles: tm.profiles,
				Vetoers: []app.MembershipEndVetoer{a, b}, Subscribers: []app.MembershipEndSubscriber{a, b}}
			return app.NewRemoveMember(app.RemoveMemberDeps{Locker: tm.store, Finder: tm.store, Ender: ender,
				Auth: tm.auth, Tx: tm.tx, Clock: tm.clock, Logger: tm.logger()}).Execute(tm.as(tm.alice), tm.bob.ID)
		}, true, "ended", "EndMemberships"},
		{"a membership's restore", func(tm *team, a, b registrant) error {
			delete(tm.store.active, tm.bob.ID)
			tm.store.ended[tm.bob.ID] = tm.bob
			_, err := app.NewReactivateMember(app.ReactivateMemberDeps{Accounts: tm.accounts, Locker: tm.store, Members: tm.store,
				Updater: tm.store, Subscribers: []app.MembershipRestoreSubscriber{a, b}, Tx: tm.tx, Clock: tm.clock,
				Logger: tm.logger()}).Execute(as(tm.alice.UserID), "acme", "bob@corp.com")
			return err
		}, false, "restored", ""},
		{"a workspace's deletion", func(tm *team, a, b registrant) error {
			return app.NewDeleteWorkspace(app.DeleteWorkspaceDeps{Locker: tm.store, Workspaces: tm.store, Members: tm.store,
				Invitations: tm.store, Subscribers: []app.WorkspaceDeletionSubscriber{a, b}, Auth: tm.auth, Tx: tm.tx, Clock: tm.clock,
				Logger: tm.logger()}).Execute(tm.as(tm.alice), "acme")
		}, false, "deleted", ""},
	} {
		refusal := shared.NewError(shared.KindConflict, "test.vetoed", "Vetoed.")
		failure := errors.New("the second subscriber failed")
		follow := inTxCalls(point.followed+" a", point.followed+" b")
		vetoes := inTxCalls("veto a", "veto b")
		if !point.vetoes {
			vetoes = nil
		}
		for _, tt := range []struct {
			name             string
			refusal, failure error
			want             []string
		}{
			{"both follow", nil, nil, slices.Concat(vetoes, follow)},
			{"the second fails", nil, failure, slices.Concat(vetoes, follow)},
			{"the second refuses", refusal, nil, vetoes},
		} {
			if tt.refusal != nil && !point.vetoes {
				continue
			}
			t.Run(point.name+", "+tt.name, func(t *testing.T) {
				tm := newTeam()
				a, b := registrant{name: "a", store: tm.store}, registrant{name: "b", store: tm.store, refusal: tt.refusal, failure: tt.failure}

				err := point.run(tm, a, b)

				got := slices.DeleteFunc(slices.Clone(tm.store.calls), func(c string) bool {
					return !strings.HasSuffix(c, " a in tx") && !strings.HasSuffix(c, " b in tx")
				})
				wantErr := tt.refusal
				if wantErr == nil {
					wantErr = tt.failure
				}
				if !errors.Is(err, wantErr) || !slices.Equal(got, tt.want) {
					t.Errorf("Execute() = %v, the registrants did %q; want %v, %q", err, got, wantErr, tt.want)
				}
				if point.vetoes {
					wrote := slices.ContainsFunc(tm.store.calls, func(c string) bool { return strings.HasPrefix(c, point.write) })
					if wrote == (tt.refusal != nil) {
						t.Errorf("calls = %q; want %s only when no vetoer refused", tm.store.calls, point.write)
					}
				}
			})
		}
	}
}
