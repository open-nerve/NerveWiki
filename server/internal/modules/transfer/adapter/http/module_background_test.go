package httpadapter_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The real module must pass its configured bounds to the start, just as
// it does to the worker. Refuse after observing the callback, before the
// start reaches its database or queue.
func TestTheModulePassesBatchBoundsToRevertStart(t *testing.T) {
	for _, tt := range []struct {
		name  string
		batch transfer.Batch
		steps []transfer.JobStep
		want  [][2]int
	}{
		{"nodes and Joined", transfer.Batch{Nodes: 3},
			[]transfer.JobStep{{Nodes: 1}, {Nodes: 1, Joined: true}, {Nodes: 1}, {Nodes: 1}}, [][2]int{{0, 3}, {3, 4}}},
		{"bytes", transfer.Batch{Bytes: 10},
			[]transfer.JobStep{{Nodes: 1, Bytes: 6}, {Nodes: 1, Bytes: 5}, {Nodes: 1, Bytes: 5}}, [][2]int{{0, 1}, {1, 3}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			probe := &moduleRevertTree{steps: tt.steps}
			logger := slog.New(slog.DiscardHandler)
			module := transfer.New(transfer.Deps{Tree: probe, Clock: fixedClock{}, Logger: logger, Batch: tt.batch,
				DownloadKey: []byte("key"), RestoreAction: restoreAction, RevertAction: revertAction,
				ImportMaxBytes: importMaxBytes, MinRate: 64 << 10})
			router := httpserver.NewRouter(logger)
			module.Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: fakeAuth{}}))
			request := httptest.NewRequest(http.MethodPost, revertsPath(),
				strings.NewReader(`{"changeset_id":"0199a2b4-0000-7000-8000-000000000300"}`))
			request.Header.Set("Authorization", "Bearer bobpat")
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden || probe.calls != 1 || !probe.present || !slices.Equal(probe.cuts, tt.want) {
				t.Errorf("response=%d %s, callback present=%t, calls=%d, cuts=%v; want 403, present, one call, %v",
					response.Code, response.Body.String(), probe.present, probe.calls, probe.cuts, tt.want)
			}
		})
	}
}

type moduleRevertTree struct {
	tree
	steps   []transfer.JobStep
	present bool
	calls   int
	cuts    [][2]int
}

func (t *moduleRevertTree) CheckRevert(_ context.Context, _, _ uuid.UUID, _ *string, _ func(context.Context) error,
	batches transfer.JobBatches) (string, error) {
	t.calls++
	t.present = batches != nil
	if batches != nil {
		t.cuts = batches(t.steps)
	}
	return "", shared.Forbidden()
}
