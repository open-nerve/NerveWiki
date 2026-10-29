package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// sourceImportTree supplies the page module's three successful import
// results. The real transfer run must interpret them, preserve parents
// across commits, and publish reports only after Import succeeds.
type sourceImportTree struct {
	*tree
	duplicates map[string]app.DuplicateSource
	pages      []app.ImportedPage
	busyAfter  int
	failAfter  error
	attempts   int
}

func (t *sourceImportTree) Import(ctx context.Context, spec app.ImportSpec, do func(context.Context, app.ImportUnit) error) (uuid.UUID, error) {
	t.attempts++
	return t.tree.Import(ctx, spec, func(ctx context.Context, u app.ImportUnit) error {
		if err := do(ctx, sourceImportUnit{ImportUnit: u, tree: t}); err != nil {
			return err
		}
		if t.busyAfter > 0 {
			t.busyAfter--
			return shared.ServerBusy(time.Millisecond)
		}
		return t.failAfter
	})
}

type sourceImportUnit struct {
	app.ImportUnit
	tree *sourceImportTree
}

func (u sourceImportUnit) CreatePage(ctx context.Context, p app.ImportedPage) (app.CreatedNode, error) {
	u.tree.pages = append(u.tree.pages, p)
	duplicate, ok := u.tree.duplicates[p.Name]
	if !ok {
		return u.ImportUnit.CreatePage(ctx, p)
	}
	if !p.HasChildren {
		return app.CreatedNode{Skipped: true, Duplicate: &duplicate}, nil
	}
	p.Content = ""
	n, err := u.ImportUnit.CreatePage(ctx, p)
	n.Skipped, n.Duplicate = true, &duplicate
	return n, err
}

func (u sourceImportUnit) CreateAsset(ctx context.Context, a app.ImportedAsset, after func(context.Context, app.CreatedNode) error) (app.CreatedNode, error) {
	if duplicate, ok := u.tree.duplicates[a.Name]; ok {
		return app.CreatedNode{Skipped: true, Duplicate: &duplicate}, nil
	}
	return u.ImportUnit.CreateAsset(ctx, a, after)
}

func runSourceImport(t *testing.T, w *importWorld, tree app.Tree, j domain.Job) domain.Job {
	t.Helper()
	i := app.NewImport(app.ImportDeps{Authorizer: w.auth, Notebooks: w.notebooks, Nodes: w.nodes, Tree: tree, Attachments: w.files,
		Statistics: []app.Analyzer{w.stats}, Archives: w.archives, Rows: w.rows, Clock: fixedClock{now()}, Logger: w.logger,
		Beat: 5 * time.Millisecond, MaxEntries: w.max.MaxEntries, MaxUnpacked: w.max.MaxUnpacked, MaxContent: w.max.MaxContent,
		MaxAsset: w.max.MaxAsset, MaxNodes: w.max.MaxNodes, Backoff: time.Millisecond, Batch: w.max.Batch})
	if err := i.Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	return w.rows.get(j.ID)
}

func TestTheSourceImportKeepsDuplicateParentsChildren(t *testing.T) {
	for _, nodes := range []int{1, 100} {
		t.Run(fmt.Sprintf("batch_%d", nodes), func(t *testing.T) {
			w := newImportWorld()
			w.max.Batch = app.Batch{Nodes: nodes}
			tree := &sourceImportTree{tree: w.tree, duplicates: map[string]app.DuplicateSource{
				"P":         {ID: uuid.NewV7(), Path: "raw/existing-page"},
				"Q":         {ID: uuid.NewV7(), Path: "raw/existing-page"},
				"leaf":      {ID: uuid.NewV7(), Path: "raw/existing-page"},
				"asset.bin": {ID: uuid.NewV7(), Path: "raw/existing-asset"},
			}}
			j := w.queuedImport(nil, zipOf(t, files("P.md", "same", "P/c.md", "child", "P/x.bin", "child asset",
				"Q.md", "same", "Q/y.bin", "another child asset", "leaf.md", "same", "asset.bin", "same asset")...))
			got := runSourceImport(t, w, tree, j)
			wantProblems := []string{"duplicate_source: P.md -> raw/existing-page", "duplicate_source: leaf.md -> raw/existing-page",
				"duplicate_source: Q.md -> raw/existing-page", "duplicate_source: asset.bin -> raw/existing-asset"}
			if got.State != domain.StateSucceeded || got.Progress != (domain.Progress{Done: 7, Total: 7}) || got.Report == nil ||
				got.Report.Counts != (domain.Counts{Pages: 1, Attachments: 2, Skipped: 4}) || !sameProblems(problems(got.Report), wantProblems) {
				t.Fatalf("job %s, progress %+v, report %+v; want children imported and four source skips", got.State, got.Progress, got.Report)
			}
			if made := w.tree.lines(); !slices.Equal(made, []string{"-/P", "-/Q", "P/c", "P/x.bin", "Q/y.bin"}) {
				t.Errorf("created %v, want empty duplicate parent and both children", made)
			}
			parent := w.tree.named(t, "P")
			if parent.content != "" || parent.changeset != *j.ChangesetID || parent.id == (uuid.UUID{}) {
				t.Errorf("duplicate parent = %+v, want a real empty page in the import changeset", parent)
			}
			for _, p := range tree.pages {
				if p.HasChildren != (p.Name == "P" || p.Name == "Q") {
					t.Errorf("%s HasChildren=%t; want full-plan child relation even across batch boundary", p.Name, p.HasChildren)
				}
			}
			if len(w.files.attached) != 2 || w.files.kept() != 2 || len(w.files.dropped) != 1 {
				t.Errorf("files attached=%d kept=%d dropped=%d; want only child assets attached, duplicate file dropped",
					len(w.files.attached), w.files.kept(), len(w.files.dropped))
			}
			if w.tree.parses != w.tree.released {
				t.Errorf("parses=%d released=%d", w.tree.parses, w.tree.released)
			}
		})
	}
}

