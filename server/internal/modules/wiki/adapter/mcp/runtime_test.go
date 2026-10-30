package mcpadapter

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type lintData struct {
	mode      domain.Mode
	rows      []domain.LintRuleResult
	positions map[domain.LintRule][]domain.LintPosition
}

func lintLongPath(t *testing.T, depth, number int, asset bool) string {
	t.Helper()
	if depth < 1 || depth > page.MaxDepth {
		t.Fatal("fixture exceeds stored page depth")
	}
	segment := strings.Repeat("&", 255)
	parts := make([]string, depth)
	for i := range parts {
		parts[i] = segment
	}
	suffix := fmt.Sprintf("%03x", number)
	if asset {
		parts = append(parts, strings.Repeat("&", 248)+suffix+".bin")
	} else {
		parts[len(parts)-1] = strings.Repeat("&", 252) + suffix + ".md"
	}
	path := strings.Join(parts, "/")
	if err := page.NewNodePaths(nil).Check("path", path); err != nil {
		t.Fatalf("fixture is not a legal tool path: %v", err)
	}
	if depth == 10 && ((!asset && len(path) != 2562) || (asset && len(path) != 2815)) {
		t.Fatalf("stored maximum path length = %d", len(path))
	}
	return path
}

func lintDataSet(t *testing.T, depth, n int) lintData {
	t.Helper()
	d := lintData{mode: domain.ModeLLMWiki, positions: make(map[domain.LintRule][]domain.LintPosition)}
	rules := []domain.LintRule{domain.RuleWanted, domain.RuleOrphans, domain.RuleAmbiguous, domain.RuleInvalidFrontmatter, domain.RuleMissingType, domain.RuleMissingSummary, domain.RuleUndigestedSources, domain.RuleInvalidSources}
	for r, rule := range rules {
		begin, err := linking.EncodeLintCursor(linking.CursorBinding{NotebookID: lintID(10)}, linking.Rule(rule), linking.Position{Begin: true})
		if err != nil {
			t.Fatal(err)
		}
		row := domain.LintRuleResult{Rule: rule, Count: domain.LintCount{Value: int64(n)}, StartCursor: &begin, Items: make([]domain.LintItem, 0, n)}
		for i := 0; i < n; i++ {
			var id uuid.UUID
			binary.BigEndian.PutUint32(id[12:], uint32(r*1000+i+1))
			if rule == domain.RuleOrphans && i == 0 {
				id = uuid.UUID{}
			}
			raw := strings.Repeat("&", 255) + "/" + fmt.Sprintf("target-%03d", i)
			hash := sha256.Sum256([]byte(raw))
			digest := hex.EncodeToString(hash[:])
			position := domain.LintPosition{ID: &id}
			item := domain.LintItem{Rule: rule, PageID: &id, ItemKey: "p:" + id.String()}
			path := lintLongPath(t, depth, r*100+i, false)
			item.Path = &path
			switch rule {
			case domain.RuleWanted:
				item.PageID, item.Path = nil, nil
				item.ItemKey = "w:" + digest
				item.Target = raw[:256]
				item.TargetCut = true
				item.Pages = int64(n - i)
				position = domain.LintPosition{Pages: item.Pages, Prefix: []byte(raw[:256]), Hash: digest}
			case domain.RuleAmbiguous:
				item.ItemKey = "a:" + id.String() + ":" + digest
				item.Target = raw[:256]
				item.TargetCut = true
				position.Hash = digest
			case domain.RuleInvalidSources:
				var header [9]byte
				header[0] = 'L'
				binary.BigEndian.PutUint64(header[1:], uint64(len(raw)))
				full := append(header[:], []byte(raw)...)
				sum := sha256.Sum256(full)
				digest = hex.EncodeToString(sum[:])
				item.ItemKey = "s:" + id.String() + ":" + digest
				item.Target = raw[:256]
				item.TargetCut = true
				position.Hash = digest
			case domain.RuleUndigestedSources:
				item.PageID = nil
				item.NodeID = &id
				item.Kind = domain.NodeAsset
				item.Bytes = int64(9000 + i)
				item.CreatedAt = lintTime().Add(time.Duration(i) * time.Second)
				item.ItemKey = "n:" + id.String()
				path = lintLongPath(t, depth, r*100+i, true)
				item.Path = &path
				position.CreatedAt = item.CreatedAt
			}
			cursor, err := (lintCodec{}).Encode(app.LintScope{NotebookID: lintID(10)}, rule, position)
			if err != nil {
				t.Fatal(err)
			}
			item.Cursor = cursor
			row.Items = append(row.Items, item)
			d.positions[rule] = append(d.positions[rule], position)
		}
		if n == 0 {
			row.StartCursor = nil
		}
		d.rows = append(d.rows, row)
	}
	return d
}

