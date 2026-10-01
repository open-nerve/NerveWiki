package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fakeHoldings answers holdings, and records the cascade's writes.
type fakeHoldings struct {
	*recorder
	holdings []domain.Holding
}

func (f *fakeHoldings) LockHoldings(ctx context.Context, userID uuid.UUID, workspaceIDs []uuid.UUID) ([]domain.Holding, error) {
	f.record(ctx, fmt.Sprintf("LockHoldings %d workspaces", len(workspaceIDs)))
	return f.holdings, nil
}

func (f *fakeHoldings) EndMembershipsOf(ctx context.Context, userID uuid.UUID, notebookIDs []uuid.UUID, by uuid.UUID, at time.Time) error {
	f.record(ctx, fmt.Sprintf("EndMembershipsOf %v by %s at %s", notebookIDs, by, at.Format(time.RFC3339)))
	return nil
}

func (f *fakeHoldings) SetOwnerless(ctx context.Context, notebookIDs []uuid.UUID, formerOwner uuid.UUID, at time.Time) error {
	f.record(ctx, fmt.Sprintf("SetOwnerless %v of %s at %s", notebookIDs, formerOwner, at.Format(time.RFC3339)))
	return nil
}

// fakeSlugs names the workspaces of slugs.
type fakeSlugs struct {
	*recorder
	slugs map[uuid.UUID]string
}

func (f fakeSlugs) Slugs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	f.record(ctx, fmt.Sprintf("Slugs of %d", len(ids)))
	return f.slugs, nil
}

// fakeReturner answers ownerless, and records the returns; fakeAudit
// records the events.
type fakeReturner struct {
	*recorder
	ownerless []domain.Notebook
}

func (f *fakeReturner) LockOwnerlessOf(ctx context.Context, workspaceID, userID uuid.UUID) ([]domain.Notebook, error) {
	f.record(ctx, "LockOwnerlessOf")
	return f.ownerless, nil
}

func (f *fakeReturner) ReturnNotebooks(ctx context.Context, notebookIDs []uuid.UUID, userID, by uuid.UUID, at time.Time) error {
	f.record(ctx, fmt.Sprintf("ReturnNotebooks %v to %s by %s at %s", notebookIDs, userID, by, at.Format(time.RFC3339)))
	return nil
}

type fakeAudit struct {
	*recorder
	events []domain.AuditEvent
}

func (f *fakeAudit) AddAuditEvent(ctx context.Context, e domain.AuditEvent) error {
	f.record(ctx, "AddAuditEvent "+string(e.Action))
	f.events = append(f.events, e)
	return nil
}

// cascadeTeam is two workspaces, acme and lab, and alice's notebooks in
// them: soloNB, hers alone; teamNB, hers with others; sharedNB, with a
// second admin; editedNB, where she edits.
type cascadeTeam struct {
	rec                                *recorder
	acme, lab, alice, admin            uuid.UUID
	soloNB, teamNB, sharedNB, editedNB uuid.UUID
	holdings                           []domain.Holding
	visibility                         *fakeVisibility
}

func newCascadeTeam() cascadeTeam {
	tm := cascadeTeam{rec: &recorder{}, acme: uuid.NewV7(), lab: uuid.NewV7(), alice: uuid.NewV7(), admin: uuid.NewV7(),
		soloNB: uuid.NewV7(), teamNB: uuid.NewV7(), sharedNB: uuid.NewV7(), editedNB: uuid.NewV7()}
	tm.holdings = []domain.Holding{
		{NotebookID: tm.soloNB, WorkspaceID: tm.acme, Role: shared.NotebookAdmin, Admins: 1, Members: 1},
		{NotebookID: tm.teamNB, WorkspaceID: tm.lab, Role: shared.NotebookAdmin, Admins: 1, Members: 3},
		{NotebookID: tm.sharedNB, WorkspaceID: tm.acme, Role: shared.NotebookAdmin, Admins: 2, Members: 2},
		{NotebookID: tm.editedNB, WorkspaceID: tm.acme, Role: shared.NotebookEditor, Admins: 1, Members: 2},
	}
	tm.visibility = &fakeVisibility{recorder: tm.rec, name: "v"}
	return tm
}

