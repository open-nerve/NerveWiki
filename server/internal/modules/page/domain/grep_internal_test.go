package domain

import (
	"regexp/syntax"
	"testing"
)

// TestPastGoBounds checks which of the parser's errors are Go's bounds,
// too_long, and which the syntax's: ErrLarge and ErrNestingDepth no
// pattern of MaxGrepPatternBytes reaches, so their rows are made here.
func TestPastGoBounds(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		err  syntax.Error
		want bool
	}{
		{syntax.Error{Code: syntax.ErrLarge, Expr: "x"}, true},
		{syntax.Error{Code: syntax.ErrNestingDepth, Expr: "("}, true},
		{syntax.Error{Code: syntax.ErrInvalidRepeatSize, Expr: "{1001}"}, true},
		{syntax.Error{Code: syntax.ErrInvalidRepeatSize, Expr: "{1001,}"}, true},
		{syntax.Error{Code: syntax.ErrInvalidRepeatSize, Expr: "{2,1001}"}, true},
		{syntax.Error{Code: syntax.ErrInvalidRepeatSize, Expr: "{2}"}, true}, // nested past 1,000
		{syntax.Error{Code: syntax.ErrInvalidRepeatSize, Expr: "{2,1}"}, false},
		{syntax.Error{Code: syntax.ErrMissingParen, Expr: "a("}, false},
	} {
		if got := pastGoBounds(&row.err); got != row.want {
			t.Errorf("pastGoBounds(%v) = %v, want %v", &row.err, got, row.want)
		}
	}
}
