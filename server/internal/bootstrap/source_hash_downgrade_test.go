package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/access"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/ratelimit"
)

// Downgrade clears the complete source area in the mode transaction. Both
// Forget's own partial failure and a later subscriber's failure must restore
// the mode, root and every hash, without publishing or logging success.
func TestSourceDowngradeClearsHashesAtomically(t *testing.T) {
	for _, failure := range []string{"Forget mid-update", "subscriber after Forget"} {
		t.Run(failure, func(t *testing.T) {
			logs := &syncBuffer{}
			logger := slog.New(slog.NewJSONHandler(logs, nil))
			var cfg config.Config
			keyFile := signingKeyFile(t)
			tm := newAcmeTeamLogging(t, "", "", func(c *config.Config) {
				c.Auth.JWT.PrivateKeyFile = keyFile
				cfg = *c
			}, transfer.Batch{}, logger)
			ctx := t.Context()
			wiki := tm.llmWiki(t, "Downgrade source hashes")
			source := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Source", "source bytes\r\n")
			tm.upload(t, "alice", wiki.id, wiki.raw, "source.txt", "attachment bytes")
			tm.upload(t, "alice", wiki.id, wiki.raw, "empty.bin", "")
			trash := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Trash", "deleted source")
			trashAsset := tm.upload(t, "alice", wiki.id, wiki.raw, "trash.bin", "deleted attachment")
			tm.send(t, nodeDeletion("alice", trash), http.StatusNoContent)
			tm.send(t, nodeDeletion("alice", trashAsset.ID), http.StatusNoContent)
			other := tm.llmWiki(t, "Untouched source hashes")
			tm.createPageWith(t, "alice", other.id, other.raw, "Other source", "source bytes\r\n")
			tm.upload(t, "alice", other.id, other.raw, "other.bin", "")
			marker := tm.createPage(t, "alice", wiki.id, "", "Marker")
			tm.settle(t, wiki.id, marker)
			stream := openStream(t, tm.base, "alice", tm.tokens["alice"])

			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND source_hash IS NOT NULL", wiki.id); n != 3 {
				t.Fatalf("downgrade fixture has %d live source hashes; want page, attachment and zero-byte attachment", n)
			}
			if n := count(t, tm.pool, `SELECT count(*) FROM nodes n JOIN asset_blobs b ON b.node_id = n.id
				WHERE n.notebook_id = $1 AND n.source_hash = b.sha256 AND b.byte_size = 0`, wiki.id); n != 1 {
				t.Fatalf("downgrade fixture has %d hashed empty attachments, want one", n)
			}
			assertSourceHashesMatchRebuild(t, ctx, tm.pool)
			checkPages(t, tm.pool)

			// Query again outside each failed transaction, including full mode/root
			// rows, deleted nodes, content/history and both notebooks' blobs.
			snapshot := func() map[string]string {
				t.Helper()
				out := map[string]string{}
				for _, table := range []string{"notebooks", "nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "asset_blobs"} {
					var digest string
					if err := tm.pool.QueryRow(ctx, "SELECT md5(coalesce(string_agg(row_to_json(x)::text, '' ORDER BY row_to_json(x)::text), '')) FROM "+table+" x").Scan(&digest); err != nil {
						t.Fatal(err)
					}
					out[table] = digest
				}
				return out
			}
			otherRows := func() []string {
				t.Helper()
				return queryStrings(t, tm.pool, `SELECT row_to_json(x)::text FROM (
					SELECT id, mode, source_root_id, updated_at, updated_by_id FROM notebooks WHERE id = $1
				) x UNION ALL SELECT row_to_json(x)::text FROM (
					SELECT id, parent_id, source_hash, deleted_at FROM nodes WHERE notebook_id = $1 ORDER BY id
				) x ORDER BY 1`, other.id)
			}
			before, keptOther := snapshot(), otherRows()
			successLogs := func() int {
				t.Helper()
				found := 0
				for _, line := range (&mcpApp{logs: logs}).logLines(t) {
					if line["msg"] == "notebook downgraded" && line["notebook_id"] == wiki.id {
						found++
					}
				}
				return found
			}
			downgrade := func() (int, string) { return tm.downgrade(t, "alice", wiki.id) }
			var resume func()
			var observer, successor *sourceHashModeObserver
			var order []string
			if failure == "Forget mid-update" {
				for _, sql := range []string{
					"CREATE SEQUENCE downgrade_hash_updates",
					`CREATE FUNCTION refuse_second_downgrade_hash() RETURNS trigger LANGUAGE plpgsql AS $$
					DECLARE current_mode text; seen bigint;
					BEGIN
						IF NEW.notebook_id = TG_ARGV[0]::uuid AND OLD.deleted_at IS NULL
							AND OLD.source_hash IS NOT NULL AND NEW.source_hash IS NULL THEN
							SELECT mode INTO current_mode FROM notebooks WHERE id = NEW.notebook_id;
							IF current_mode <> 'standard' THEN RAISE EXCEPTION 'Forget preceded SetMode'; END IF;
							seen := nextval('downgrade_hash_updates');
							IF seen = 2 THEN RAISE EXCEPTION 'source downgrade hash cleanup refused'; END IF;
						END IF;
						RETURN NEW;
					END $$`,
					fmt.Sprintf("CREATE TRIGGER refuse_second_downgrade_hash BEFORE UPDATE OF source_hash ON nodes "+
						"FOR EACH ROW EXECUTE FUNCTION refuse_second_downgrade_hash('%s')", uuid.MustParse(wiki.id)),
				} {
					if _, err := tm.pool.Exec(ctx, sql); err != nil {
						t.Fatal(err)
					}
				}
				resume = func() {
					t.Helper()
					if n := count(t, tm.pool, "SELECT last_value FROM downgrade_hash_updates"); n != 2 {
						t.Fatalf("Forget attempted %d hash updates; want first applied and second refused, irrespective of row order", n)
					}
					if !strings.Contains(logs.String(), "source downgrade hash cleanup refused") {
						t.Error("missing the injected SQL failure in the API error log")
					}
					if _, err := tm.pool.Exec(ctx, "DROP TRIGGER refuse_second_downgrade_hash ON nodes"); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				observer = &sourceHashModeObserver{pool: tm.pool, name: "observer", order: &order, err: errors.New("source mode subscriber refused")}
				successor = &sourceHashModeObserver{pool: tm.pool, name: "successor", order: &order}
				handler := sourceDowngradeHandler(t, tm, cfg, logger, observer, successor)
				downgrade = func() (int, string) {
					req := httptest.NewRequest(http.MethodPost, "/api/v0/notebooks/"+wiki.id+"/downgrade", nil)
					req.Header.Set("Authorization", "Bearer "+tm.tokens["alice"])
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, req)
					return rec.Code, rec.Body.String()
				}
				resume = func() {
					t.Helper()
					if !slices.Equal(order, []string{"observer"}) || observer.calls != 1 || successor.calls != 0 ||
						!observer.inTx || observer.mode != "standard" || observer.root != nil || observer.hashes != 0 ||
						observer.notebook != wiki.id {
						t.Fatalf("subscriber order %q, observer %+v, successor calls %d; want only observer in tx seeing standard/NULL root/zero hashes",
							order, observer, successor.calls)
					}
					observer.err = nil
					order = nil
				}
			}

			if status, body := downgrade(); status != http.StatusInternalServerError || !strings.Contains(body, `"code":"internal_error"`) {
				t.Fatalf("failed downgrade = %d %s; want 500 internal_error", status, body)
			}
			if after := snapshot(); !maps.Equal(before, after) {
				t.Errorf("failed downgrade changed persisted mode/root/hash/content/history/blob rows: before %v after %v", before, after)
			}
			if n := successLogs(); n != 0 {
				t.Errorf("failed downgrade logged success %d times, want none", n)
			}
			assertSourceHashesMatchRebuild(t, ctx, tm.pool)
			checkPages(t, tm.pool)
			checkAssets(t, tm.pool, tm.storage)
			// The marker is outside raw. Its next frame excludes an earlier mode
			// event, including a notification queued by a rolled-back subscriber.
			tm.quiet(t, wiki.id, marker, stream)
			resume()

			beforeRetry := snapshot()
			status, body := downgrade()
			if status != http.StatusOK {
				t.Fatalf("retry downgrade = %d %s; want 200", status, body)
			}
			var result createdNotebook
			decodeAnswer(t, body, &result)
			if result.Mode != "standard" || result.SourceRootID != nil {
				t.Errorf("retry returned %+v, want standard with no source root", result)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND source_hash IS NOT NULL", wiki.id); n != 0 {
				t.Errorf("successful downgrade left %d source hashes, want none", n)
			}
			afterRetry := snapshot()
			for _, table := range []string{"page_contents", "page_revisions", "changesets", "changeset_items", "asset_blobs"} {
				if afterRetry[table] != beforeRetry[table] {
					t.Errorf("downgrade changed %s, want source hashes and notebook mode only", table)
				}
			}
			if !slices.Equal(otherRows(), keptOther) {
				t.Error("downgrade changed the other wiki's mode, root or hashes")
			}
			if observer != nil && (!slices.Equal(order, []string{"observer", "successor"}) || observer.calls != 2 ||
				successor.calls != 1 || !successor.inTx || successor.mode != "standard" || successor.root != nil || successor.hashes != 0) {
				t.Errorf("retry subscriber order %q, observer calls %d, successor %+v; want both seeing the cleared transaction", order, observer.calls, successor)
			}
			if n := successLogs(); n != 1 {
				t.Errorf("successful retry logged success %d times, want one", n)
			}
			if event := stream.expect(t, "mode"); event.field(t, "notebook_id") != wiki.id {
				t.Errorf("success mode event = %s, want notebook %s", event.data, wiki.id)
			}
			tm.quiet(t, wiki.id, marker, stream)
			stream.close()
			assertSourceHashesMatchRebuild(t, ctx, tm.pool)

			// Old source trash and ordinary writes remain unhashed after downgrade.
			tm.restored(t, "alice", trash)
			tm.restored(t, "alice", trashAsset.ID)
			tm.send(t, contentWrite("alice", source, "ordinary after downgrade", 1, ""), http.StatusOK)
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND source_hash IS NOT NULL", wiki.id); n != 0 {
				t.Errorf("standard restore/write recreated %d source hashes, want none", n)
			}
			assertSourceHashesMatchRebuild(t, ctx, tm.pool)
			checkPages(t, tm.pool)
			checkAssets(t, tm.pool, tm.storage)
		})
	}
}

