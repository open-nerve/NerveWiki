package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Count and Items have separate observations: a zero-row Items query is still
// an unnecessary query, and an Items-only error must not become a Count error.
type lintPageCountProbe struct {
	app.LintRuleStore
	rule                   domain.Rule
	notebook               uuid.UUID
	candidates             []uuid.UUID
	count                  domain.Count
	countErr, itemsErr     error
	ids                    []uuid.UUID
	afterCount             func()
	countCalls, itemsCalls int
	after                  *uuid.UUID
	take                   int
}

func (p *lintPageCountProbe) countOf(in app.PageRuleInput, rule domain.Rule) (domain.Count, error) {
	p.countCalls++
	if rule != p.rule || in.NotebookID != p.notebook || !slices.Equal(in.CandidateIDs, p.candidates) {
		return domain.Count{}, fmt.Errorf("unexpected %s Count input: %+v", rule, in)
	}
	if p.afterCount != nil {
		p.afterCount()
	}
	return p.count, p.countErr
}

func (p *lintPageCountProbe) itemsOf(ctx context.Context, in app.PageRuleInput, rule domain.Rule, after *uuid.UUID, take int) ([]uuid.UUID, error) {
	p.itemsCalls++
	p.take = take
	if after != nil {
		p.after = lintPtr(*after)
	}
	if rule != p.rule || in.NotebookID != p.notebook || !slices.Equal(in.CandidateIDs, p.candidates) {
		return nil, fmt.Errorf("unexpected %s Items input: %+v", rule, in)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return slices.Clone(p.ids), p.itemsErr
}

func (p *lintPageCountProbe) LintOrphanCount(_ context.Context, in app.PageRuleInput) (domain.Count, error) {
	return p.countOf(in, domain.Orphans)
}
func (p *lintPageCountProbe) LintOrphanItems(ctx context.Context, in app.PageRuleInput, after *uuid.UUID, take int) ([]uuid.UUID, error) {
	return p.itemsOf(ctx, in, domain.Orphans, after, take)
}
func (p *lintPageCountProbe) LintFrontmatterCount(_ context.Context, in app.PageRuleInput) (domain.Count, error) {
	return p.countOf(in, domain.InvalidFrontmatter)
}
func (p *lintPageCountProbe) LintFrontmatterItems(ctx context.Context, in app.PageRuleInput, after *uuid.UUID, take int) ([]uuid.UUID, error) {
	return p.itemsOf(ctx, in, domain.InvalidFrontmatter, after, take)
}
func (p *lintPageCountProbe) LintMissingTypeCount(_ context.Context, in app.PageRuleInput) (domain.Count, error) {
	return p.countOf(in, domain.MissingType)
}
func (p *lintPageCountProbe) LintMissingTypeItems(ctx context.Context, in app.PageRuleInput, after *uuid.UUID, take int) ([]uuid.UUID, error) {
	return p.itemsOf(ctx, in, domain.MissingType, after, take)
}
func (p *lintPageCountProbe) LintMissingSummaryCount(_ context.Context, in app.PageRuleInput) (domain.Count, error) {
	return p.countOf(in, domain.MissingSummary)
}
func (p *lintPageCountProbe) LintMissingSummaryItems(ctx context.Context, in app.PageRuleInput, after *uuid.UUID, take int) ([]uuid.UUID, error) {
	return p.itemsOf(ctx, in, domain.MissingSummary, after, take)
}

func lintPageCountFixture(rule domain.Rule, pages int) (*app.Rules, *lintPageCountProbe, app.ReadInput) {
	root := lintID(10)
	nodes := []app.CompactNode{lintNode(10, nil, "raw", domain.NodePage, 0)}
	var candidates []uuid.UUID
	if rule == domain.InvalidFrontmatter {
		candidates = append(candidates, root)
	}
	for i := range pages {
		id := uint64(i + 20)
		nodes = append(nodes, lintNode(id, nil, fmt.Sprintf("page-%d", i), domain.NodePage, 1))
		candidates = append(candidates, lintID(id))
	}
	f, in := lintFixture(nodes...)
	if rule == domain.InvalidFrontmatter {
		for i := range f.heads {
			f.heads[i].FrontmatterValid = false
		}
	}
	in.EffectiveMode, in.AreaRootID = domain.ModeLLM, &root
	probe := &lintPageCountProbe{LintRuleStore: f, rule: rule, notebook: in.NotebookID, candidates: candidates}
	return app.NewRules(app.NewLintReads(f, 5<<20), probe, f), probe, in
}

func TestLintPrepareSkipsEmptyPageItems(t *testing.T) {
	for _, rule := range []domain.Rule{domain.Orphans, domain.InvalidFrontmatter, domain.MissingType, domain.MissingSummary} {
		t.Run(string(rule), func(t *testing.T) {
			for _, after := range []bool{false, true} {
				t.Run(fmt.Sprintf("zero-count/after-%t", after), func(t *testing.T) {
					rules, probe, in := lintPageCountFixture(rule, 2)
					query := app.RuleQuery{Rule: &rule, Limit: 1}
					if after {
						query.After = &domain.Position{ID: lintPtr(lintID(20))}
					}
					prepared, err := rules.Prepare(t.Context(), in, query)
					if err != nil || prepared == nil {
						t.Fatalf("valid zero count returned preparation=%v error=%v", prepared, err)
					}
					if probe.countCalls != 1 || probe.itemsCalls != 0 {
						t.Fatalf("zero count performed Items: count calls=%d items calls=%d, want 1/0", probe.countCalls, probe.itemsCalls)
					}
					items, more := lintAnswer(t, prepared, rule, 0, false)
					if len(items) != 0 || more {
						t.Fatalf("zero count projected items=%+v more=%t, want empty/false", items, more)
					}
				})
			}
			t.Run("canceled-after-count", func(t *testing.T) {
				rules, probe, in := lintPageCountFixture(rule, 2)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				probe.afterCount = cancel
				prepared, err := rules.Prepare(ctx, in, app.RuleQuery{Rule: &rule})
				if prepared != nil || !errors.Is(err, context.Canceled) {
					t.Fatalf("canceled zero count returned preparation=%v error=%v", prepared, err)
				}
				if probe.countCalls != 1 || probe.itemsCalls != 0 {
					t.Fatalf("canceled zero count performed Items: count calls=%d items calls=%d, want 1/0", probe.countCalls, probe.itemsCalls)
				}
			})
			for _, invalid := range []domain.Count{{Value: -1}, {Value: 10001, Capped: true}, {Capped: true}, {Value: 9999, Capped: true}, {Value: 10000}} {
				t.Run(fmt.Sprintf("invalid-count/%d-%t", invalid.Value, invalid.Capped), func(t *testing.T) {
					rules, probe, in := lintPageCountFixture(rule, 2)
					probe.count = invalid
					prepared, err := rules.Prepare(t.Context(), in, app.RuleQuery{Rule: &rule})
					if prepared != nil || err == nil || probe.countCalls != 1 || probe.itemsCalls != 0 {
						t.Fatalf("invalid count %+v bypassed validation: preparation=%v error=%v calls=%d/%d", invalid, prepared, err, probe.countCalls, probe.itemsCalls)
					}
				})
			}
			t.Run("count-error", func(t *testing.T) {
				rules, probe, in := lintPageCountFixture(rule, 2)
				probe.countErr = errors.New("independent Count failure")
				prepared, err := rules.Prepare(t.Context(), in, app.RuleQuery{Rule: &rule})
				if prepared != nil || !errors.Is(err, probe.countErr) || probe.countCalls != 1 || probe.itemsCalls != 0 {
					t.Fatalf("Count failure was suppressed: preparation=%v error=%v calls=%d/%d", prepared, err, probe.countCalls, probe.itemsCalls)
				}
			})
			t.Run("positive-count-items-error", func(t *testing.T) {
				rules, probe, in := lintPageCountFixture(rule, 2)
				probe.count.Value = 1
				probe.itemsErr = errors.New("independent Items failure")
				prepared, err := rules.Prepare(t.Context(), in, app.RuleQuery{Rule: &rule})
				if prepared != nil || !errors.Is(err, probe.itemsErr) || probe.countCalls != 1 || probe.itemsCalls != 1 {
					t.Fatalf("positive Count skipped/suppressed Items failure: preparation=%v error=%v calls=%d/%d", prepared, err, probe.countCalls, probe.itemsCalls)
				}
			})
			t.Run("positive-first-page", func(t *testing.T) {
				rules, probe, in := lintPageCountFixture(rule, 2)
				probe.count.Value, probe.ids = 1, []uuid.UUID{lintID(20)}
				prepared, err := rules.Prepare(t.Context(), in, app.RuleQuery{Rule: &rule, Limit: 1})
				if err != nil || prepared == nil {
					t.Fatalf("positive page returned preparation=%v error=%v", prepared, err)
				}
				items, more := lintAnswer(t, prepared, rule, 1, false)
				if probe.countCalls != 1 || probe.itemsCalls != 1 || probe.after != nil || probe.take != 2 ||
					len(items) != 1 || items[0].PageID == nil || *items[0].PageID != lintID(20) || more {
					t.Fatalf("positive first page lost Items: calls=%d/%d after=%v take=%d items=%+v more=%t", probe.countCalls, probe.itemsCalls, probe.after, probe.take, items, more)
				}
			})
			t.Run("capped-after-page", func(t *testing.T) {
				rules, probe, in := lintPageCountFixture(rule, 10001)
				probe.count = domain.Count{Value: 10000, Capped: true}
				probe.ids = []uuid.UUID{lintID(21), lintID(22)}
				after := domain.Position{ID: lintPtr(lintID(20))}
				prepared, err := rules.Prepare(t.Context(), in, app.RuleQuery{Rule: &rule, After: &after, Limit: 1})
				if err != nil || prepared == nil {
					t.Fatalf("capped page returned preparation=%v error=%v", prepared, err)
				}
				items, more := lintAnswer(t, prepared, rule, 10000, true)
				if probe.countCalls != 1 || probe.itemsCalls != 1 || probe.after == nil || *probe.after != lintID(20) || probe.take != 2 ||
					len(items) != 1 || items[0].PageID == nil || *items[0].PageID != lintID(21) || !more {
					t.Fatalf("capped After lost Items: calls=%d/%d after=%v take=%d items=%+v more=%t", probe.countCalls, probe.itemsCalls, probe.after, probe.take, items, more)
				}
			})
		})
	}
}
