package shared_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// notBegun is an error as platform/postgres marks a transaction's that
// could not begin.
type notBegun struct{}

func (notBegun) Error() string    { return "begin transaction: conn closed" }
func (notBegun) TxNotBegun() bool { return true }

// TxNotBegun finds the mark through wrapping, and nowhere else (M8/P5A fix
// check M-1).
func TestTxNotBegun(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want bool
	}{
		{notBegun{}, true},
		{fmt.Errorf("write: %w", notBegun{}), true},
		{errors.Join(errors.New("rollback"), notBegun{}), true},
		{errors.New("begin transaction: conn closed"), false},
		{nil, false},
	} {
		if got := shared.TxNotBegun(tt.err); got != tt.want {
			t.Errorf("TxNotBegun(%v) = %t, want %t", tt.err, got, tt.want)
		}
	}
}
