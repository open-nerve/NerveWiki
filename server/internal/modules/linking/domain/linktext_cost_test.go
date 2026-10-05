//go:build !race

package domain_test

import (
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The link targets of a notebook whose ten thousand folders each hold a
// page of the same title, which only its path writes, are written in well
// under a second, though each one's path is read against all the others
// of its title (M6/P5 design 6). The race detector slows the code too much
// for a bound in time: make test-go runs it in a build without.
func TestTheLinktextsOfManyPagesOfOneTitleTakeUnderASecond(t *testing.T) {
	const folders = 10_000
	named := map[string][]domain.Node{}
	var nodes []domain.Node
	for i := range folders {
		var folder, page uuid.UUID
		folder[0], folder[1], folder[2] = 1, byte(i>>8), byte(i)
		page[0], page[1], page[2] = 2, byte(i>>8), byte(i)
		name := fmt.Sprintf("f%d", i)
		f := domain.Step{ID: folder, Key: shared.TitleKey(name), Name: name}
		n := domain.Node{ID: page, Path: []domain.Step{f, {ID: page, Key: "index", Name: "index"}}}
		named[f.Key] = append(named[f.Key], domain.Node{ID: folder, Path: []domain.Step{f}})
		named["index"] = append(named["index"], n)
		nodes = append(nodes, n)
	}
	tree := domain.Tree{Named: named}
	start := time.Now()
	for i, n := range nodes {
		if got, ok := domain.Linktext(n, nil, tree); !ok || got != fmt.Sprintf("f%d/index", i) {
			t.Fatalf("the linktext of %d is %q, %t", i, got, ok)
		}
	}
	if took := time.Since(start); took > time.Second {
		t.Errorf("the linktexts took %s", took)
	}
}
