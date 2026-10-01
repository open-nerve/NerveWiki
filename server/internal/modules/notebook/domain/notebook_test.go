package domain

import (
	"errors"
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fields are the problems of err, a 422, as "field code".
func fields(t *testing.T, err error) []string {
	t.Helper()
	var se *shared.Error
	if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed {
		t.Fatalf("err = %v, want validation_failed", err)
	}
	var list []string
	for _, f := range se.Fields {
		list = append(list, f.Field+" "+f.Code)
	}
	return list
}

func ptr(s string) *string { return &s }

func TestCheckDraft(t *testing.T) {
	d, err := CheckDraft("  Engineering ", nil)
	if err != nil || d != (Draft{Name: "Engineering", Access: shared.AccessNone}) {
		t.Errorf("CheckDraft without an access = %+v, %v; want the name trimmed, none", d, err)
	}
	for _, a := range shared.WorkspaceAccesses() {
		if d, err := CheckDraft("x", ptr(string(a))); err != nil || d.Access != a {
			t.Errorf("CheckDraft(%s) = %+v, %v", a, d, err)
		}
	}
	got := fields(t, func() error { _, err := CheckDraft("a/b", ptr("public")); return err }())
	if want := []string{"name invalid_format", "workspace_access invalid_format"}; !slices.Equal(got, want) {
		t.Errorf("both problems = %q, want %q", got, want)
	}
	for _, a := range []string{"", "Viewer", "owner"} {
		if got := fields(t, func() error { _, err := CheckDraft("x", ptr(a)); return err }()); !slices.Equal(got, []string{"workspace_access invalid_format"}) {
			t.Errorf("CheckDraft(access %q) = %q", a, got)
		}
	}
}

func TestCheckChangeAndApply(t *testing.T) {
	n := Notebook{Name: "Engineering", Access: shared.AccessNone}
	viewer := shared.AccessViewer
	for _, tt := range []struct {
		name         string
		nameIn, acc  *string
		want         Notebook
		wantModified bool
	}{
		{"nothing", nil, nil, n, false},
		{"the same name, padded", ptr(" Engineering "), nil, n, false},
		{"the same access", nil, ptr("none"), n, false},
		{"a new name", ptr("Eng"), nil, Notebook{Name: "Eng", Access: shared.AccessNone}, true},
		{"a new access", nil, ptr("viewer"), Notebook{Name: "Engineering", Access: viewer}, true},
		{"both", ptr("Eng"), ptr("viewer"), Notebook{Name: "Eng", Access: viewer}, true},
	} {
		c, err := CheckChange(tt.nameIn, tt.acc)
		if err != nil {
			t.Fatalf("%s: CheckChange() = %v", tt.name, err)
		}
		if got, modified := n.Apply(c); got != tt.want || modified != tt.wantModified {
			t.Errorf("%s: Apply() = %+v, %v; want %+v, %v", tt.name, got, modified, tt.want, tt.wantModified)
		}
	}
	got := fields(t, func() error { _, err := CheckChange(ptr(" "), ptr("all")); return err }())
	if want := []string{"name required", "workspace_access invalid_format"}; !slices.Equal(got, want) {
		t.Errorf("both problems = %q, want %q", got, want)
	}
}