type lintCodec struct{}
type lintDecoded struct{ cursor linking.LintCursor }

func (lintCodec) Decode(raw string) (app.DecodedCursor, error) {
	c, err := linking.DecodeLintCursor(raw)
	return lintDecoded{c}, err
}
func (lintCodec) Encode(scope app.LintScope, rule domain.LintRule, p domain.LintPosition) (string, error) {
	return linking.EncodeLintCursor(linking.CursorBinding{NotebookID: scope.NotebookID}, linking.Rule(rule), linking.Position{Begin: p.Begin, Pages: p.Pages, Prefix: p.Prefix, Hash: p.Hash, ID: p.ID, CreatedAt: p.CreatedAt})
}
func (c lintDecoded) PositionFor(scope app.LintScope, rule domain.LintRule) (domain.LintPosition, error) {
	p, err := c.cursor.PositionFor(linking.CursorBinding{NotebookID: scope.NotebookID}, linking.Rule(rule))
	return domain.LintPosition{Begin: p.Begin, Pages: p.Pages, Prefix: p.Prefix, Hash: p.Hash, ID: p.ID, CreatedAt: p.CreatedAt}, err
}

type lintRuntime struct {
	data                            lintData
	lint                            *app.Lint
	mu                              sync.Mutex
	prepares, authorizes, snapshots int
	gate                            <-chan struct{}
	entered                         chan struct{}
	enter                           sync.Once
	joined                          chan struct{}
	events                          []app.RunEvent
}

