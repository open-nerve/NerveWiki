package app

import (
	"context"
	"errors"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Lint coordinates one snapshot and each caller's own access checks.
type Lint struct{ deps LintDeps }

// NewLint uses the composition's shared run owner. It creates no worker or
// process cache; each invocation still authorizes its own caller twice.
func NewLint(deps LintDeps) *Lint { return &Lint{deps: deps} }

var errLintInvariant = errors.New("wiki lint: invalid internal read contract")

// Execute validates pure cursor shape before access and leaves waiting and
// worker lifetimes to Runs. No failed request returns a partial result.
func (l *Lint) Execute(ctx context.Context, scope LintScope, query LintQuery) (domain.LintResult, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return domain.LintResult{}, err
	}
	scope.Contract, scope.AreaRootID = lintCopy(scope.Contract), lintCopy(scope.AreaRootID)
	query.Rule, query.Limit, query.Cursor = lintCopy(query.Rule), lintCopy(query.Limit), lintCopy(query.Cursor)
	if err := lintScopeValid(scope); err != nil {
		return domain.LintResult{}, err
	}
	var rule domain.LintRule
	var known bool
	if query.Rule != nil {
		rule, known = domain.ParseLintRule(*query.Rule)
	}
	var after *domain.LintPosition
	if query.Cursor != nil {
		decoded, err := l.deps.Cursors.Decode(*query.Cursor)
		if err != nil {
			return domain.LintResult{}, err
		}
		if decoded == nil {
			return domain.LintResult{}, errLintInvariant
		}
		if known {
			pos, err := decoded.PositionFor(scope, rule)
			if err != nil {
				return domain.LintResult{}, err
			}
			pos.ID, pos.Prefix = lintCopy(pos.ID), slices.Clone(pos.Prefix)
			after = &pos
		}
	}
	if err := l.authorize(ctx, actor, scope.NotebookID); err != nil {
		return domain.LintResult{}, err
	}
	key := RunKey{NotebookID: scope.NotebookID, Kind: domain.LintSummary}
	readQuery := RuleQuery{}
	if query.Rule == nil {
		var fields []shared.FieldError
		if query.Limit != nil {
			fields = append(fields, shared.FieldError{Field: "limit", Code: shared.FieldNotAllowed, Message: "requires a rule"})
		}
		if query.Cursor != nil {
			fields = append(fields, shared.FieldError{Field: "cursor", Code: shared.FieldNotAllowed, Message: "requires a rule"})
		}
		if len(fields) != 0 {
			return domain.LintResult{}, shared.Invalid(fields...)
		}
	} else {
		if !known {
			return domain.LintResult{}, domain.ErrLintRuleNotFound
		}
		if lintExclusive(rule) {
			state, found, err := l.deps.Notebooks.StateOf(ctx, scope.NotebookID)
			if err != nil {
				return domain.LintResult{}, err
			}
			if !found {
				return domain.LintResult{}, domain.ErrNotebookNotFound
			}
			mode, _, err := lintEffective(scope, state)
			if err != nil {
				return domain.LintResult{}, err
			}
			if mode != domain.ModeLLMWiki {
				return domain.LintResult{}, domain.ErrNotLLMWiki
			}
		}
		limit, err := shared.PageSize(query.Limit)
		if err != nil {
			return domain.LintResult{}, err
		}
		key.Kind, key.Rule, key.Limit = domain.LintItems, rule, limit
		readQuery = RuleQuery{Rule: &rule, After: after, Limit: limit}
		if after != nil {
			key.CursorCanonical, err = l.deps.Cursors.Encode(scope, rule, *after)
			if err != nil {
				return domain.LintResult{}, err
			}
		}
	}
	if scope.Contract != nil {
		key.Explicit, key.Contract = true, *scope.Contract
		if scope.AreaRootID != nil {
			key.HasRoot, key.RootID = true, *scope.AreaRootID
		}
	}
	result, err := l.deps.Runs.Do(ctx, key, func(worker context.Context) (RunOutput, error) {
		return l.compute(worker, scope, readQuery, key.Kind)
	})
	if err != nil {
		return domain.LintResult{}, err
	}
	if err := l.authorize(ctx, actor, scope.NotebookID); err != nil {
		return domain.LintResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.LintResult{}, err
	}
	return domain.CloneLintResult(result), nil
}

