package app_test

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The root, empty pages, and final zero-byte attachment all count. Complete
// metadata makes an erroneous unbounded move reach a real write, not a fake
// fixture failure; real database rollback/work bounds are tested in bootstrap.
func sourceEntryFixture(size int) (*fixture, domain.Node, domain.Node) {
	f := newFixture()
	f.grant(domain.ActionRead, domain.ActionMove, domain.ActionRename, domain.ActionRevert)
	raw, folder := f.page("raw", nil, 0), f.page("Folder", nil, 1)
	for i := range size - 2 {
		f.page(fmt.Sprintf("empty-%05d", i), &folder.ID, float64(i))
	}
	asset := f.asset("zero.bin", &folder.ID, float64(size))
	hash := sha256.Sum256(nil)
	f.infos = map[uuid.UUID]app.AttachmentInfo{asset.ID: {SHA256: hash[:]}}
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
	f.guards = []app.WriteGuard{app.NewModeGuard(f.auth, f.store)}
	f.rec.calls = nil
	return f, raw, folder
}

func TestSourceEntryRefusesBeforeHashAndAssetWork(t *testing.T) {
	for _, route := range []string{"move", "relocate"} {
		t.Run(route, func(t *testing.T) {
			f, raw, folder := sourceEntryFixture(domain.MaxUnitNodes + 1)
			var err error
			if route == "move" {
				_, err = f.move(folder.ID, app.Destination{ParentID: &raw.ID})
			} else {
				_, err = f.movePath(f.eng, app.PathMove{From: "Folder.md", To: "raw/Folder.md"})
			}
			var refused *domain.Refused
			if !errors.Is(err, domain.ErrSourceTooLarge) || !errors.As(err, &refused) || refused.NodeID != folder.ID {
				t.Errorf("source entry = %v (%+v), want source_too_large marked at moved root", err, refused)
			}
			if !f.called("SubtreeWithin in tx") || f.called("Subtree in tx") {
				t.Errorf("source entry did not use only the finite subtree reader: %v", f.rec.calls)
			}
			for _, call := range f.rec.calls {
				if strings.HasPrefix(call, "SourceHeads") || strings.HasPrefix(call, "AttachmentInfos") ||
					strings.HasPrefix(call, "SetSourceHashes") || strings.HasPrefix(call, "ContentMeta") ||
					strings.Contains(call, "page.manage_source") {
					t.Errorf("overflow did later source work: %s", call)
				}
			}
			if f.store.nodes[folder.ID].ParentID != nil || len(f.store.changesets) != 0 || len(f.store.recorded) != 0 || f.took() {
				t.Errorf("overflow wrote or parsed: node=%+v changesets=%d items=%d", f.store.nodes[folder.ID], len(f.store.changesets), len(f.store.recorded))
			}
		})
	}
}

func TestSourceEntryLimitIncludesRootEmptyPagesAndAssets(t *testing.T) {
	f, raw, folder := sourceEntryFixture(domain.MaxUnitNodes)
	if _, err := f.move(folder.ID, app.Destination{ParentID: &raw.ID}); err != nil {
		t.Fatal(err)
	}
	if !f.called("SubtreeWithin in tx") || f.called("Subtree in tx") || len(f.store.changesets) != 1 {
		t.Errorf("exactly 10000 should move once through finite reader: %v", f.rec.calls)
	}
	checkFakeSourceRebuild(t, f, 1)
}

func TestSourceEntryHistoryChecksCurrentCarriedSubtree(t *testing.T) {
	for _, route := range []string{"sync", "check", "plan"} {
		t.Run(route, func(t *testing.T) {
			f, raw, folder := sourceEntryFixture(domain.MaxUnitNodes + 1)
			// A previously committed move out carried this whole valid subtree.
			// Only its root is a changeset item; undo would bring all 10001 back.
			before, after := folder.State(), folder.State()
			before.ParentID = &raw.ID
			c := f.log.changeset(domain.ChangesetEdit, f.alice, now(), []domain.ChangesetItem{{NodeID: folder.ID, Before: &before, After: &after}})
			var err error
			switch route {
			case "sync":
				_, err = f.revert(c.ID, nil, 0)
			case "check":
				_, err = f.jobs(now(), 0, 0).CheckRevert(f.asAlice(), f.eng, c.ID, nil, nil, allJobBatches)
			case "plan":
				_, err = f.jobs(now(), 0, 0).PlanRevert(f.asJob(), f.jobSpec(), c.ID, allJobBatches)
			}
			var refused *domain.Refused
			if !errors.Is(err, domain.ErrSourceTooLarge) || !errors.As(err, &refused) || refused.NodeID != folder.ID {
				t.Errorf("history %s = %v (%+v), want source_too_large for carried root", route, err, refused)
			}
			if f.took() || f.called("LockNotebook in tx") || len(f.store.changesets) != 0 || f.store.nodes[folder.ID].ParentID != nil {
				t.Errorf("history refusal reached parser/unit/write: %v", f.rec.calls)
			}
		})
	}
}