func newLintRuntime(t *testing.T, data lintData) *lintRuntime {
	t.Helper()
	f := &lintRuntime{data: data, entered: make(chan struct{}), joined: make(chan struct{}, 8)}
	runs := app.NewRuns(1, 2*time.Second, slog.New(slog.DiscardHandler), func(e app.RunEvent) {
		f.mu.Lock()
		f.events = append(f.events, e)
		f.mu.Unlock()
		if e.Kind == app.RunJoined {
			f.joined <- struct{}{}
		}
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runs.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	f.lint = app.NewLint(app.LintDeps{Notebooks: f, Sources: lintRuntimeSources{f}, Nodes: lintRuntimeNodes{f}, Rules: f, Paths: f, Cursors: lintCodec{}, Snapshots: f, Authorizer: f, Clock: f, Runs: runs})
	return f
}

func (f *lintRuntime) WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	return lintID(3), id == lintID(10), ctx.Err()
}
func (f *lintRuntime) StateOf(ctx context.Context, _ uuid.UUID) (app.NotebookState, bool, error) {
	return app.NotebookState{Mode: f.data.mode, SourceRootID: lintID(20)}, true, ctx.Err()
}
func (f *lintRuntime) Now() time.Time { return lintTime() }
func (f *lintRuntime) Authorize(ctx context.Context, actor shared.Actor, action shared.Action, target shared.Target) (shared.Grant, error) {
	if actor.UserID != lintID(1) || action != domain.ActionLintRead || target.NotebookID != lintID(10) || target.WorkspaceID != lintID(3) {
		return shared.Grant{}, errors.New("wrong runtime authorization")
	}
	f.mu.Lock()
	f.authorizes++
	f.mu.Unlock()
	return shared.Grant{}, ctx.Err()
}

type lintSnapshotKey struct{}

func (f *lintRuntime) WithinSnapshot(ctx context.Context, fn func(context.Context) error) error {
	f.mu.Lock()
	f.snapshots++
	f.mu.Unlock()
	f.enter.Do(func() { close(f.entered) })
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fn(context.WithValue(ctx, lintSnapshotKey{}, true))
}

type lintRuntimeSources struct{ f *lintRuntime }

func (s lintRuntimeSources) Nodes(ctx context.Context, id, root uuid.UUID) ([]app.SourceNode, error) {
	if id != lintID(10) || root != lintID(20) || ctx.Value(lintSnapshotKey{}) != true {
		return nil, errors.New("source scope escaped snapshot")
	}
	var out []app.SourceNode
	for _, row := range s.f.data.rows {
		if row.Rule == domain.RuleUndigestedSources {
			for _, item := range row.Items {
				out = append(out, app.SourceNode{ID: *item.NodeID, Kind: item.Kind, CreatedAt: item.CreatedAt})
			}
		}
	}
	return out, ctx.Err()
}

type lintRuntimeNodes struct{ f *lintRuntime }

func (s lintRuntimeNodes) Nodes(ctx context.Context, id uuid.UUID) ([]app.CompactNode, error) {
	if id != lintID(10) || ctx.Value(lintSnapshotKey{}) != true {
		return nil, errors.New("nodes escaped snapshot")
	}
	var out []app.CompactNode
	for _, row := range s.f.data.rows {
		for _, item := range row.Items {
			if item.PageID != nil {
				out = append(out, app.CompactNode{ID: *item.PageID, Kind: domain.NodePage})
			} else if item.NodeID != nil {
				out = append(out, app.CompactNode{ID: *item.NodeID, Kind: item.Kind, CreatedAt: item.CreatedAt})
			}
		}
	}
	return out, ctx.Err()
}
func (f *lintRuntime) Paths(ctx context.Context, id uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	if id != lintID(10) || ctx.Value(lintSnapshotKey{}) != true {
		return nil, errors.New("paths escaped snapshot")
	}
	out := make(map[uuid.UUID]string)
	for _, row := range f.data.rows {
		for _, item := range row.Items {
			key := item.PageID
			if key == nil {
				key = item.NodeID
			}
			if key != nil && slices.Contains(ids, *key) {
				out[*key] = *item.Path
			}
		}
	}
	return out, ctx.Err()
}
func (f *lintRuntime) PrepareRules(ctx context.Context, input app.ReadInput, query app.RuleQuery) (app.PreparedRules, error) {
	if input.NotebookID != lintID(10) || input.Explicit || ctx.Value(lintSnapshotKey{}) != true {
		return nil, errors.New("rules escaped implicit snapshot")
	}
	f.mu.Lock()
	f.prepares++
	f.mu.Unlock()
	return lintPrepared{data: f.data, query: query}, ctx.Err()
}

type lintPrepared struct {
	data  lintData
	query app.RuleQuery
}

func (p lintPrepared) StalePages() int64               { return 7 }
func (p lintPrepared) Timings() app.PreparationTimings { return app.PreparationTimings{} }
func (p lintPrepared) RuleCount(ctx context.Context, rule domain.LintRule) (domain.LintCount, error) {
	for _, row := range p.data.rows {
		if row.Rule == rule {
			return row.Count, ctx.Err()
		}
	}
	return domain.LintCount{}, errors.New("unknown fixture rule")
}
func (p lintPrepared) RuleItems(ctx context.Context, rule domain.LintRule) ([]domain.LintItem, bool, error) {
	for _, row := range p.data.rows {
		if row.Rule != rule {
			continue
		}
		start := 0
		if p.query.After != nil && !p.query.After.Begin {
			start = -1
			for i, pos := range p.data.positions[rule] {
				if reflect.DeepEqual(pos, *p.query.After) {
					start = i + 1
					break
				}
			}
			if start < 0 {
				return nil, false, errors.New("cursor does not name fixture position")
			}
		}
		limit := p.query.Limit
		if p.query.Rule == nil {
			limit = 5
		}
		end := min(start+limit, len(row.Items))
		return slices.Clone(row.Items[start:end]), end < len(row.Items), ctx.Err()
	}
	return nil, false, errors.New("unknown fixture rule")
}
