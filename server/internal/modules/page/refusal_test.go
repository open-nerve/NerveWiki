package page_test

import (
	"errors"
	"fmt"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Refusal reads a marked refusal's code and node through what wraps it
// (M10/P1 design 3.22): a batch's op, a job's wrap; an unmarked problem,
// another error and none are no refusal.
func TestRefusalReadsTheMark(t *testing.T) {
	node := uuid.NewV7()
	marked := domain.Refuse(domain.ErrSourceAdminOnly, node)
	for _, tt := range []struct {
		name string
		err  error
		code string
		ok   bool
	}{
		{"marked", marked, "page.source_admin_only", true},
		{"at a batch's op", &domain.OpFailed{Op: 1, Tool: domain.OpKindMove, Err: marked}, "page.source_admin_only", true},
		{"wrapped", fmt.Errorf("a revert's batch: %w", domain.Refuse(domain.ErrLocked, node)), "page.locked", true},
		{"an unmarked problem", domain.ErrProtected, "", false},
		{"another error", errors.New("the database is gone"), "", false},
		{"none", nil, "", false},
	} {
		info, ok := page.Refusal(tt.err)
		code, id := info.Code, info.NodeID
		want := uuid.UUID{}
		if tt.ok {
			want = node
		}
		if code != tt.code || id != want || ok != tt.ok {
			t.Errorf("%s: Refusal() = %q, %s, %v; want %q, %s, %v", tt.name, code, id, ok, tt.code, want, tt.ok)
		}
	}
}

func TestRefusalCopiesTheOptionalDuplicateTarget(t *testing.T) {
	node, target := uuid.NewV7(), uuid.NewV7()
	problem := domain.SourceDuplicate(&target)
	info, ok := page.Refusal(fmt.Errorf("worker: %w", domain.Refuse(problem, node)))
	if !ok || info.Code != "page.source_duplicate" || info.NodeID != node || info.DuplicateOf == nil || *info.DuplicateOf != target {
		t.Fatalf("duplicate refusal=%+v/%v", info, ok)
	}
	original := target
	target = uuid.NewV7()
	if *problem.DuplicateOf != original {
		t.Error("caller mutated fresh error target")
	}
	other := domain.SourceDuplicate(nil)
	if other == problem || other.DuplicateOf != nil {
		t.Error("source problems share mutable state")
	}
}
