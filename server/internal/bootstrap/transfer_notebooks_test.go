package bootstrap

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// transferNotebooks converts the notebook module's state into the transfer
// module's (M11/P1 design 3.5), unlocked and under the row's FOR SHARE,
// each through the notebook module's read of its own: a notebook with no
// upgrade has the zero id; one upgrading its upgrade's id; one rolling
// back that id too, and RollingBack; a notebook not found is not found,
// with the zero state; an error is answered as it is.
func TestTransferNotebooksConvertTheState(t *testing.T) {
	plain, upgrading, rolling, gone := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	first, second, initiator := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	var calls []string
	n := transferNotebooks{states{of: map[uuid.UUID]notebook.State{
		plain:     {Mode: notebook.ModeStandard},
		upgrading: {Mode: notebook.ModeStandard, Upgrade: notebook.Upgrade{ID: first, State: notebook.UpgradeReviewing, By: initiator, At: time.Now()}},
		rolling:   {Mode: notebook.ModeStandard, Upgrade: notebook.Upgrade{ID: second, State: notebook.UpgradeRollingBack, By: initiator, At: time.Now()}},
	}, calls: &calls}}
	failure := errors.New("the database is gone")
	failing := transferNotebooks{states{err: failure, calls: &calls}}
	reads := []struct {
		name, call string
		of         func(transferNotebooks) func(context.Context, uuid.UUID) (transfer.NotebookState, bool, error)
	}{
		{"StateOf", "StateOf", func(n transferNotebooks) func(context.Context, uuid.UUID) (transfer.NotebookState, bool, error) {
			return n.StateOf
		}},
		{"ShareByID", "ShareState", func(n transferNotebooks) func(context.Context, uuid.UUID) (transfer.NotebookState, bool, error) {
			return n.ShareByID
		}},
	}
	for _, r := range reads {
		for _, tt := range []struct {
			name string
			id   uuid.UUID
			want transfer.NotebookState
			ok   bool
		}{
			{"a standard notebook", plain, transfer.NotebookState{}, true},
			{"an upgrading notebook", upgrading, transfer.NotebookState{UpgradeID: first}, true},
			{"a notebook rolling back", rolling, transfer.NotebookState{UpgradeID: second, RollingBack: true}, true},
			{"none", gone, transfer.NotebookState{}, false},
		} {
			calls = nil
			if got, ok, err := r.of(n)(context.Background(), tt.id); err != nil || ok != tt.ok || got != tt.want ||
				!slices.Equal(calls, []string{r.call}) {
				t.Errorf("%s: %s() = %+v, %v, %v by %v; want %+v, %v by %s", tt.name, r.name, got, ok, err, calls, tt.want, tt.ok, r.call)
			}
		}
		if got, ok, err := r.of(failing)(context.Background(), plain); !errors.Is(err, failure) || ok || got != (transfer.NotebookState{}) {
			t.Errorf("a failing read: %s() = %+v, %v, %v; want its error", r.name, got, ok, err)
		}
	}
}