func (tm cascadeTeam) end(holdings []domain.Holding) (app.MembershipEnd, *fakeHoldings) {
	h := &fakeHoldings{recorder: tm.rec, holdings: holdings}
	return app.MembershipEnd{
		Holdings: h, Workspaces: fakeSlugs{recorder: tm.rec, slugs: map[uuid.UUID]string{tm.acme: "acme", tm.lab: "lab"}},
		Subscribers: []app.VisibilitySubscriber{tm.visibility},
	}, h
}

// Rule two refuses an account that leaves itself while it is the only
// admin of a notebook with other members, counted by workspace slug; a
// removal is never refused, nor read.
func TestVetoMembershipEnd(t *testing.T) {
	tm := newCascadeTeam()
	ctx := context.Background()
	for _, tt := range []struct {
		name      string
		voluntary bool
		holdings  []domain.Holding
		detail    string // "" for none
		calls     []string
	}{
		{"a removal", false, tm.holdings, "", nil},
		{"leaving, alone or beside an admin", true, []domain.Holding{tm.holdings[0], tm.holdings[2], tm.holdings[3]}, "",
			[]string{"LockHoldings 2 workspaces"}},
		{"leaving the only admin of a notebook with members", true, tm.holdings,
			"The account is the only admin of notebooks with other members (1 in lab): another member must become their admin, or they must be deleted, first.",
			[]string{"LockHoldings 2 workspaces", "Slugs of 1"}},
	} {
		tm.rec.calls = nil
		end, _ := tm.end(tt.holdings)

		err := end.VetoMembershipEnd(ctx, app.WorkspaceMembershipEnd{UserID: tm.alice, WorkspaceIDs: []uuid.UUID{tm.acme, tm.lab},
			Voluntary: tt.voluntary, By: tm.alice, At: now()})

		var se *shared.Error
		refused := errors.As(err, &se) && errors.Is(err, domain.ErrSoleAdmin) && se.Detail == tt.detail
		if (tt.detail == "" && err != nil) || (tt.detail != "" && !refused) || !slices.Equal(tm.rec.calls, tt.calls) {
			t.Errorf("%s: VetoMembershipEnd() = %v, calls %q; want %q, calls %q", tt.name, err, tm.rec.calls, tt.detail, tt.calls)
		}
	}
}

// The end ends every notebook membership of the account in the workspaces,
// by the ender at the end's time; leaves ownerless those it was the only
// admin of, alone or not; and tells the visibility once per workspace,
// those without a notebook of its included.
func TestMembershipEnded(t *testing.T) {
	tm := newCascadeTeam()
	end, _ := tm.end(tm.holdings)
	at := now().Format(time.RFC3339)

	err := end.MembershipEnded(context.Background(), app.WorkspaceMembershipEnd{UserID: tm.alice,
		WorkspaceIDs: []uuid.UUID{tm.acme, tm.lab}, By: tm.admin, At: now()})

	want := []string{
		"LockHoldings 2 workspaces",
		fmt.Sprintf("EndMembershipsOf %v by %s at %s", []uuid.UUID{tm.soloNB, tm.teamNB, tm.sharedNB, tm.editedNB}, tm.admin, at),
		fmt.Sprintf("SetOwnerless %v of %s at %s", []uuid.UUID{tm.soloNB, tm.teamNB}, tm.alice, at),
		"VisibilityChanged v", "VisibilityChanged v",
	}
	if err != nil || !slices.Equal(tm.rec.calls, want) {
		t.Errorf("MembershipEnded() = %v, calls %q; want %q", err, tm.rec.calls, want)
	}
	told := []app.VisibilityChange{
		{WorkspaceID: tm.acme, UserIDs: []uuid.UUID{tm.alice}, At: now()}, {WorkspaceID: tm.lab, UserIDs: []uuid.UUID{tm.alice}, At: now()},
	}
	if !slices.EqualFunc(tm.visibility.got, told, sameChange) {
		t.Errorf("told %+v, want %+v", tm.visibility.got, told)
	}
}

