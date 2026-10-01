package shared_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestErrNotVisibleIsANotFoundWithoutCode(t *testing.T) {
	if !errors.Is(fmt.Errorf("authorize: %w", shared.ErrNotVisible), shared.ErrNotVisible) {
		t.Error("a wrapped ErrNotVisible is not ErrNotVisible")
	}
	// The use case's own 404 is not the Authorizer's: it has a code.
	if errors.Is(shared.NewError(shared.KindNotFound, "workspace.not_found", "No such workspace."), shared.ErrNotVisible) {
		t.Error("workspace.not_found is ErrNotVisible")
	}
	if shared.ErrNotVisible.ProblemStatus() != 404 || shared.ErrNotVisible.Code != "" {
		t.Errorf("ErrNotVisible = %d %q, want 404 without a code", shared.ErrNotVisible.ProblemStatus(), shared.ErrNotVisible.Code)
	}
}
