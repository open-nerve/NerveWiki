package domain

import (
	"errors"
	"maps"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Rule two counts, by workspace, the notebooks whose only active admin the
// account is while another active explicit member is left: not one with a
// second admin, nor one it is alone in, nor one it is no admin of.
func TestRuleTwo(t *testing.T) {
	acme, lab := uuid.NewV7(), uuid.NewV7()
	held := func(ws uuid.UUID, role shared.NotebookRole, admins, members int) Holding {
		return Holding{NotebookID: uuid.NewV7(), WorkspaceID: ws, Role: role, Admins: admins, Members: members}
	}
	for _, tt := range []struct {
		name     string
		holdings []Holding
		want     map[uuid.UUID]int
	}{
		{"none", nil, map[uuid.UUID]int{}},
		{"the only admin, with a member", []Holding{held(acme, shared.NotebookAdmin, 1, 2)}, map[uuid.UUID]int{acme: 1}},
		{"the only admin, alone", []Holding{held(acme, shared.NotebookAdmin, 1, 1)}, map[uuid.UUID]int{}},
		{"one of two admins", []Holding{held(acme, shared.NotebookAdmin, 2, 3)}, map[uuid.UUID]int{}},
		{"an editor beside the only admin", []Holding{held(acme, shared.NotebookEditor, 1, 2)}, map[uuid.UUID]int{}},
		{"by workspace", []Holding{
			held(acme, shared.NotebookAdmin, 1, 2), held(lab, shared.NotebookAdmin, 1, 5), held(acme, shared.NotebookAdmin, 1, 3),
			held(lab, shared.NotebookAdmin, 1, 1), held(lab, shared.NotebookReader, 1, 4),
		}, map[uuid.UUID]int{acme: 2, lab: 1}},
	} {
		if got := RuleTwo(tt.holdings); !maps.Equal(got, tt.want) {
			t.Errorf("%s: RuleTwo() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// A notebook becomes ownerless when its only active admin's membership
// ends, alone in it or not.
func TestHoldingSoleAdmin(t *testing.T) {
	for _, tt := range []struct {
		h    Holding
		want bool
	}{
		{Holding{Role: shared.NotebookAdmin, Admins: 1, Members: 1}, true},
		{Holding{Role: shared.NotebookAdmin, Admins: 1, Members: 4}, true},
		{Holding{Role: shared.NotebookAdmin, Admins: 2, Members: 2}, false},
		{Holding{Role: shared.NotebookEditor, Admins: 1, Members: 2}, false},
	} {
		if got := tt.h.SoleAdmin(); got != tt.want {
			t.Errorf("%+v.SoleAdmin() = %v, want %v", tt.h, got, tt.want)
		}
	}
}

// The refusal is notebook.sole_admin with the workspaces in slug order.
func TestErrSoleAdminOf(t *testing.T) {
	err := ErrSoleAdminOf(map[string]int{"lab": 1, "acme": 2})

	want := "The account is the only admin of notebooks with other members (2 in acme, 1 in lab): " +
		"another member must become their admin, or they must be deleted, first."
	if !errors.Is(err, ErrSoleAdmin) || err.ProblemStatus() != 409 || err.Detail != want {
		t.Errorf("ErrSoleAdminOf() = %d %s %q, want 409 notebook.sole_admin %q", err.ProblemStatus(), err.Code, err.Detail, want)
	}
}
