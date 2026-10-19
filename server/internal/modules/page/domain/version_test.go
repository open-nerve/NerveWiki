package domain_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A version holds the revisions from after its base to its own; the one
// that created its page, those from 1; none holds one below 1.
func TestAVersionHoldsTheRevisionsAfterItsBase(t *testing.T) {
	base := 2
	session := domain.Version{Base: &base, Revision: 5}
	creation := domain.Version{Revision: 3}
	for _, tt := range []struct {
		v        domain.Version
		revision int
		want     bool
	}{
		{session, 2, false}, {session, 3, true}, {session, 5, true}, {session, 6, false},
		{creation, 0, false}, {creation, -1, false}, {creation, 1, true}, {creation, 3, true}, {creation, 4, false},
	} {
		if got := tt.v.Holds(tt.revision); got != tt.want {
			t.Errorf("version (%v, %d] holds %d = %v, want %v", tt.v.Base, tt.v.Revision, tt.revision, got, tt.want)
		}
	}
}

// fieldsOf are the fields of err's 422, field and code each; nil for no
// error, and a failure for another.
func fieldsOf(t *testing.T, err error) []string {
	t.Helper()
	if err == nil {
		return nil
	}
	var e *shared.Error
	if !errors.As(err, &e) || e.Code != shared.CodeValidationFailed {
		t.Fatalf("%v is no 422", err)
	}
	out := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		out[i] = f.Field + " " + f.Code
	}
	return out
}

// A revision versions reads is 1 to MaxRevision, out_of_range on revision
// otherwise, which no version can hold (M9/P2 design 3.16.1).
func TestCheckRevision(t *testing.T) {
	for _, r := range []int{1, 2, domain.MaxRevision} {
		if err := domain.CheckRevision(r); err != nil {
			t.Errorf("CheckRevision(%d) = %v, want nil", r, err)
		}
	}
	for _, r := range []int{0, -1, domain.MaxRevision + 1} {
		var e *shared.Error
		err := domain.CheckRevision(r)
		if got := fieldsOf(t, err); !slices.Equal(got, []string{"revision out_of_range"}) || !errors.As(err, &e) ||
			e.Fields[0].Message != "must be from 1 to 2147483647" {
			t.Errorf("CheckRevision(%d) = %v, want out_of_range on revision", r, err)
		}
	}
}

// Each argument of versions' other call is not_allowed, one field each:
// with revision, cursor; without it, offset and column. limit and the
// call's own arguments pass, whatever their values (M9/P2 design
// 3.16.1).
func TestCheckVersionsArgs(t *testing.T) {
	zero, two, cursor := 0, 2, "c"
	for _, tt := range []struct {
		name                     string
		revision, offset, column *int
		cursor                   *string
		want                     []string
	}{
		{"the list", nil, nil, nil, nil, nil},
		{"the list, on from a cursor", nil, nil, nil, &cursor, nil},
		{"a version", &two, nil, nil, nil, nil},
		{"a version from a line and a column", &two, &two, &zero, nil, nil},
		{"a revision out of range, judged after", &zero, &zero, &zero, nil, nil},
		{"a version and a cursor", &two, nil, nil, &cursor, []string{"cursor not_allowed"}},
		{"a version from a line, and a cursor", &two, &two, &two, &cursor, []string{"cursor not_allowed"}},
		{"the list and an offset", nil, &two, nil, nil, []string{"offset not_allowed"}},
		{"the list and a column", nil, nil, &zero, nil, []string{"column not_allowed"}},
		{"the list, an offset and a column", nil, &two, &zero, nil, []string{"offset not_allowed", "column not_allowed"}},
		{"the list on from a cursor, and an offset", nil, &two, nil, &cursor, []string{"offset not_allowed"}},
	} {
		if got := fieldsOf(t, domain.CheckVersionsArgs(tt.revision, tt.offset, tt.column, tt.cursor)); !slices.Equal(got, tt.want) {
			t.Errorf("%s: CheckVersionsArgs = %v, want %v", tt.name, got, tt.want)
		}
	}
	for _, tt := range []struct {
		err  error
		want string
	}{
		{domain.CheckVersionsArgs(&two, nil, nil, &cursor), "is for the list of versions, without revision"},
		{domain.CheckVersionsArgs(nil, &two, &two, nil), "is for reading a version, with revision"},
	} {
		var e *shared.Error
		if !errors.As(tt.err, &e) || e.Fields[0].Message != tt.want {
			t.Errorf("%v tells %q", tt.err, tt.want)
		}
	}
}
