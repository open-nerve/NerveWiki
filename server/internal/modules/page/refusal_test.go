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
		code, id, ok := page.Refusal(tt.err)
		want := uuid.UUID{}
		if tt.ok {
			want = node
		}
		if code != tt.code || id != want || ok != tt.ok {
			t.Errorf("%s: Refusal() = %q, %s, %v; want %q, %s, %v", tt.name, code, id, ok, tt.code, want, tt.ok)
		}
	}
}
