package bootstrap

import (
	"context"
	"errors"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

var errWikiLintValue = errors.New("wiki: invalid internal lint projection")

// These adapters keep the transaction context and convert only module-root
// values. No adapter starts a transaction, resolves paths early or reads bodies.
type wikiLintNotebooks struct{ notebooks notebook.Notebooks }

func (n wikiLintNotebooks) WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	return n.notebooks.WorkspaceOf(ctx, id)
}

func (n wikiLintNotebooks) StateOf(ctx context.Context, id uuid.UUID) (wiki.NotebookState, bool, error) {
	state, found, err := n.notebooks.StateOf(ctx, id)
	if err != nil || !found {
		return wiki.NotebookState{}, found, err
	}
	var mode wiki.Mode
	switch state.Mode {
	case notebook.ModeStandard:
		mode = wiki.ModeStandard
	case notebook.ModeLLMWiki:
		mode = wiki.ModeLLMWiki
	default:
		return wiki.NotebookState{}, false, errWikiLintValue
	}
	return wiki.NotebookState{Mode: mode, SourceRootID: state.SourceRootID}, true, nil
}

type wikiSources struct{ sources page.SourceArea }

func (s wikiSources) LintData(ctx context.Context, notebookID, root uuid.UUID) (wiki.SourceData, error) {
	data, err := s.sources.LintData(ctx, notebookID, root)
	if err != nil {
		return wiki.SourceData{}, err
	}
	out := wiki.SourceData{Sources: make([]wiki.SourceNode, len(data.Sources))}
	for i, node := range data.Sources {
		kind, err := wikiPageKind(node.Kind)
		if err != nil {
			return wiki.SourceData{}, err
		}
		out.Sources[i] = wiki.SourceNode{ID: node.ID, Kind: kind, CreatedAt: node.CreatedAt, PageBytes: node.PageBytes}
	}
	out.Nodes, err = wikiCompactNodes(data.Nodes)
	if err != nil {
		return wiki.SourceData{}, err
	}
	return out, nil
}

type wikiNodes struct{ nodes page.LintNodes }

func (n wikiNodes) Nodes(ctx context.Context, notebookID uuid.UUID) ([]wiki.CompactNode, error) {
	nodes, err := n.nodes.Nodes(ctx, notebookID)
	if err != nil {
		return nil, err
	}
	return wikiCompactNodes(nodes)
}

func wikiCompactNodes(nodes []page.LintNode) ([]wiki.CompactNode, error) {
	out := make([]wiki.CompactNode, len(nodes))
	for i, node := range nodes {
		kind, err := wikiPageKind(node.Kind)
		if err != nil {
			return nil, err
		}
		out[i] = wiki.CompactNode{ID: node.ID, ParentID: wikiCopy(node.ParentID), Kind: kind,
			NameKey: node.NameKey, Linkable: node.Linkable, CreatedAt: node.CreatedAt, PageBytes: node.PageBytes}
	}
	return out, nil
}

func wikiPageKind(kind page.NodeKind) (wiki.NodeKind, error) {
	switch kind {
	case page.KindPage:
		return wiki.NodePage, nil
	case page.KindAsset:
		return wiki.NodeAsset, nil
	}
	return "", errWikiLintValue
}

type wikiPaths struct{ paths page.NodePaths }

func (p wikiPaths) Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	return p.paths.Paths(ctx, notebookID, ids)
}

// The linking consumer chooses the final unique asset IDs. Missing or negative
// sizes remain visible to its checks; this adapter never substitutes a zero.
type wikiAssetSizes struct{ infos asset.AttachmentInfos }

func (a wikiAssetSizes) BytesOf(ctx context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	infos, err := a.infos.Of(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int64, len(infos))
	for id, info := range infos {
		out[id] = info.Bytes
	}
	return out, nil
}

type wikiRules struct{ rules *linking.LintRules }

func (r wikiRules) PrepareRules(ctx context.Context, in wiki.ReadInput, query wiki.RuleQuery) (wiki.PreparedRules, error) {
	mode, err := wikiLinkingMode(in.EffectiveMode)
	if err != nil {
		return nil, err
	}
	input := linking.ReadInput{NotebookID: in.NotebookID, EffectiveMode: mode, Explicit: in.Explicit,
		AreaRootID: wikiCopy(in.AreaRootID), SourceIDs: slices.Clone(in.SourceIDs), Nodes: make([]linking.CompactNode, len(in.Nodes))}
	for i, node := range in.Nodes {
		var kind linking.NodeKind
		switch node.Kind {
		case wiki.NodePage:
			kind = linking.NodePage
		case wiki.NodeAsset:
			kind = linking.NodeAsset
		default:
			return nil, errWikiLintValue
		}
		input.Nodes[i] = linking.CompactNode{ID: node.ID, ParentID: wikiCopy(node.ParentID), Kind: kind,
			NameKey: node.NameKey, Linkable: node.Linkable, CreatedAt: node.CreatedAt, PageBytes: node.PageBytes}
	}
	q := linking.RuleQuery{Limit: query.Limit}
	if query.Rule != nil {
		rule := linking.Rule(*query.Rule)
		q.Rule = &rule
	}
	if query.After != nil {
		after := wikiLinkingPosition(*query.After)
		q.After = &after
	}
	prepared, err := r.rules.Prepare(ctx, input, q)
	if err != nil {
		return nil, err
	}
	return wikiPrepared{prepared: prepared}, nil
}

