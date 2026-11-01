package domain_test

import (
	"errors"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestCanonicalVirtualRead(t *testing.T) {
	for _, tt := range []struct {
		path      string
		name      domain.VirtualName
		canonical bool
	}{
		{"index.md", domain.VirtualIndex, true}, {"/INDEX.MD", domain.VirtualIndex, true}, {"log.md", domain.VirtualLog, true},
		{"index", "", false}, {"index.md.md", "", false}, {"log.md.md", "", false}, {"A/index.md", "", false}, {"Other.md", "", false},
	} {
		p, err := domain.ParsePath("path", tt.path, false)
		if err != nil {
			t.Fatal(err)
		}
		name, canonical := domain.CanonicalVirtualRead(p)
		if name != tt.name || canonical != tt.canonical {
			t.Errorf("%q: %q %v", tt.path, name, canonical)
		}
	}
}

func TestVirtualBatchLookup(t *testing.T) {
	nb := newNotebook("A.md", "A/index.md")
	for _, op := range []domain.BatchArgs{writeOp("index/Child.md", "x"), editOp("log.md", "x", "y"), moveOp("index.md", "B.md"), moveOp("A.md", "log/B.md"), deleteOp("log.md")} {
		p, _ := planIn(t, nb, domain.NotebookState{Mode: domain.ModeLLMWiki}, op)
		if p.Failed == nil || !errors.Is(p.Failed, domain.ErrVirtualFile) {
			t.Errorf("%+v: %v, want virtual_file", op, p.Failed)
		}
	}
}