func (l *Lint) authorize(ctx context.Context, actor shared.Actor, notebook uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	workspace, found, err := l.deps.Notebooks.WorkspaceOf(ctx, notebook)
	if err != nil {
		return err
	}
	if !found {
		return domain.ErrNotebookNotFound
	}
	_, err = l.deps.Authorizer.Authorize(ctx, actor, domain.ActionLintRead, shared.Target{WorkspaceID: workspace, NotebookID: notebook})
	if errors.Is(err, shared.ErrNotVisible) {
		return domain.ErrNotebookNotFound
	}
	return err
}

func lintScopeValid(scope LintScope) error {
	if scope.Contract == nil {
		if scope.AreaRootID != nil {
			return errLintInvariant
		}
		return nil
	}
	switch *scope.Contract {
	case domain.ModeStandard:
		if scope.AreaRootID == nil {
			return nil
		}
	case domain.ModeLLMWiki:
		if scope.AreaRootID != nil {
			return nil
		}
	}
	return errLintInvariant
}

func lintEffective(scope LintScope, state NotebookState) (domain.Mode, *uuid.UUID, error) {
	if scope.Contract != nil {
		return *scope.Contract, lintCopy(scope.AreaRootID), nil
	}
	switch state.Mode {
	case domain.ModeStandard:
		return state.Mode, nil, nil
	case domain.ModeLLMWiki:
		return state.Mode, lintCopy(&state.SourceRootID), nil
	}
	return "", nil, errLintInvariant
}

func lintExclusive(rule domain.LintRule) bool {
	return rule == domain.RuleMissingType || rule == domain.RuleMissingSummary ||
		rule == domain.RuleUndigestedSources || rule == domain.RuleInvalidSources
}

func lintCopy[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (l *Lint) compute(ctx context.Context, scope LintScope, query RuleQuery, kind domain.LintQueryKind) (RunOutput, error) {
	var output RunOutput
	var result domain.LintResult
	err := l.deps.Snapshots.WithinSnapshot(ctx, func(snapshot context.Context) error {
		state, found, err := l.deps.Notebooks.StateOf(snapshot, scope.NotebookID)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrNotebookNotFound
		}
		computedAt := l.deps.Clock.Now().UTC()
		mode, root, err := lintEffective(scope, state)
		if err != nil {
			return err
		}
		output.Stats.Mode = mode
		if query.Rule != nil && lintExclusive(*query.Rule) && mode != domain.ModeLLMWiki {
			return domain.ErrNotLLMWiki
		}
		input := ReadInput{NotebookID: scope.NotebookID, EffectiveMode: mode, Explicit: scope.Contract != nil, AreaRootID: root}
		if mode == domain.ModeLLMWiki {
			sources, err := l.deps.Sources.Nodes(snapshot, scope.NotebookID, *root)
			if err != nil {
				return err
			}
			input.SourceIDs = make([]uuid.UUID, len(sources))
			for i, source := range sources {
				if err := snapshot.Err(); err != nil {
					return err
				}
				input.SourceIDs[i] = source.ID
			}
		}
		nodes, err := l.deps.Nodes.Nodes(snapshot, scope.NotebookID)
		if err != nil {
			return err
		}
		input.Nodes = slices.Clone(nodes)
		for i := range input.Nodes {
			input.Nodes[i].ParentID = lintCopy(input.Nodes[i].ParentID)
		}
		started := time.Now()
		prepared, err := l.deps.Rules.PrepareRules(snapshot, input, query)
		elapsed := time.Since(started)
		output.Stats.PrepareElapsed = &elapsed
		if err != nil {
			return err
		}
		if prepared == nil {
			return errLintInvariant
		}
		stale := prepared.StalePages()
		if stale < 0 {
			return errLintInvariant
		}
		output.Stats.StalePages = &stale
		result = domain.LintResult{Kind: kind, Mode: mode, ComputedAt: computedAt, StalePages: stale}
		if err := l.project(snapshot, scope, query, prepared, &result, &output.Stats); err != nil {
			return err
		}
		return snapshot.Err()
	})
	if err != nil {
		return output, err
	}
	if err := ctx.Err(); err != nil {
		return output, err
	}
	output.Result = result
	return output, nil
}
