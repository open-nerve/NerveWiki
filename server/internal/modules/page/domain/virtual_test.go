package domain_test

import (
	"errors"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestVirtualBatchLookup(t *testing.T) {
	nb := newNotebook("A.md", "A/index.md")
	for _, op := range []domain.BatchArgs{writeOp("index/Child.md", "x"), editOp("log.md", "x", "y"), moveOp("index.md", "B.md"), moveOp("A.md", "log/B.md"), deleteOp("log.md")} {
		p, _ := planIn(t, nb, domain.NotebookState{Mode: domain.ModeLLMWiki}, op)
		if p.Failed == nil || !errors.Is(p.Failed, domain.ErrVirtualFile) {
			t.Errorf("%+v: %v, want virtual_file", op, p.Failed)
		}
	}
}