type sourceHashModeObserver struct {
	pool     *pgxpool.Pool
	name     string
	order    *[]string
	err      error
	calls    int
	inTx     bool
	mode     string
	root     *string
	hashes   int
	notebook string
}

func (s *sourceHashModeObserver) ModeChanged(ctx context.Context, change notebook.ModeChange) error {
	*s.order = append(*s.order, s.name)
	s.calls++
	s.inTx, s.notebook = postgres.InTx(ctx), change.NotebookID.String()
	if err := postgres.DB(ctx, s.pool).QueryRow(ctx, `SELECT mode, source_root_id::text,
		(SELECT count(*) FROM nodes WHERE notebook_id = $1 AND source_hash IS NOT NULL)
		FROM notebooks WHERE id = $1`, change.NotebookID).Scan(&s.mode, &s.root, &s.hashes); err != nil {
		return err
	}
	return s.err
}

// The second case keeps production notebookDeps, auth, transaction, source
// bridge and existing stream subscriber. Only its additional subscribers are
// test-owned; the default complete serve route is exercised by the first case.
func sourceDowngradeHandler(t *testing.T, tm acmeTeam, cfg config.Config, logger *slog.Logger,
	subscribers ...notebook.ModeSubscriber,
) http.Handler {
	t.Helper()
	pem, err := readSigningKey(cfg.Auth.JWT.PrivateKeyFile)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := identity.LoadSigningKeys(pem, logger)
	if err != nil {
		t.Fatal(err)
	}
	limiter := ratelimit.New(time.Now)
	ident, err := identity.New(identityDeps(cfg, tm.pool, logger, limiter, keys, signupPolicy{open: true}))
	if err != nil {
		t.Fatal(err)
	}
	authorizer := access.New(access.Deps{Memberships: workspace.NewMemberships(tm.pool), Notebooks: notebookFacts{notebook.NewFacts(tm.pool)}})
	deps := notebookDeps(cfg, tm.pool, logger, authorizer, []notebook.Seeder{sourceDowngradeUnusedSeeder{}})
	deps.ModeSubscribers = append(deps.ModeSubscribers, subscribers...)
	api, err := httpserver.NewAPI(apiConfig(cfg, logger, limiter, ident.Authenticator(), nil, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	router := httpserver.NewRouter(logger)
	notebook.New(deps).Register(router, api)
	return router
}

// This handler serves downgrade only; creation must never use a fake template.
type sourceDowngradeUnusedSeeder struct{}

func (sourceDowngradeUnusedSeeder) NotebookCreated(context.Context, notebook.NotebookCreation) (uuid.UUID, error) {
	return uuid.UUID{}, errors.New("creation is outside the downgrade test handler")
}
