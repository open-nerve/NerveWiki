package httpadapter_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func lintID(n byte) uuid.UUID { var id uuid.UUID; id[15] = n; return id }

func lintTime() time.Time { return time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC) }

type lintAuth struct{}

func (lintAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	if token != "session" {
		return nil, "", shared.Unauthenticated()
	}
	return shared.WithActor(ctx, shared.Actor{UserID: lintID(1), SessionID: lintID(2)}), token, nil
}

type lintSnapshotKey struct{}

type lintFixture struct {
	mu        sync.Mutex
	calls     []string
	mode      domain.Mode
	items     map[domain.LintRule][]domain.LintItem
	positions map[domain.LintRule][]domain.LintPosition
	counts    map[domain.LintRule]domain.LintCount
	query     app.RuleQuery
	handler   lintHTTP
	budget    time.Duration
	block     bool
}

func (f *lintFixture) note(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *lintFixture) trace() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func (f *lintFixture) WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	f.note("workspace")
	return lintID(3), id == lintID(10) || id == lintID(11), ctx.Err()
}

func (f *lintFixture) StateOf(ctx context.Context, _ uuid.UUID) (app.NotebookState, bool, error) {
	f.note("state")
	return app.NotebookState{Mode: f.mode, SourceRootID: lintID(20)}, true, ctx.Err()
}