type wikiPrepared struct{ prepared *linking.PreparedRules }

func (p wikiPrepared) StalePages() int64 { return p.prepared.StalePages() }

func (p wikiPrepared) RuleCount(ctx context.Context, rule wiki.LintRule) (wiki.LintCount, error) {
	count, err := p.prepared.RuleCount(ctx, linking.Rule(rule))
	return wiki.LintCount{Value: count.Value, Capped: count.Capped}, err
}

func (p wikiPrepared) RuleItems(ctx context.Context, rule wiki.LintRule) ([]wiki.LintItem, bool, error) {
	items, more, err := p.prepared.RuleItems(ctx, linking.Rule(rule))
	if err != nil {
		return nil, false, err
	}
	out := make([]wiki.LintItem, len(items))
	for i, item := range items {
		out[i] = wiki.LintItem{Rule: wiki.LintRule(item.Rule), ItemKey: item.ItemKey, Cursor: item.Cursor,
			PageID: wikiCopy(item.PageID), NodeID: wikiCopy(item.NodeID), Target: item.Target, TargetCut: item.TargetCut,
			Pages: item.Pages, NotLink: item.NotLink, Kind: wiki.NodeKind(item.Kind), Bytes: item.Bytes, CreatedAt: item.CreatedAt}
	}
	return out, more, nil
}

func (p wikiPrepared) Timings() wiki.PreparationTimings {
	timings := p.prepared.Timings()
	out := wiki.PreparationTimings{SharedSources: wikiCopy(timings.SharedSources), Rules: make([]wiki.RuleTiming, len(timings.Rules))}
	for i, timing := range timings.Rules {
		out.Rules[i] = wiki.RuleTiming{Rule: wiki.LintRule(timing.Rule), Duration: timing.Duration}
	}
	return out
}

type wikiCursors struct{}

func (wikiCursors) Decode(raw string) (wiki.DecodedCursor, error) {
	cursor, err := linking.DecodeLintCursor(raw)
	if err != nil {
		return nil, err
	}
	return wikiCursor{cursor: cursor}, nil
}

func (wikiCursors) Encode(scope wiki.LintScope, rule wiki.LintRule, position wiki.LintPosition) (string, error) {
	binding, err := wikiCursorBinding(scope)
	if err != nil {
		return "", err
	}
	return linking.EncodeLintCursor(binding, linking.Rule(rule), wikiLinkingPosition(position))
}

type wikiCursor struct{ cursor linking.LintCursor }

func (c wikiCursor) PositionFor(scope wiki.LintScope, rule wiki.LintRule) (wiki.LintPosition, error) {
	binding, err := wikiCursorBinding(scope)
	if err != nil {
		return wiki.LintPosition{}, err
	}
	position, err := c.cursor.PositionFor(binding, linking.Rule(rule))
	if err != nil {
		return wiki.LintPosition{}, err
	}
	return wiki.LintPosition{Begin: position.Begin, Pages: position.Pages, Prefix: slices.Clone(position.Prefix),
		Hash: position.Hash, ID: wikiCopy(position.ID), CreatedAt: position.CreatedAt}, nil
}

func wikiCursorBinding(scope wiki.LintScope) (linking.CursorBinding, error) {
	binding := linking.CursorBinding{NotebookID: scope.NotebookID, Explicit: scope.Contract != nil, AreaRootID: wikiCopy(scope.AreaRootID)}
	if scope.Contract != nil {
		mode, err := wikiLinkingMode(*scope.Contract)
		if err != nil {
			return linking.CursorBinding{}, err
		}
		binding.Mode = mode
	}
	return binding, nil
}

func wikiLinkingMode(mode wiki.Mode) (linking.LintMode, error) {
	switch mode {
	case wiki.ModeStandard:
		return linking.ModeStandard, nil
	case wiki.ModeLLMWiki:
		return linking.ModeLLM, nil
	}
	return 0, errWikiLintValue
}

func wikiLinkingPosition(position wiki.LintPosition) linking.Position {
	return linking.Position{Begin: position.Begin, Pages: position.Pages, Prefix: slices.Clone(position.Prefix),
		Hash: position.Hash, ID: wikiCopy(position.ID), CreatedAt: position.CreatedAt}
}

func wikiCopy[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
