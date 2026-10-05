package app_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// invalid tells whether err is 422 on target with code.
func invalid(err error, code string) bool {
	var e *shared.Error
	return errors.As(err, &e) && e.Code == shared.CodeValidationFailed &&
		len(e.Fields) == 1 && e.Fields[0].Field == "target" && e.Fields[0].Code == code
}

// A landing is a writer's: a role that does not write is forbidden, before
// the target is judged. Then a target absent or longer than 4096 bytes is
// 422 on target; one that cuts into no segments, holds a NUL or is not
// valid UTF-8 has no landing, without a read (M6/P6 design 2).
func TestALandingJudgesTheRoleThenTheTarget(t *testing.T) {
	l := newLibrary()
	l.refused = []shared.Action{domain.ActionReadLinkLanding}
	for _, target := range []*string{nil, new("x")} {
		if _, err := l.getLinkLanding(10).Execute(reader(), l.p, target); !errors.Is(err, shared.Forbidden()) {
			t.Errorf("a reader's landing of %v: %v", target, err)
		}
	}
	l.refused = nil
	l.notebooks[l.tree.add("p")] = l.nb
	from := l.tree.names["p"]
	if _, err := l.getLinkLanding(10).Execute(reader(), from, nil); !invalid(err, shared.FieldRequired) {
		t.Errorf("no target: %v", err)
	}
	if _, err := l.getLinkLanding(10).Execute(reader(), from, new(strings.Repeat("a", 4097))); !invalid(err, shared.FieldTooLong) {
		t.Errorf("4097 bytes: %v", err)
	}
	l.calls = nil
	for _, target := range []string{"", "/", "./", "../", "a//b", "a/./b", "a/../b", "a/", "a\x00b", "\xff", "a/\xc3"} {
		got, err := l.getLinkLanding(10).Execute(reader(), from, &target)
		if err != nil || got != (domain.Landing{Reason: domain.TargetInvalid}) {
			t.Errorf("%q: %+v, %v", target, got, err)
		}
	}
	for _, c := range l.calls {
		if !strings.HasPrefix(c, "WorkspaceOf") && !strings.HasPrefix(c, "NotebookOf") && !strings.HasPrefix(c, "Authorize") {
			t.Errorf("read %s for a target that is none", c)
		}
	}
	got, err := l.getLinkLanding(10).Execute(reader(), from, new(strings.Repeat("a", 4096)))
	if err != nil || got != (domain.Landing{Reason: domain.TitleInvalid}) {
		t.Errorf("4096 bytes: %+v, %v", got, err)
	}
}

// A landing reads the pages with the target's last keys and the key of
// its segment before the last, the pages with an alias of its last keys
// for a name alone, and the paths of those and of its page; an aliased page deleted since
// is none, and its page deleted since the decision is page.not_found. It
// lands as domain.Land says, pages nesting MaxDepth deep (M6/P6 design 2).
func TestALandingReadsThePagesItsTargetNames(t *testing.T) {
	l := newLibrary()
	for _, p := range []string{"A", "A/src", "A/B", "Q", "P"} {
		l.notebooks[l.tree.add(p)] = l.nb
	}
	id := func(p string) uuid.UUID { return l.tree.names[p] }
	l.aliasRows = []app.Alias{{PageID: id("P"), Key: "al"}, {PageID: uuid.NewV7(), Key: "ghost"}}
	tests := []struct {
		target string
		depth  int
		want   domain.Landing
		calls  []string
	}{
		{"x", 10, domain.Landing{Parent: id("A"), Title: "x"}, []string{"ByKeys x", "Aliases x", "Paths"}},
		// Only a name alone may lead by an alias (M6/P3 design 2, step 4).
		{"B/x", 10, domain.Landing{Parent: id("A/B"), Title: "x"}, []string{"ByKeys x b", "Paths"}},
		{"Q/X.md", 10, domain.Landing{Parent: id("Q"), Title: "X"}, []string{"ByKeys x x.md q", "Paths"}},
		{"./x.md", 10, domain.Landing{Parent: id("A"), Title: "x"}, []string{"ByKeys x x.md", "Paths"}},
		{"X.md", 10, domain.Landing{Parent: id("A"), Title: "X"}, []string{"ByKeys x x.md", "Aliases x x.md", "Paths"}},
		{"Al", 10, domain.Landing{Node: id("P")}, nil},
		{"src", 10, domain.Landing{Node: id("A/src")}, nil},
		{"ghost", 10, domain.Landing{Parent: id("A"), Title: "ghost"}, nil},
		{"B/x", 2, domain.Landing{Reason: domain.TooDeep}, nil},
		{"Z/x", 10, domain.Landing{Reason: domain.ParentMissing}, nil},
	}
	for _, tt := range tests {
		l.calls = nil
		got, err := l.getLinkLanding(tt.depth).Execute(reader(), id("A/src"), &tt.target)
		if err != nil || got != tt.want {
			t.Errorf("%s: %+v, %v; want %+v", tt.target, got, err, tt.want)
		}
		read := slices.DeleteFunc(slices.Clone(l.calls), func(c string) bool {
			return strings.HasPrefix(c, "WorkspaceOf") || strings.HasPrefix(c, "NotebookOf") || strings.HasPrefix(c, "Authorize")
		})
		if tt.calls != nil && !slices.Equal(read, tt.calls) {
			t.Errorf("%s read %q, want %q", tt.target, read, tt.calls)
		}
	}
	l.tree.nodes[id("A/src")].gone = true
	if _, err := l.getLinkLanding(10).Execute(reader(), id("A/src"), new("x")); !errors.Is(err, domain.ErrPageNotFound) {
		t.Errorf("a page deleted since the decision: %v", err)
	}
}