func TestSourceEntryOverflowPrecedesOnlyUnseenDescendantDepth(t *testing.T) {
	for _, size := range []int{11, domain.MaxUnitNodes + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			f, raw, folder := sourceEntryFixture(size)
			parent := folder.ID
			for i := range 9 {
				name := fmt.Sprintf("empty-%05d", i)
				for id, n := range f.store.nodes {
					if n.Name == name {
						n.ParentID = new(parent)
						f.store.nodes[id] = n
						parent = id
						break
					}
				}
			}
			_, err := f.move(folder.ID, app.Destination{ParentID: &raw.ID})
			want := domain.ErrTooDeep
			if size > domain.MaxUnitNodes {
				want = domain.ErrSourceTooLarge
			}
			if !errors.Is(err, want) {
				t.Errorf("entry of %d with a deep descendant = %v, want %v", size, err, want)
			}
			if len(f.store.changesets) != 0 || f.called("SourceHeads in tx") {
				t.Errorf("refused depth/size started source writes or metadata: %v", f.rec.calls)
			}
		})
	}
}

func TestSourceEntryKnownErrorsPrecedeTheFiniteReader(t *testing.T) {
	for _, kind := range []string{"cycle", "title", "protected"} {
		t.Run(kind, func(t *testing.T) {
			f, raw, folder := sourceEntryFixture(3)
			want := ""
			switch kind {
			case "cycle":
				raw.ParentID = &folder.ID
				f.store.nodes[raw.ID] = raw
				want = "page.cycle"
			case "title":
				f.page("Folder", &raw.ID, 1)
				want = "page.title_taken"
			case "protected":
				folder.Name, folder.NameKey = "AGENTS", "agents"
				f.store.nodes[folder.ID] = folder
				want = "page.protected"
			}
			_, err := f.move(folder.ID, app.Destination{ParentID: &raw.ID})
			if codeOf(err) != want {
				t.Errorf("known %s = %v, want %s", kind, err, want)
			}
			if f.called("SubtreeWithin in tx") || f.called("Subtree in tx") || f.called("SourceHeads in tx") || len(f.store.changesets) != 0 {
				t.Errorf("known %s read whole/finite subtree or source metadata before refusing: %v", kind, f.rec.calls)
			}
		})
	}
}

func TestSourceEntryLockedHistoryPlaceRechecksGrowth(t *testing.T) {
	f, raw, folder := sourceEntryFixture(domain.MaxUnitNodes)
	before, after := folder.State(), folder.State()
	before.ParentID = &raw.ID
	c := f.log.changeset(domain.ChangesetEdit, f.alice, now(), []domain.ChangesetItem{{NodeID: folder.ID, Before: &before, After: &after}})
	w, spec := f.jobs(now(), 0, 0), f.jobSpec()
	plan, err := w.PlanRevert(f.asJob(), spec, c.ID, allJobBatches)
	if err != nil {
		t.Fatal(err)
	}
	// The earlier plan was legal. Growth does not change the recorded root
	// placement, so the locked place must enforce the current carried size.
	f.page("Arrived after plan", &folder.ID, domain.MaxUnitNodes+1)
	f.rec.calls = nil
	_, err = w.RunBatch(f.asJob(), spec, plan, 0, len(plan.Steps()))
	var refused *domain.Refused
	if !errors.Is(err, domain.ErrSourceTooLarge) || !errors.As(err, &refused) || refused.NodeID != folder.ID {
		t.Errorf("locked history place = %v (%+v), want current source-entry refusal", err, refused)
	}
	if !f.called("SubtreeWithin in tx") || f.called("Subtree in tx") {
		t.Errorf("locked place did not use finite reader: %v", f.rec.calls)
	}
	for _, call := range f.rec.calls {
		if strings.HasPrefix(call, "SourceHeads") || strings.HasPrefix(call, "AttachmentInfos") || strings.HasPrefix(call, "SetSourceHashes") {
			t.Errorf("locked place overflow did later source work: %s", call)
		}
	}
	if len(f.store.changesets) != 0 || f.store.nodes[folder.ID].ParentID != nil {
		t.Errorf("locked place wrote before rejecting: %v", f.rec.calls)
	}
}

func TestSourceEntryBatchRechecksTheLockedTree(t *testing.T) {
	f, _, folder := sourceEntryFixture(domain.MaxUnitNodes)
	f.notebooks.waited = func() {
		f.notebooks.waited = nil
		f.page("Arrived at lock", &folder.ID, domain.MaxUnitNodes+1)
	}
	_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, moveOp("Folder.md", "raw/Folder.md"))
	var refused *domain.Refused
	var failed *domain.OpFailed
	if !errors.Is(err, domain.ErrSourceTooLarge) || !errors.As(err, &refused) || refused.NodeID != folder.ID ||
		!errors.As(err, &failed) || failed.Op != 0 || failed.Tool != domain.OpKindMove {
		t.Errorf("batch after growth = %v, refusal=%+v op=%+v", err, refused, failed)
	}
	if !f.called("SubtreeWithin in tx") || f.called("SourceHeads in tx") || len(f.store.changesets) != 0 || f.store.nodes[folder.ID].ParentID != nil {
		t.Errorf("batch growth did not reject before source work/writes: %v", f.rec.calls)
	}
}
