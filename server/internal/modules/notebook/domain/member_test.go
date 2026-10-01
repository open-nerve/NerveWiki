package domain

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestCheckRole(t *testing.T) {
	for _, r := range shared.NotebookRoles() {
		if got, err := CheckRole(string(r)); err != nil || got != r {
			t.Errorf("CheckRole(%q) = %q, %v", r, got, err)
		}
	}
	for _, r := range []string{"", "owner", "Admin", "member"} {
		if got := fields(t, func() error { _, err := CheckRole(r); return err }()); !slices.Equal(got, []string{"role invalid_format"}) {
			t.Errorf("CheckRole(%q) problems = %q", r, got)
		}
	}
}

// A new member: every problem at once; one whose membership ended may come
// back, one still active is a duplicate; outside the workspace, whether it
// was ever a member does not matter.
func TestCheckAddition(t *testing.T) {
	ended := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	active, left := &Member{Role: shared.NotebookReader}, &Member{Role: shared.NotebookAdmin, EndedAt: &ended}
	for _, tt := range []struct {
		name        string
		role        string
		inWorkspace bool
		existing    *Member
		problems    []string
	}{
		{"never a member", "editor", true, nil, nil},
		{"a member once", "reader", true, left, nil},
		{"a member", "editor", true, active, []string{"user_id duplicate"}},
		{"outside the workspace", "reader", false, nil, []string{"user_id not_allowed"}},
		{"outside the workspace, a member once", "reader", false, left, []string{"user_id not_allowed"}},
		{"a member, a role unknown", "owner", true, active, []string{"user_id duplicate", "role invalid_format"}},
		{"outside, a role unknown", "", false, nil, []string{"user_id not_allowed", "role invalid_format"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckAddition(tt.role, tt.inWorkspace, tt.existing)
			if tt.problems == nil {
				if err != nil || got != shared.NotebookRole(tt.role) {
					t.Errorf("CheckAddition() = %q, %v; want %q", got, err, tt.role)
				}
				return
			}
			if problems := fields(t, err); !slices.Equal(problems, tt.problems) {
				t.Errorf("problems = %q, want %q", problems, tt.problems)
			}
		})
	}
}

// Rule one: the only active admin cannot leave, even alone; another admin,
// or any other role, can.
func TestCheckLeave(t *testing.T) {
	for _, tt := range []struct {
		role   shared.NotebookRole
		admins int
		want   error
	}{
		{shared.NotebookAdmin, 1, ErrSoleAdmin},
		{shared.NotebookAdmin, 2, nil},
		{shared.NotebookEditor, 1, nil},
		{shared.NotebookReader, 1, nil},
	} {
		if err := CheckLeave(Member{Role: tt.role}, tt.admins); !errors.Is(err, tt.want) {
			t.Errorf("CheckLeave(%s of %d admins) = %v, want %v", tt.role, tt.admins, err, tt.want)
		}
	}
}
