package domain

import (
	"regexp/syntax"
	"slices"
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

// TestGrepKeysReadsABoundedPart: of review A-m1's patterns, whose facts
// hold some 250,000 atoms in some 500 segments, GrepKeys reads 8,192 at
// most (maxGrepKeyAtoms), its keys those of the whole.
func TestGrepKeysReadsABoundedPart(t *testing.T) {
	t.Parallel()
	nested := func(prefix, seed, open, close string, n int) string {
		for range n {
			seed = open + seed + close
		}
		return prefix + seed
	}
	for _, row := range []struct {
		pattern string
		keys    []string
	}{
		{nested("", "a{1000}", "(", ")+b", 249), []string{"aaa", "aab", "abb", "bbb"}},
		{nested("(?i)", "k{1000}", "(", ")+k", 249), []string{"kkk"}},
		{nested("", "a{1000}", "(?:", ")+b", 169), []string{"aaa", "aab", "abb", "bbb"}},
	} {
		re, err := syntax.Parse(row.pattern, syntax.Perl)
		if err != nil {
			t.Fatal(err)
		}
		atoms := 0
		for _, seg := range factsOf(re).must {
			atoms += len(seg)
		}
		keys, read := grepKeys(re)
		if atoms < 100_000 || read > 8192 || !slices.Equal(keys, row.keys) {
			t.Errorf("%s…: %d atoms in its segments, %d read, keys %q; want more than 100,000, at most 8,192, %q",
				row.pattern[:12], atoms, read, keys, row.keys)
		}
	}
}
