package app

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// A list's globs are bound by MaxGlobWork (M9/P1 design 3.10.1): its
// constructor gives it, and the module has no other to pass (M9/P1 S2
// review, C-N1).
func TestAListsGlobsAreBoundByMaxGlobWork(t *testing.T) {
	if l := NewListByPattern(nil, nil, nil, nil, nil); l.maxWork != domain.MaxGlobWork {
		t.Errorf("a list's glob work is bound at %d, want MaxGlobWork", l.maxWork)
	}
}