func (f *lintFixture) Authorize(ctx context.Context, actor shared.Actor, action shared.Action, target shared.Target) (shared.Grant, error) {
	f.note("authorize")
	if actor.UserID != lintID(1) || action != domain.ActionLintRead || target.WorkspaceID != lintID(3) {
		return shared.Grant{}, errors.New("wrong authorization input")
	}
	if target.NotebookID != lintID(10) {
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{}, ctx.Err()
}

func (f *lintFixture) WithinSnapshot(ctx context.Context, fn func(context.Context) error) error {
	f.note("snapshot")
	return fn(context.WithValue(ctx, lintSnapshotKey{}, true))
}

func (f *lintFixture) Now() time.Time { return lintTime() }

type lintNodes struct{ f *lintFixture }

func (n lintNodes) Nodes(ctx context.Context, id uuid.UUID) ([]app.CompactNode, error) {
	n.f.note("nodes")
	if id != lintID(10) || ctx.Value(lintSnapshotKey{}) != true {
		return nil, errors.New("nodes outside the authorized snapshot")
	}
	return []app.CompactNode{{ID: lintID(0), Kind: domain.NodePage}, {ID: lintID(21), Kind: domain.NodePage}}, ctx.Err()
}

type lintSources struct{ f *lintFixture }

func (s lintSources) Nodes(ctx context.Context, id, root uuid.UUID) ([]app.SourceNode, error) {
	s.f.note("sources")
	if id != lintID(10) || root != lintID(20) || ctx.Value(lintSnapshotKey{}) != true {
		return nil, errors.New("sources outside the authorized snapshot")
	}
	return nil, ctx.Err()
}

func (f *lintFixture) Paths(ctx context.Context, id uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	f.note("paths")
	if id != lintID(10) || ctx.Value(lintSnapshotKey{}) != true {
		return nil, errors.New("paths outside the authorized snapshot")
	}
	paths := make(map[uuid.UUID]string, len(ids))
	for _, id := range ids {
		paths[id] = "private/title/" + id.String()
	}
	return paths, ctx.Err()
}

type lintPrepared struct {
	items     map[domain.LintRule][]domain.LintItem
	counts    map[domain.LintRule]domain.LintCount
	positions map[domain.LintRule][]domain.LintPosition
	query     app.RuleQuery
}

func (p lintPrepared) StalePages() int64               { return 2 }
func (p lintPrepared) Timings() app.PreparationTimings { return app.PreparationTimings{} }
func (p lintPrepared) RuleCount(ctx context.Context, rule domain.LintRule) (domain.LintCount, error) {
	return p.counts[rule], ctx.Err()
}
func (p lintPrepared) RuleItems(ctx context.Context, rule domain.LintRule) ([]domain.LintItem, bool, error) {
	items := p.items[rule]
	start := 0
	if p.query.After != nil && !p.query.After.Begin {
		start = -1
		for i, position := range p.positions[rule] {
			if reflect.DeepEqual(position, *p.query.After) {
				start = i + 1
				break
			}
		}
		if start < 0 {
			return nil, false, errors.New("cursor did not name a fixture position")
		}
	}
	limit := p.query.Limit
	if p.query.Rule == nil {
		limit = 5
	}
	end := min(start+limit, len(items))
	return slices.Clone(items[start:end]), end < len(items), ctx.Err()
}

func (f *lintFixture) PrepareRules(ctx context.Context, input app.ReadInput, query app.RuleQuery) (app.PreparedRules, error) {
	f.note("prepare")
	if input.NotebookID != lintID(10) || input.Explicit || ctx.Value(lintSnapshotKey{}) != true {
		return nil, errors.New("rules outside the implicit authorized snapshot")
	}
	f.mu.Lock()
	f.query = query
	f.mu.Unlock()
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return lintPrepared{items: f.items, counts: f.counts, positions: f.positions, query: query}, ctx.Err()
}

type lintCursors struct{ f *lintFixture }
type lintDecoded struct {
	cursor linking.LintCursor
	f      *lintFixture
}

func (c lintCursors) Decode(raw string) (app.DecodedCursor, error) {
	if c.f != nil {
		c.f.note("decode")
	}
	cursor, err := linking.DecodeLintCursor(raw)
	return lintDecoded{cursor: cursor, f: c.f}, err
}

func (lintCursors) Encode(scope app.LintScope, rule domain.LintRule, position domain.LintPosition) (string, error) {
	return linking.EncodeLintCursor(linking.CursorBinding{NotebookID: scope.NotebookID}, linking.Rule(rule), linking.Position{
		Begin: position.Begin, Pages: position.Pages, Prefix: position.Prefix, Hash: position.Hash, ID: position.ID, CreatedAt: position.CreatedAt,
	})
}

func (d lintDecoded) PositionFor(scope app.LintScope, rule domain.LintRule) (domain.LintPosition, error) {
	if d.f != nil {
		d.f.note("bind")
	}
	p, err := d.cursor.PositionFor(linking.CursorBinding{NotebookID: scope.NotebookID}, linking.Rule(rule))
	return domain.LintPosition{Begin: p.Begin, Pages: p.Pages, Prefix: p.Prefix, Hash: p.Hash, ID: p.ID, CreatedAt: p.CreatedAt}, err
}

func newLintFixture(t *testing.T, options ...func(*lintFixture)) *lintFixture {
	t.Helper()
	f := &lintFixture{mode: domain.ModeStandard, items: make(map[domain.LintRule][]domain.LintItem), counts: make(map[domain.LintRule]domain.LintCount), positions: make(map[domain.LintRule][]domain.LintPosition), budget: time.Second}
	page := lintID(0)
	cursor, err := (lintCursors{}).Encode(app.LintScope{NotebookID: lintID(10)}, domain.RuleOrphans, domain.LintPosition{ID: &page})
	if err != nil {
		t.Fatal(err)
	}
	f.items[domain.RuleOrphans] = []domain.LintItem{{Rule: domain.RuleOrphans, ItemKey: "p:00000000-0000-0000-0000-000000000000", Cursor: cursor, PageID: &page}}
	f.counts[domain.RuleOrphans] = domain.LintCount{Value: 1}
	f.positions[domain.RuleOrphans] = []domain.LintPosition{{ID: &page}}
	for _, option := range options {
		option(f)
	}
	log := slog.New(slog.DiscardHandler)
	runs := app.NewRuns(1, f.budget, log, nil)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runs.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	lint := app.NewLint(app.LintDeps{Notebooks: f, Sources: lintSources{f}, Nodes: lintNodes{f}, Rules: f,
		Cursors: lintCursors{f: f}, Paths: f, Snapshots: f, Authorizer: f, Clock: f, Runs: runs})
	router := httpserver.NewRouter(log)
	api := httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: lintAuth{}})
	httpadapter.Register(router, api, lint)
	f.handler = lintHTTP{Handler: router, contract: apitest.Load(t)}
	return f
}

type lintHTTP struct {
	http.Handler
	contract *apitest.Contract
}

func lintCall(t *testing.T, handler lintHTTP, path string) (*http.Response, []byte) {
	t.Helper()
	return lintRequest(t, handler, path, "session")
}

func lintRequest(t *testing.T, handler lintHTTP, path, token string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	response := rec.Result()
	handler.contract.CheckResponse(t, req, response)
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	return response, body
}

const lintPath = "/api/v0/notebooks/00000000-0000-0000-0000-00000000000a/lint"
