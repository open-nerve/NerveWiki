package notebook_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The activity extension point on a real database, the module wired as
// bootstrap wires it (M3 design 8): the sources stand in for M4's pages and
// M7's attachments.

// activitySource answers what it holds of the notebooks asked for, or
// fails.
type activitySource struct {
	of   map[uuid.UUID]notebook.NotebookActivity
	fail bool
}

func (s activitySource) NotebookActivities(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]notebook.NotebookActivity, error) {
	if s.fail {
		return nil, errors.New("the source failed")
	}
	out := make(map[uuid.UUID]notebook.NotebookActivity)
	for _, id := range ids {
		if a, ok := s.of[id]; ok {
			out[id] = a
		}
	}
	return out, nil
}

// anyProfiles answers a profile for every account.
type anyProfiles struct{}

func (anyProfiles) MemberProfiles(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]notebook.Profile, error) {
	out := make(map[uuid.UUID]notebook.Profile, len(ids))
	for _, id := range ids {
		out[id] = notebook.Profile{DisplayName: "Alice", Email: "alice@corp.com"}
	}
	return out, nil
}

// The sources the module is given reach the ownerless list: the bytes
// summed, the latest of their writes and the notebook's own update; with
// none, its size is 0 and its activity its update; a failing source fails
// the list.
func TestTheActivitySourcesReachTheOwnerlessList(t *testing.T) {
	updated := testNow().Add(-time.Hour)
	written := testNow().Add(-30 * time.Minute)
	for _, c := range []struct {
		name     string
		sources  func(f fixture) []notebook.NotebookActivitySource
		code     int
		bytes    int64
		activity time.Time
	}{
		{"none", func(fixture) []notebook.NotebookActivitySource { return nil }, http.StatusOK, 0, updated},
		{"two", func(f fixture) []notebook.NotebookActivitySource {
			return []notebook.NotebookActivitySource{
				activitySource{of: map[uuid.UUID]notebook.NotebookActivity{f.eng: {Bytes: 100, LastWriteAt: &written}}},
				activitySource{of: map[uuid.UUID]notebook.NotebookActivity{f.eng: {Bytes: 24}, f.ops: {Bytes: 7}}},
			}
		}, http.StatusOK, 124, written},
		{"a failing one", func(fixture) []notebook.NotebookActivitySource {
			return []notebook.NotebookActivitySource{activitySource{fail: true}}
		}, http.StatusInternalServerError, 0, time.Time{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.orphanEng(t)
			router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
			notebook.New(notebook.Deps{
				Pool: f.pool, Tx: postgres.NewTxManager(f.pool, 5*time.Second), Clock: &tickingClock{},
				Logger: slog.New(slog.DiscardHandler), Authorizer: adminsAuthorizer{facts: notebook.NewFacts(f.pool)},
				Workspaces: sqlWorkspaces{f.pool}, Profiles: anyProfiles{}, ActivitySources: c.sources(f),
			}).Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: tokenAuth{}}))

			req := httptest.NewRequest(http.MethodGet, "/api/v0/workspaces/acme/ownerless-notebooks", nil)
			req.Header.Set("Authorization", "Bearer "+f.bob.String())
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != c.code {
				t.Fatalf("GET = %d %s, want %d", rec.Code, rec.Body, c.code)
			}
			if c.code != http.StatusOK {
				return
			}
			var page struct {
				Data []struct {
					ID             uuid.UUID `json:"id"`
					SizeBytes      int64     `json:"size_bytes"`
					LastActivityAt time.Time `json:"last_activity_at"`
				} `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			if len(page.Data) != 1 || page.Data[0].ID != f.eng || page.Data[0].SizeBytes != c.bytes ||
				!page.Data[0].LastActivityAt.Equal(c.activity) {
				t.Errorf("listed %s; want eng alone, %d bytes, its activity at %v", rec.Body, c.bytes, c.activity)
			}
		})
	}
}