func TestSourceImportPublishesDuplicateReportsOnlyAfterCommit(t *testing.T) {
	for _, retry := range []bool{false, true} {
		t.Run(fmt.Sprintf("retry_%t", retry), func(t *testing.T) {
			w := newImportWorld()
			tree := &sourceImportTree{tree: w.tree, duplicates: map[string]app.DuplicateSource{
				"leaf": {ID: uuid.NewV7(), Path: "raw/existing"}, "asset.bin": {ID: uuid.NewV7(), Path: "raw/file"},
			}}
			if retry {
				tree.busyAfter = 1
			} else {
				tree.failAfter = errors.New("the unit rolled back")
			}
			got := runSourceImport(t, w, tree, w.queuedImport(nil, zipOf(t, files("leaf.md", "same", "asset.bin", "same asset", "ok.md", "new")...)))
			if retry {
				if tree.attempts != 2 || got.State != domain.StateSucceeded || got.Report.Counts != (domain.Counts{Pages: 1, Skipped: 2}) ||
					!sameProblems(problems(got.Report), []string{"duplicate_source: leaf.md -> raw/existing", "duplicate_source: asset.bin -> raw/file"}) ||
					got.Progress != (domain.Progress{Done: 3, Total: 3}) || len(w.tree.made) != 1 || len(w.files.dropped) != 1 {
					t.Errorf("attempts=%d job=%s progress=%+v report=%+v made=%d dropped=%d; want one committed report after retry",
						tree.attempts, got.State, got.Progress, got.Report, len(w.tree.made), len(w.files.dropped))
				}
			} else if got.State != domain.StateFailed || got.Report.Counts != (domain.Counts{}) || len(got.Report.Problems) != 0 ||
				got.Progress.Done != 0 || len(w.tree.made) != 0 {
				t.Errorf("job=%s progress=%+v report=%+v made=%d; rolled back result leaked", got.State, got.Progress, got.Report, len(w.tree.made))
			}
		})
	}
}

func TestSourceJobRefusalsCarryTheDuplicateTarget(t *testing.T) {
	for _, target := range []*uuid.UUID{nil, new(uuid.NewV7())} {
		for _, stage := range []string{"import", "plan", "first_batch", "later_batch"} {
			t.Run(fmt.Sprintf("%s/target_%t", stage, target != nil), func(t *testing.T) {
				node := uuid.NewV7()
				refused := &app.Refused{Code: "page.source_duplicate", NodeID: node, DuplicateOf: target,
					Err: &shared.Error{Kind: shared.KindConflict, Code: "page.source_duplicate", DuplicateOf: target}}
				var got domain.Job
				wantPages := int64(0)
				if stage == "import" {
					w := newImportWorld()
					w.tree.refuse = fmt.Errorf("import refused: %w", refused)
					got = w.run(t, w.queuedImport(nil, zipOf(t, files("p.md", "p")...)))
				} else {
					w := newBackgroundWorld()
					w.batch, w.tree.steps = app.Batch{Nodes: 1}, steps(3, 0)
					if stage == "plan" {
						w.tree.planErr = refused
					} else {
						at := 0
						if stage == "later_batch" {
							at, wantPages = 1, 1
						}
						w.tree.fail[at] = fmt.Errorf("batch refused: %w", refused)
					}
					got = w.run(t, w.queued(domain.KindRevert))
					if len(w.tree.succeeded()) != int(wantPages) {
						t.Errorf("committed batches=%v, want %d", w.tree.succeeded(), wantPages)
					}
				}
				if got.State != domain.StateFailed || got.Report == nil || got.Report.Failure != domain.FailureRefused ||
					got.Report.Counts.Pages != wantPages || got.Report.Refusal == nil {
					t.Fatalf("job=%s report=%+v, want refused after %d committed pages", got.State, got.Report, wantPages)
				}
				r := got.Report.Refusal
				if r.Code != refused.Code || r.NodeID != node || !sameDuplicateID(r.DuplicateOf, target) {
					t.Errorf("refusal=%+v, want code %s node %s duplicate %v", r, refused.Code, node, target)
				}
			})
		}
	}
}

func sameDuplicateID(a, b *uuid.UUID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
