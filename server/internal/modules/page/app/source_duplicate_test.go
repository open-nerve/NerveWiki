package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func sourceDuplicateFixture() (*fixture, domain.Node, domain.Node) {
	f := newFixture()
	f.grant(domain.ActionCreate, domain.ActionWrite, domain.ActionMove, domain.ActionManageSource, upload)
	raw := f.page("raw", nil, 0)
	known := f.page("Known", &raw.ID, 0)
	f.writeAs(known.ID, "same")
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
	f.store.sourceHashes[known.ID] = sourceHashOf("same")
	f.guards = []app.WriteGuard{app.NewModeGuard(f.auth, f.store, f.store)}
	return f, raw, known
}
func TestSourceGuardRejectsDuplicateBeforeWritingAndLocksOnlyShareFirst(t *testing.T) {
	for _, first := range []bool{false, true} {
		t.Run(map[bool]string{false: "tree creation", true: "SHARE first body"}[first], func(t *testing.T) {
			f, raw, known := sourceDuplicateFixture()
			var err error
			if first {
				empty := f.page("Empty", &raw.ID, 1)
				f.rec.calls = nil
				_, err = f.put(empty.ID, app.ContentPut{Content: "same", Base: 1})
			} else {
				f.rec.calls = nil
				_, err = f.create(app.PageDraft{Title: "Duplicate", ParentID: &raw.ID, Content: "same"})
			}
			var e *shared.Error
			if !errors.As(err, &e) || e.Code != "page.source_duplicate" || e.DuplicateOf == nil || *e.DuplicateOf != known.ID {
				t.Errorf("duplicate result=%v, target=%+v", err, e)
			}
			lock, probe := -1, -1
			for i, c := range f.rec.calls {
				if c == "LockSources in tx" {
					lock = i
				}
				if c == "ProbeSources in tx" {
					probe = i
				}
				if strings.HasPrefix(c, "CreateChangeset") || strings.HasPrefix(c, "RecordItems") {
					t.Errorf("refused admission wrote: %s", c)
				}
			}
			if probe < 0 || first && (lock < 0 || lock > probe) || !first && lock >= 0 {
				t.Errorf("lock/probe order=%d/%d first=%v calls=%v", lock, probe, first, f.rec.calls)
			}
		})
	}
}
func TestSourceImportSkipsDuplicateLeafAndKeepsNonleafEmpty(t *testing.T) {
	for _, hasChildren := range []bool{false, true} {
		t.Run(map[bool]string{false: "leaf", true: "parent"}[hasChildren], func(t *testing.T) {
			f, raw, known := sourceDuplicateFixture()
			parsed, err := f.imports().Parse(f.asAlice(), "same")
			if err != nil {
				t.Fatal(err)
			}
			defer parsed.Release()
			var got app.CreatedNode
			_, err = f.importAs(uuid.Nil(), func(ctx context.Context, u *app.ImportUnit) error {
				got, err = u.CreatePage(ctx, app.ImportedPage{ParentID: &raw.ID, Name: "Duplicate", Content: "same", Parsed: parsed, HasChildren: hasChildren})
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			if !got.Skipped || got.Duplicate == nil || got.Duplicate.ID != known.ID {
				t.Errorf("import duplicate=%+v", got)
			}
			if hasChildren {
				if got.ID == uuid.Nil() || f.store.contents[got.ID].Content != "" {
					t.Errorf("duplicate parent must exist empty: %+v", got)
				}
			} else if got.ID != uuid.Nil() {
				t.Errorf("duplicate leaf inserted: %s", got.ID)
			}
		})
	}
}
func TestSourceRestoreCallsAdmissionAfterBasicChecks(t *testing.T) {
	f := newBoxFixture()
	sentinel := errors.New("admission full")
	called := 0
	_, err := f.jobs(now(), 0, 0).CheckRestore(f.asAlice(), f.eng, f.box.ID, func(context.Context) error { called++; return sentinel })
	if !errors.Is(err, sentinel) || called != 1 {
		t.Errorf("admit error/calls=%v/%d", err, called)
	}
	if slices.Contains(f.rec.calls, "TrashShape") {
		t.Errorf("busy read full group: %v", f.rec.calls)
	}
}

func TestSourceGuardKeepsTheFirstKeyAcrossRewrites(t *testing.T) {
	for _, back := range []bool{false, true} {
		t.Run(map[bool]string{false: "different key", true: "back to baseline"}[back], func(t *testing.T) {
			f, raw, known := sourceDuplicateFixture()
			f.grant(domain.ActionRename)
			body := "changed"
			if back {
				body = "same"
			}
			_, err := f.run(f.asAlice(), domain.ClientAPI, func(ctx context.Context, u *app.Unit) error {
				rev, err := u.WriteContent(ctx, app.ContentWrite{NodeID: known.ID, Content: "changed", Base: f.store.contents[known.ID].Revision, Rewrite: true})
				if err != nil {
					return err
				}
				if back {
					if _, err = u.WriteContent(ctx, app.ContentWrite{NodeID: known.ID, Content: "same", Base: rev, Rewrite: true}); err != nil {
						return err
					}
				}
				_, err = u.CreatePage(ctx, app.PageDraft{Title: "Duplicate", ParentID: &raw.ID, Content: body})
				return err
			})
			var e *shared.Error
			if !errors.As(err, &e) || e.Code != "page.source_duplicate" {
				t.Fatalf("rewrite duplicate=%v", err)
			}
			if (e.DuplicateOf != nil) != back || back && *e.DuplicateOf != known.ID {
				t.Errorf("rewrite provenance=%v, want original key restored=%v", e.DuplicateOf, back)
			}
		})
	}
}

// This exercises the actual revertChecks callback, not merely SourcePrecheck
// with a test no-op. Permission reads must share the snapshot connection.
func TestSourceRevertPrechecksAuthorizeWithTheSnapshotContext(t *testing.T) {
	for _, entry := range []string{"sync", "start", "worker"} {
		t.Run(entry, func(t *testing.T) {
			f, _, known := sourceDuplicateFixture()
			f.grant(domain.ActionRevert)
			c := f.creation(known.ID)
			f.rec.calls = nil
			var err error
			switch entry {
			case "sync":
				_, err = f.revert(c.ID, nil, 0)
			case "start":
				_, err = f.jobs(now(), 0, 0).CheckRevert(f.asAlice(), f.eng, c.ID, nil, nil, allJobBatches)
			case "worker":
				_, err = f.jobs(now(), 0, 0).PlanRevert(f.asJob(), f.jobSpec(), c.ID, allJobBatches)
			}
			if err != nil {
				t.Fatal(err)
			}
			snapshot := slices.Index(f.rec.calls, "WithinSnapshot")
			if snapshot < 0 {
				t.Fatalf("missing source snapshot: %v", f.rec.calls)
			}
			managed := slices.Index(f.rec.calls[snapshot+1:], "Authorize page.manage_source in tx")
			if managed < 0 {
				t.Errorf("actual revertChecks authorization lost snapshot context: %v", f.rec.calls)
			}
			if slices.Contains(f.rec.calls, "Authorize page.manage_source") {
				t.Errorf("source authorization requested a connection outside snapshot: %v", f.rec.calls)
			}
			if managed >= 0 {
				at := snapshot + 1 + managed
				if at == 0 || f.rec.calls[at-1] != "WorkspaceOf in tx" {
					t.Errorf("source workspace point read lost snapshot context: %v", f.rec.calls)
				}
			}
		})
	}
}
