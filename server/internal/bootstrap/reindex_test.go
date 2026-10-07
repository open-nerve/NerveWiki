package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
)

// The pages whose titles would share a key are told by their addresses,
// of the workspace's slug; when the slug is not read, by their ids, the
// error logged, and reindex goes on to the next notebook; when ctx is done,
// it stops (M6 Codex review, fix check B3-M3).
func TestClashLineTellsThePagesByIDWhenTheSlugIsNotRead(t *testing.T) {
	nb, a, b := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	clashes := []linking.Clash{{a, b}}
	lost := errors.New("connection lost")
	byID := fmt.Sprintf("notebook %s: not reindexed: the pages whose titles would share a key: %s, %s", nb, a, b)
	done, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tt := range []struct {
		name   string
		ctx    context.Context
		slug   string
		err    error
		line   string
		logged bool
		fails  bool
	}{
		{"the slug read", context.Background(), "acme", nil, fmt.Sprintf(
			"notebook %[1]s: not reindexed: the pages whose titles would share a key: /acme/notebooks/%[1]s/pages/%[2]s, /acme/notebooks/%[1]s/pages/%[3]s",
			nb, a, b), false, false},
		{"the workspace deleted", context.Background(), "", nil, byID, false, false},
		{"the slug not read", context.Background(), "", lost, byID, true, false},
		{"ctx done", done, "", lost, "", false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			slugOf := func(_ context.Context, id uuid.UUID) (string, error) {
				if id != nb {
					t.Errorf("slug of %s, want of the notebook %s", id, nb)
				}
				return tt.slug, tt.err
			}
			line, err := clashLine(tt.ctx, logger, slugOf, nb, clashes)
			if line != tt.line || (err != nil) != tt.fails || (tt.fails && !errors.Is(err, lost)) {
				t.Errorf("clashLine = %q, %v; want %q, failing %v", line, err, tt.line, tt.fails)
			}
			if got := strings.Contains(logs.String(), `"notebook_id":"`+nb.String()+`"`) && strings.Contains(logs.String(), lost.Error()); got != tt.logged {
				t.Errorf("logs = %s, want the error logged %v", logs.String(), tt.logged)
			}
		})
	}
}
