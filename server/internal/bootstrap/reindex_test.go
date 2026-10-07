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
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
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

// A notebook's workspace slug is "" when the notebook or its workspace was
// deleted, or there is no such notebook: no read failed, and its pages are
// told by their ids with no error logged (M6 Codex review, fix check
// B4-N1).
func TestTheWorkspaceSlugOfANotebookOrWorkspaceDeletedIsNone(t *testing.T) {
	pool := connect(t, pgtest.NewDatabase(t))
	ctx := context.Background()
	user, ws, nb := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@example.com', 'x', 'Alice', now(), now())`,
			[]any{user}},
		{`INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())`,
			[]any{ws, user}},
		{`INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Notes', $3, $3, now(), now())`,
			[]any{nb, ws, user}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name, stmt string
		nb         uuid.UUID
		want       string
	}{
		{"live", "", nb, "acme"},
		{"no such notebook", "", uuid.NewV7(), ""},
		{"the notebook deleted", "UPDATE notebooks SET deleted_at = now()", nb, ""},
		{"the workspace deleted", "UPDATE notebooks SET deleted_at = NULL; UPDATE workspaces SET deleted_at = now()", nb, ""},
	} {
		if tt.stmt != "" {
			if _, err := pool.Exec(ctx, tt.stmt); err != nil {
				t.Fatal(err)
			}
		}
		if slug, err := workspaceSlug(ctx, pool, tt.nb); slug != tt.want || err != nil {
			t.Errorf("%s: workspaceSlug = %q, %v; want %q", tt.name, slug, err, tt.want)
		}
	}
}
