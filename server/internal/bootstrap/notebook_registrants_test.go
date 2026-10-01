package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The notebook module's part in a workspace's deletion reaches it through
// serve (M2 handoff to M3, item 1; v0.1 design 13.1, item 21): the
// composition check proves only that serve reaches the registrants; this
// proves they are handed over. The other paths of the membership's end and
// restore come with their registrants (M3/P3).
func TestDeletingAWorkspaceDeletesItsNotebooks(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	var ids []string
	for _, name := range []string{"alice", "bob"} {
		status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces/acme/notebooks", tm.tokens[name],
			`{"name":"Notes of `+name+`"}`)
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(answer), &created); status != http.StatusCreated || err != nil {
			t.Fatalf("create %s's notebook = %d %s", name, status, answer)
		}
		ids = append(ids, created.ID)
	}

	if status, answer := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/workspaces/acme", tm.tokens["alice"], ""); status != http.StatusNoContent {
		t.Fatalf("DELETE acme = %d %s", status, answer)
	}

	// The notebooks and their members, deleted with acme's time.
	if n := count(t, tm.pool, `SELECT count(*) FROM notebooks n JOIN workspaces w ON w.id = n.workspace_id
		WHERE w.slug = 'acme' AND n.deleted_at = w.deleted_at AND n.updated_by_id = w.updated_by_id`); n != 2 {
		t.Errorf("%d of acme's notebooks deleted with it, want both", n)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM notebook_members m JOIN notebooks n ON n.id = m.notebook_id
		JOIN workspaces w ON w.id = n.workspace_id WHERE w.slug = 'acme' AND m.deleted_at = w.deleted_at`); n != 2 {
		t.Errorf("%d of the notebooks' members deleted with acme, want both admins", n)
	}
	for _, id := range ids {
		if status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+id, tm.tokens["alice"], ""); status != http.StatusNotFound ||
			problemCode(t, answer) != "notebook.not_found" {
			t.Errorf("GET a notebook of the deleted acme = %d %s, want 404 notebook.not_found", status, answer)
		}
	}
}

// visibilityRecorder records the visibility changes it is told.
type visibilityRecorder struct{ got []notebook.VisibilityChange }

func (r *visibilityRecorder) VisibilityChanged(_ context.Context, v notebook.VisibilityChange) error {
	r.got = append(r.got, v)
	return nil
}

// The workspace module's addition and role change reach the notebook
// module's part, which tells the notebook module's visibility subscribers
// (M3/P2 design 3.6): the values converted field by field, and only a
// change of a default role told. M3 has no visibility subscriber, so the
// test hands its own to workspaceRegistrantsWith; workspaceRegistrants
// hands notebookRegistrants'.
func TestTheWorkspaceMemberEventsReachTheVisibility(t *testing.T) {
	r := &visibilityRecorder{}
	ext := workspaceRegistrantsWith(nil, notebookExtensions{visibilitySubscribers: []notebook.VisibilitySubscriber{r}})
	ctx := context.Background()
	acme, bob, alice := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	at := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	if len(ext.additionSubscribers) != 1 || len(ext.roleChangeSubscribers) != 1 {
		t.Fatalf("subscribers: %d of the addition, %d of the role change; want the notebook module's for each",
			len(ext.additionSubscribers), len(ext.roleChangeSubscribers))
	}

	for _, err := range []error{
		ext.additionSubscribers[0].MembershipAdded(ctx, workspace.MembershipAddition{WorkspaceID: acme, UserID: bob, Role: shared.WorkspaceMember, By: bob, At: at}),
		ext.additionSubscribers[0].MembershipAdded(ctx, workspace.MembershipAddition{WorkspaceID: acme, UserID: alice, Role: shared.WorkspaceGuest, By: alice, At: at}),
		ext.roleChangeSubscribers[0].MemberRoleChanged(ctx, workspace.MemberRoleChange{WorkspaceID: acme, UserID: bob,
			From: shared.WorkspaceMember, To: shared.WorkspaceGuest, By: alice, At: at.Add(time.Minute)}),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}

	want := []notebook.VisibilityChange{
		{WorkspaceID: acme, UserIDs: []uuid.UUID{bob}, At: at},
		{WorkspaceID: acme, UserIDs: []uuid.UUID{bob}, At: at.Add(time.Minute)},
	}
	if !slices.EqualFunc(r.got, want, func(a, b notebook.VisibilityChange) bool {
		return a.WorkspaceID == b.WorkspaceID && slices.Equal(a.UserIDs, b.UserIDs) && a.Reached == b.Reached && a.At.Equal(b.At)
	}) {
		t.Errorf("told %+v, want %+v", r.got, want)
	}
}
