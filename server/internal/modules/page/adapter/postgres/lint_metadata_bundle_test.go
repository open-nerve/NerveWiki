package postgresadapter_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

type lintBundleTrace struct{ calls atomic.Int64 }

func (p *lintBundleTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(q.SQL, "-- name: LintNodeMetadata ") {
		p.calls.Add(1)
	}
	return ctx
}

func (*lintBundleTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestLintMetadataBundle(t *testing.T) {
	t.Run("complete-live-notebook-and-strict-area", func(t *testing.T) {
		f := newFixture(t)
		shelf := f.page(t, f.eng, nil, "Shelf", 0)
		root := f.page(t, f.eng, &shelf.ID, "Raw", 0)
		child := f.page(t, f.eng, &root.ID, "Member", 0)
		f.write(t, child.ID, "body🙂", 2, now())
		asset := f.asset(t, f.eng, &root.ID, "scan.pdf", 1)
		outside := f.page(t, f.eng, nil, "Outside", 1)
		f.page(t, f.ops, nil, "Foreign", 0)
		got, err := page.NewSourceArea(f.pool).LintData(t.Context(), f.eng, root.ID)
		want := []uuid.UUID{child.ID, asset.ID}
		slices.SortFunc(want, uuid.UUID.Compare)
		if err != nil || !slices.Equal(lintIDs(got.Sources), want) {
			t.Fatalf("bundle lost complete strict sources: ids=%v error=%v, want %v", lintIDs(got.Sources), err, want)
		}
		wantNodes := map[uuid.UUID]bool{shelf.ID: true, root.ID: true, child.ID: true, asset.ID: true, outside.ID: true}
		if len(got.Nodes) != len(wantNodes) {
			t.Fatalf("bundle lost complete live metadata: nodes=%d, want %d", len(got.Nodes), len(wantNodes))
		}
		for _, node := range got.Nodes {
			if !wantNodes[node.ID] || node.CreatedAt != now() || node.NameKey == "" {
				t.Fatalf("bundle changed live metadata: %+v", node)
			}
			delete(wantNodes, node.ID)
			if node.ID == child.ID && (node.PageBytes != int64(len("body🙂")) || node.ParentID == nil || *node.ParentID != root.ID) {
				t.Fatalf("bundle changed page bytes or parent: %+v", node)
			}
			if node.ID == asset.ID && (node.Kind != page.KindAsset || !node.Linkable) {
				t.Fatalf("bundle lost original asset linkability: %+v", node)
			}
		}
	})
	t.Run("complete-hashless-area-past-ten-thousand", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "Raw", 0)
		lintSeedPages(t, f, f.eng, &root.ID, 91, 10017)
		got, err := page.NewSourceArea(f.pool).LintData(t.Context(), f.eng, root.ID)
		if err != nil || len(got.Sources) != 10017 || len(got.Nodes) != 10018 {
			t.Fatalf("bundle truncated complete hashless area: sources=%d nodes=%d error=%v", len(got.Sources), len(got.Nodes), err)
		}
		seen := map[uuid.UUID]bool{}
		for _, source := range got.Sources {
			if source.ID == root.ID || seen[source.ID] || source.PageBytes != 0 || source.Kind != page.KindPage {
				t.Fatalf("bundle changed strict hashless member: %+v", source)
			}
			seen[source.ID] = true
		}
	})
	for _, defect := range []string{"foreign-root", "missing-ancestor", "cycle", "asset-parent", "depth", "missing-content", "cancelled"} {
		t.Run(defect, func(t *testing.T) {
			f := newFixture(t)
			root := f.page(t, f.eng, nil, "Raw", 0)
			child := f.page(t, f.eng, &root.ID, "Child", 0)
			leaf := f.page(t, f.eng, &child.ID, "Leaf", 0)
			ctx := t.Context()
			rootID := root.ID
			switch defect {
			case "foreign-root":
				rootID = f.page(t, f.ops, nil, "Foreign", 0).ID
			case "missing-ancestor":
				f.exec(t, "UPDATE nodes SET deleted_at=$2 WHERE id=$1", child.ID, now())
				f.exec(t, "UPDATE page_contents SET deleted_at=$2 WHERE node_id=$1", child.ID, now())
			case "cycle":
				f.exec(t, "UPDATE nodes SET parent_id=$2 WHERE id=$1", child.ID, leaf.ID)
			case "asset-parent":
				asset := f.asset(t, f.eng, &root.ID, "file.bin", 1)
				f.exec(t, "UPDATE nodes SET parent_id=$2 WHERE id=$1", leaf.ID, asset.ID)
			case "depth":
				at := leaf
				for depth := 4; depth <= domain.MaxDepth; depth++ {
					at = f.page(t, f.eng, &at.ID, fmt.Sprintf("Depth %d", depth), 0)
				}
				asset := f.asset(t, f.eng, &at.ID, "legal.bin", 0)
				got, err := page.NewSourceArea(f.pool).LintData(ctx, f.eng, leaf.ID)
				if err != nil || !slices.Contains(lintIDs(got.Sources), asset.ID) {
					t.Fatalf("bundle rejected existing legal depth and asset level: %v", err)
				}
				f.page(t, f.eng, &at.ID, "Too deep", 1)
			case "missing-content":
				f.exec(t, "DELETE FROM page_contents WHERE node_id=$1", leaf.ID)
			case "cancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			got, err := page.NewSourceArea(f.pool).LintData(ctx, f.eng, rootID)
			if err == nil || !reflect.DeepEqual(got, page.SourceData{}) {
				t.Fatalf("bundle exposed partial data for %s: sources=%d nodes=%d error=%v", defect, len(got.Sources), len(got.Nodes), err)
			}
			if defect == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("bundle lost cancellation: %v", err)
			}
			if defect == "missing-content" && err.Error() != fmt.Sprintf("lint metadata: live page has no content (notebook_id=%s node_id=%s)", f.eng, leaf.ID) {
				t.Fatalf("bundle changed redacted missing-content error: %v", err)
			}
		})
	}
	t.Run("fresh-read-and-private-nodes", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "Raw", 0)
		child := f.page(t, f.eng, &root.ID, "Child", 0)
		reader := page.NewSourceArea(f.pool)
		first, err := reader.LintData(t.Context(), f.eng, root.ID)
		if err != nil {
			t.Fatal(err)
		}
		for i := range first.Nodes {
			first.Nodes[i].NameKey = "caller changed"
			if first.Nodes[i].ParentID != nil {
				*first.Nodes[i].ParentID = uuid.UUID{}
			}
		}
		first.Sources[0].ID = uuid.UUID{}
		added := f.page(t, f.eng, &root.ID, "Added", 1)
		second, err := reader.LintData(t.Context(), f.eng, root.ID)
		want := []uuid.UUID{child.ID, added.ID}
		slices.SortFunc(want, uuid.UUID.Compare)
		if err != nil || !slices.Equal(lintIDs(second.Sources), want) || len(second.Nodes) != 3 {
			t.Fatalf("bundle reused an old or caller-mutated read: %+v/%v", second, err)
		}
		for _, n := range second.Nodes {
			if n.NameKey == "caller changed" || n.ID != root.ID && (n.ParentID == nil || *n.ParentID != root.ID) {
				t.Fatalf("bundle retained caller metadata: %+v", n)
			}
		}
	})
	t.Run("one-planned-read-without-body-toast", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "Raw", 0)
		child := f.page(t, f.eng, &root.ID, "Large", 0)
		random := rand.New(rand.NewPCG(20261030, 2))
		data := make([]byte, 3<<18)
		for i := range data {
			data[i] = byte(random.Uint32())
		}
		body := base64.StdEncoding.EncodeToString(data)
		f.write(t, child.ID, body, 2, now())
		trace := &lintBundleTrace{}
		cfg := f.pool.Config()
		cfg.MaxConns, cfg.ConnConfig.Tracer = 1, trace
		pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		const toastBlocks = `SELECT coalesce(sum(pg_stat_get_xact_blocks_fetched(x.r)),0)::bigint FROM (
            SELECT reltoastrelid AS r FROM pg_class WHERE oid='page_contents'::regclass
            UNION ALL SELECT i.indexrelid FROM pg_index i JOIN pg_class c ON c.reltoastrelid=i.indrelid
            WHERE c.oid='page_contents'::regclass) x`
		err = postgres.NewTxManager(pool, 5*time.Second).WithinSnapshot(t.Context(), func(ctx context.Context) error {
			db := postgres.DB(ctx, pool)
			var before, metadata, positive int64
			if err := db.QueryRow(ctx, toastBlocks).Scan(&before); err != nil {
				return err
			}
			got, err := page.NewSourceArea(pool).LintData(ctx, f.eng, root.ID)
			if err != nil || len(got.Sources) != 1 || got.Sources[0].ID != child.ID || got.Sources[0].PageBytes != int64(len(body)) || len(got.Nodes) != 2 {
				t.Fatalf("nonempty bundle metadata control failed: %+v/%v", got, err)
			}
			if trace.calls.Load() != 1 {
				t.Fatalf("bundle metadata read twice: calls=%d, want 1", trace.calls.Load())
			}
			if err := db.QueryRow(ctx, toastBlocks).Scan(&metadata); err != nil {
				return err
			}
			content, err := f.s.PageContent(ctx, child.ID)
			if err != nil || content.Content != body {
				t.Fatalf("full-body positive control failed: %v", err)
			}
			if err := db.QueryRow(ctx, toastBlocks).Scan(&positive); err != nil {
				return err
			}
			t.Logf("bundle TOAST blocks: metadata=%d full-body-positive=%d", metadata-before, positive-metadata)
			if metadata != before || positive-metadata < 100 {
				t.Fatalf("bundle body TOAST work: metadata=%d positive=%d; want 0/>=100", metadata-before, positive-metadata)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := postgresadapter.New(pool).FindNodeIn(t.Context(), f.eng, root.ID); err != nil {
			t.Fatal(err)
		}
		cached := pgtest.CachedStatements(t, pool)
		if slices.Contains(cached, "LintNodeMetadata") || !slices.Contains(cached, "FindNodeIn") {
			t.Fatalf("bundle bypassed Planned or lost cached positive control: %v", cached)
		}
	})
}
