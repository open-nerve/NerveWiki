package shared_test

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestKindStatus(t *testing.T) {
	tests := []struct {
		kind shared.Kind
		want int
	}{
		{shared.KindInvalid, 422},
		{shared.KindBadRequest, 400},
		{shared.KindUnauthenticated, 401},
		{shared.KindForbidden, 403},
		{shared.KindNotFound, 404},
		{shared.KindConflict, 409},
		{shared.KindRateLimited, 429},
		{shared.KindUnavailable, 503},
		{shared.Kind(0), 500},
		{shared.Kind(99), 500},
	}
	for _, tt := range tests {
		if got := shared.NewError(tt.kind, "m.code", "d").ProblemStatus(); got != tt.want {
			t.Errorf("Kind %d: ProblemStatus() = %d, want %d", tt.kind, got, tt.want)
		}
	}
}

func TestConstructors(t *testing.T) {
	tests := []struct {
		name   string
		err    *shared.Error
		status int
		code   string
		retry  time.Duration
	}{
		{"Invalid", shared.Invalid(), 422, "validation_failed", 0},
		{"Unauthenticated", shared.Unauthenticated(), 401, "unauthorized", 0},
		{"Forbidden", shared.Forbidden(), 403, "forbidden", 0},
		{"RateLimited", shared.RateLimited(1500 * time.Millisecond), 429, "rate_limited", 1500 * time.Millisecond},
		{"ServerBusy", shared.ServerBusy(time.Second), 503, "server_busy", time.Second},
		{"NotReady", shared.NotReady(time.Second), 503, "not_ready", time.Second},
		{"NewError", shared.NewError(shared.KindConflict, "page.locked", "taken"), 409, "page.locked", 0},
	}
	for _, tt := range tests {
		if tt.err.ProblemStatus() != tt.status || tt.err.ProblemCode() != tt.code || tt.err.RetryAfter() != tt.retry || tt.err.Error() == "" {
			t.Errorf("%s = %d %q retry %s detail %q, want %d %q retry %s and a detail",
				tt.name, tt.err.ProblemStatus(), tt.err.ProblemCode(), tt.err.RetryAfter(), tt.err.Error(), tt.status, tt.code, tt.retry)
		}
	}
}

func TestProblemFields(t *testing.T) {
	err := shared.Invalid(
		shared.FieldError{Field: "title", Code: shared.FieldTooLong, Message: "at most 200 characters"},
		shared.FieldError{Field: "slug", Code: shared.FieldInvalidFormat, Message: "letters, digits and hyphens only"},
	)

	fields := err.ProblemFields()

	if len(fields) != 2 {
		t.Fatalf("ProblemFields() = %v, want 2 fields", fields)
	}
	type field interface {
		error
		ProblemField() string
		ProblemCode() string
	}
	var f field
	if !errors.As(fields[1], &f) || f.ProblemField() != "slug" || f.ProblemCode() != "invalid_format" || f.Error() != "letters, digits and hyphens only" {
		t.Errorf("second field = %v, want slug invalid_format with its message", fields[1])
	}
}

func TestIsMatchesKindAndCode(t *testing.T) {
	taken := shared.NewError(shared.KindConflict, "page.locked", "taken")
	wrapped := fmt.Errorf("register: %w", shared.NewError(shared.KindConflict, "page.locked", "another detail"))

	if !errors.Is(wrapped, taken) {
		t.Error("errors.Is(wrapped page.locked, page.locked) = false, want true")
	}
	if errors.Is(wrapped, shared.NewError(shared.KindForbidden, "page.locked", "")) {
		t.Error("errors.Is matched another kind")
	}
	if errors.Is(wrapped, shared.NewError(shared.KindConflict, "page.other", "")) {
		t.Error("errors.Is matched another code")
	}
}

// The lock and ended_by members are there when set, and absent otherwise:
// the platform leaves a member out of the problem when ok is false.
func TestProblemMembers(t *testing.T) {
	page, user := uuid.New(), uuid.New()
	err := shared.NewError(shared.KindConflict, "page.locked", "locked")

	if _, _, _, ok := err.ProblemLock(); ok {
		t.Error("ProblemLock() ok without a lock")
	}
	if _, _, ok := err.ProblemEndedBy(); ok {
		t.Error("ProblemEndedBy() ok without ended_by")
	}

	err.Lock = &shared.LockHolder{PageID: page, UserID: user, DisplayName: "Ada"}
	err.EndedBy = &shared.Person{UserID: user, DisplayName: "Grace"}
	if p, u, name, ok := err.ProblemLock(); !ok || p != page || u != user || name != "Ada" {
		t.Errorf("ProblemLock() = %s %s %q %v, want %s %s Ada true", p, u, name, ok, page, user)
	}
	if u, name, ok := err.ProblemEndedBy(); !ok || u != user || name != "Grace" {
		t.Errorf("ProblemEndedBy() = %s %q %v, want %s Grace true", u, name, ok, user)
	}
}

// FieldCodes is the whole closed set: it lists every Field* constant of the
// package's source, so a code declared but left out of it fails here, not
// silently past the contract test (P4).
func TestFieldCodesListEveryFieldConstant(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var declared []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.CONST {
				for _, spec := range gen.Specs {
					declared = append(declared, fieldCodes(spec.(*ast.ValueSpec))...)
				}
			}
		}
	}
	slices.Sort(declared)
	listed := slices.Sorted(slices.Values(shared.FieldCodes()))
	if len(declared) == 0 || !slices.Equal(listed, declared) {
		t.Errorf("FieldCodes() = %q, want the Field* constants %q", listed, declared)
	}
}

// fieldCodes returns the values of the Field* string constants of spec.
func fieldCodes(spec *ast.ValueSpec) []string {
	var codes []string
	for i, name := range spec.Names {
		if !strings.HasPrefix(name.Name, "Field") || i >= len(spec.Values) {
			continue
		}
		if lit, ok := spec.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if code, err := strconv.Unquote(lit.Value); err == nil {
				codes = append(codes, code)
			}
		}
	}
	return codes
}