// An account with no notebook writes nothing, and is told all the same:
// it saw the open notebooks of its workspace.
func TestMembershipEndedWithNoNotebook(t *testing.T) {
	tm := newCascadeTeam()
	end, _ := tm.end(nil)
	tm.visibility.err = errors.New("the subscriber failed")

	err := end.MembershipEnded(context.Background(), app.WorkspaceMembershipEnd{UserID: tm.alice, WorkspaceIDs: []uuid.UUID{tm.acme},
		By: tm.alice, At: now()})

	if !errors.Is(err, tm.visibility.err) || !slices.Equal(tm.rec.calls, []string{"LockHoldings 1 workspaces", "VisibilityChanged v"}) {
		t.Errorf("MembershipEnded() = %v, calls %q; want the subscriber's failure after the lock alone", err, tm.rec.calls)
	}
}

// A restore returns the notebooks the account left ownerless, recorded as
// returned by it, whatever its role; it tells the visibility when the
// account sees a notebook again: one returned, or the open ones as an
// admin or a member.
func TestMembershipRestored(t *testing.T) {
	tm := newCascadeTeam()
	plans := domain.Notebook{ID: uuid.NewV7(), WorkspaceID: tm.acme, Name: "Plans"}
	notes := domain.Notebook{ID: uuid.NewV7(), WorkspaceID: tm.acme, Name: "Notes"}
	at := now().Format(time.RFC3339)
	for _, tt := range []struct {
		name      string
		role      shared.WorkspaceRole
		ownerless []domain.Notebook
		calls     []string
	}{
		{"two, as a guest", shared.WorkspaceGuest, []domain.Notebook{plans, notes}, []string{
			"LockOwnerlessOf", fmt.Sprintf("ReturnNotebooks %v to %s by %s at %s", []uuid.UUID{plans.ID, notes.ID}, tm.alice, tm.alice, at),
			"AddAuditEvent returned", "AddAuditEvent returned", "VisibilityChanged v",
		}},
		{"none, as a member", shared.WorkspaceMember, nil, []string{"LockOwnerlessOf", "VisibilityChanged v"}},
		{"none, as a guest", shared.WorkspaceGuest, nil, []string{"LockOwnerlessOf"}},
	} {
		tm.rec.calls, tm.visibility.got = nil, nil
		audit := &fakeAudit{recorder: tm.rec}
		restore := app.MembershipRestore{Returner: &fakeReturner{recorder: tm.rec, ownerless: tt.ownerless}, Audit: audit,
			Subscribers: []app.VisibilitySubscriber{tm.visibility}}

		n, err := restore.MembershipRestored(context.Background(), app.WorkspaceMembershipRestore{WorkspaceID: tm.acme, UserID: tm.alice,
			Role: tt.role, By: tm.alice, At: now()})

		if err != nil || n != len(tt.ownerless) || !slices.Equal(tm.rec.calls, tt.calls) {
			t.Errorf("%s: MembershipRestored() = %d, %v, calls %q; want %d, calls %q", tt.name, n, err, tm.rec.calls, len(tt.ownerless), tt.calls)
		}
		var events []domain.AuditEvent
		for _, nb := range tt.ownerless {
			events = append(events, domain.AuditEvent{WorkspaceID: tm.acme, NotebookID: nb.ID, NotebookName: nb.Name,
				Action: domain.AuditReturned, FormerOwnerID: tm.alice, ActorID: tm.alice, At: now()})
		}
		if !slices.EqualFunc(audit.events, events, func(a, b domain.AuditEvent) bool {
			b.ID = a.ID
			return a.ID != uuid.UUID{} && a == b
		}) {
			t.Errorf("%s: recorded %+v, want %+v", tt.name, audit.events, events)
		}
		if len(tm.visibility.got) == 1 && !sameChange(tm.visibility.got[0], app.VisibilityChange{WorkspaceID: tm.acme,
			UserIDs: []uuid.UUID{tm.alice}, At: now()}) {
			t.Errorf("%s: told %+v, want alice in acme", tt.name, tm.visibility.got)
		}
	}
}
