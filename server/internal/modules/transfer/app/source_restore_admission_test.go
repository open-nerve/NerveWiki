package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestSourceRestoreClaimsBeforeFullReads(t *testing.T) {
	w := newWorld()
	q, tree := &queue{}, newJobTree(w.rec)
	s := w.starts(q, tree, 20)
	request := app.ImportRequest{NotebookID: w.eng, FileName: "other.zip", Size: -1}
	fullReads := 0
	tree.onAdmit = func() {
		fullReads++
		// A second real start competes for the same upload claim exactly
		// when the page port begins its expensive post-admission reads.
		err := s.imports.Check(as(w.bob), request)
		if !errors.Is(err, domain.ErrWriting) {
			t.Errorf("concurrent import at full reads = %v, want busy from restore's claim", err)
			if err == nil {
				s.imports.Release(request)
			}
		}
	}
	if _, err := start(w, s, domain.KindTrashRestore); err != nil {
		t.Fatal(err)
	}
	if fullReads != 1 || len(q.enqueued) != 1 {
		t.Errorf("full reads=%d, enqueued=%v; want one admitted start", fullReads, q.enqueued)
	}
	writing := 0
	for _, call := range w.rec.calls {
		if call == "Writing" {
			writing++
		}
	}
	if writing != 2 {
		t.Errorf("Writing calls=%d, want one claim read and one transaction recheck; calls=%v", writing, w.rec.calls)
	}
}

func TestSourceRestoreReleasesEveryClaim(t *testing.T) {
	target := uuid.NewV7()
	duplicate := &shared.Error{Kind: shared.KindConflict, Code: "page.source_duplicate", DuplicateOf: &target}
	readErr := errors.New("source metadata read failed")
	for _, failure := range []error{duplicate, readErr, context.Canceled} {
		t.Run(failure.Error(), func(t *testing.T) {
			w := newWorld()
			q, tree := &queue{}, newJobTree(w.rec)
			tree.late = failure
			s := w.starts(q, tree, 20)
			if _, err := start(w, s, domain.KindTrashRestore); !errors.Is(err, failure) || len(q.enqueued) != 0 {
				t.Fatalf("start=%v, enqueued=%v; want original post-admit failure", err, q.enqueued)
			}
			if !slices.Equal(w.rec.calls, []string{"Tree.CheckRestore", "Writing"}) {
				t.Errorf("calls=%v, want admitted check and no queue work", w.rec.calls)
			}
			req := app.ImportRequest{NotebookID: w.eng, FileName: "after.zip", Size: -1}
			if err := s.imports.Check(as(w.bob), req); err != nil {
				t.Errorf("import after refused restore = %v, want released claim", err)
			} else {
				s.imports.Release(req)
			}
		})
	}
}

func TestSourceRestoreBusyDoesNotReachFullReads(t *testing.T) {
	w := newWorld()
	q, tree := &queue{}, newJobTree(w.rec)
	s := w.starts(q, tree, 20)
	req := app.ImportRequest{NotebookID: w.eng, FileName: "holding.zip", Size: -1}
	if err := s.imports.Check(as(w.bob), req); err != nil {
		t.Fatal(err)
	}
	defer s.imports.Release(req)
	fullReads := 0
	tree.onAdmit = func() { fullReads++ }
	if _, err := start(w, s, domain.KindTrashRestore); !errors.Is(err, domain.ErrWriting) {
		t.Errorf("start=%v, want busy", err)
	}
	if fullReads != 0 || len(q.enqueued) != 0 {
		t.Errorf("busy start full reads=%d, enqueued=%v; want zero", fullReads, q.enqueued)
	}
}
