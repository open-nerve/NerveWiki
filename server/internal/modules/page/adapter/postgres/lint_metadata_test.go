package postgresadapter_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	linkdomain "github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func lintIDs(nodes []page.SourceNode) []uuid.UUID {
	out := make([]uuid.UUID, len(nodes))
	for i, node := range nodes {
		out[i] = node.ID
	}
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}

func lintExpectSources(t *testing.T, f fixture, root uuid.UUID, want ...uuid.UUID) []page.SourceNode {
	t.Helper()
	nodes, err := page.NewSourceArea(f.pool).Nodes(t.Context(), f.eng, root)
	slices.SortFunc(want, uuid.UUID.Compare)
	if err != nil || !slices.Equal(lintIDs(nodes), want) {
		t.Fatalf("strict source IDs=%v error=%v; want exactly %v", lintIDs(nodes), err, want)
	}
	return nodes
}

func lintSeedPages(t *testing.T, f fixture, notebook uuid.UUID, parent *uuid.UUID, namespace, count int) {
	t.Helper()
	f.exec(t, `INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,
        created_by_id,updated_by_id,created_at,updated_at)
        SELECT ('7a110000-0000-7000-'||lpad(to_hex($4::int),4,'0')||'-'||lpad(to_hex(k),12,'0'))::uuid,
        $1,$2,'page','Page '||$4::int||' '||k,'page '||$4::int||' '||k,k,$3,$3,$6,$6
        FROM generate_series(1,$5::int) k`, notebook, parent, f.alice, namespace, count, now())
	f.exec(t, `INSERT INTO page_contents(node_id,revision,content,content_hash,byte_size,updated_by_id,updated_at)
        SELECT id,1,'',sha256(''::bytea),0,$2,$3 FROM nodes
        WHERE notebook_id=$1 AND id::text LIKE $4`, notebook, f.alice, now(), fmt.Sprintf("7a110000-0000-7000-%04x-%%", namespace))
}

func TestLintSourceAreaIsComplete(t *testing.T) {
	f := newFixture(t)
	shelf := f.page(t, f.eng, nil, "Shelf", 0)
	root := f.page(t, f.eng, &shelf.ID, "Proposed source root", 0)
	empty := f.page(t, f.eng, &root.ID, "Empty source", 0)
	filled := f.page(t, f.eng, &empty.ID, "Filled source", 0)
	f.write(t, filled.ID, "bytes🙂", 2, now().Add(time.Hour))
	f.exec(t, "UPDATE nodes SET updated_at=$2,source_hash=$3 WHERE id=$1", filled.ID, now().Add(2*time.Hour), sourceSum("bytes🙂"))
	asset := f.asset(t, f.eng, &root.ID, "scan.pdf", 1)
	trash := f.page(t, f.eng, &root.ID, "Trashed source", 2)
	f.trashed(t, now(), 1, 0, 1, trash.ID)
	outside := f.page(t, f.eng, nil, "Outside", 1)
	foreign := f.page(t, f.ops, nil, "Other notebook", 0)
	for _, name := range []string{"root", "empty-page", "trash", "explicit-standard-root"} {
		t.Run(name, func(t *testing.T) {
			nodes := lintExpectSources(t, f, root.ID, empty.ID, filled.ID, asset.ID)
			for _, node := range nodes {
				if node.CreatedAt != now() {
					t.Errorf("created_at=%v, want original %v", node.CreatedAt, now())
				}
				if node.ID == filled.ID && (node.Kind != page.KindPage || node.PageBytes != int64(len("bytes🙂"))) {
					t.Errorf("filled metadata=%+v", node)
				}
				if node.ID == asset.ID && node.Kind != page.KindAsset {
					t.Errorf("attachment lost its kind: %+v", node)
				}
			}
		})
	}
	t.Run("empty-area", func(t *testing.T) {
		lintExpectSources(t, f, outside.ID)
	})
	t.Run("invalid-root", func(t *testing.T) {
		for _, id := range []uuid.UUID{uuid.NewV7(), foreign.ID, trash.ID, asset.ID} {
			if got, err := page.NewSourceArea(f.pool).Nodes(t.Context(), f.eng, id); err == nil || got != nil {
				t.Errorf("invalid root %s returned %+v/%v; want no partial result and an error", id, got, err)
			}
		}
	})
	t.Run("large", func(t *testing.T) {
		large := newFixture(t)
		r := large.page(t, large.eng, nil, "Raw", 0)
		lintSeedPages(t, large, large.eng, &r.ID, 1, 10017)
		large.page(t, large.ops, nil, "Hidden", 0)
		got, err := page.NewSourceArea(large.pool).Nodes(t.Context(), large.eng, r.ID)
		if err != nil || len(got) != 10017 {
			t.Fatalf("complete hashless source area=%d/%v; want all 10017", len(got), err)
		}
		seen := map[uuid.UUID]bool{}
		for _, node := range got {
			if seen[node.ID] || node.ID == r.ID || node.Kind != page.KindPage || node.PageBytes != 0 {
				t.Fatalf("unexpected/duplicate source member %+v", node)
			}
			seen[node.ID] = true
		}
	})
	for _, defect := range []string{"missing-ancestor", "cycle", "asset-parent", "depth-exhaustion"} {
		t.Run(defect, func(t *testing.T) {
			broken := newFixture(t)
			r := broken.page(t, broken.eng, nil, "Raw", 0)
			a := broken.page(t, broken.eng, &r.ID, "A", 0)
			b := broken.page(t, broken.eng, &a.ID, "B", 0)
			switch defect {
			case "missing-ancestor":
				// A live child whose ancestor is not live respects the FK but
				// must not silently disappear from the metadata's live graph.
				broken.exec(t, "UPDATE nodes SET deleted_at=$2 WHERE id=$1", a.ID, now())
				broken.exec(t, "UPDATE page_contents SET deleted_at=$2 WHERE node_id=$1", a.ID, now())
			case "cycle":
				broken.exec(t, "UPDATE nodes SET parent_id=$2 WHERE id=$1", a.ID, b.ID)
			case "asset-parent":
				attachment := broken.asset(t, broken.eng, &r.ID, "file.bin", 1)
				broken.exec(t, "UPDATE nodes SET parent_id=$2 WHERE id=$1", b.ID, attachment.ID)
			case "depth-exhaustion":
				at := b
				for depth := 4; depth <= domain.MaxDepth; depth++ {
					at = broken.page(t, broken.eng, &at.ID, fmt.Sprintf("Depth %d", depth), 0)
				}
				attachment := broken.asset(t, broken.eng, &at.ID, "deep.bin", 0)
				got, err := page.NewSourceArea(broken.pool).Nodes(t.Context(), broken.eng, b.ID)
				if err != nil || !slices.Contains(lintIDs(got), attachment.ID) {
					t.Fatalf("legal notebook-relative page depth plus asset level rejected: %v", err)
				}
				broken.page(t, broken.eng, &at.ID, "Too deep", 1)
			}
			got, err := page.NewSourceArea(broken.pool).Nodes(t.Context(), broken.eng, r.ID)
			if err == nil || got != nil {
				t.Fatalf("%s returned partial membership %+v/%v", defect, got, err)
			}
		})
	}
}

func TestLintMetadataKeepsItsScope(t *testing.T) {
	f := newFixture(t)
	raw := f.page(t, f.eng, nil, "Raw", 0)
	empty := f.page(t, f.eng, &raw.ID, "Empty", 0)
	agents := f.page(t, f.eng, nil, "AGENTS.md", 1)
	stale := f.page(t, f.eng, nil, "Stale", 2)
	f.write(t, stale.ID, "old", 2, now().Add(time.Hour))
	f.exec(t, "UPDATE nodes SET updated_at=$2 WHERE id=$1", stale.ID, now().Add(2*time.Hour))
	trash := f.page(t, f.eng, nil, "Trash", 3)
	f.trashed(t, now(), 1, 0, 1, trash.ID)
	f.page(t, f.ops, nil, "Invisible", 0)
	want := map[uuid.UUID]bool{raw.ID: true, empty.ID: true, agents.ID: true, stale.ID: true}
	originalNames := map[uuid.UUID]string{}
	for i, tt := range []struct {
		name     string
		linkable bool
	}{{"without-extension", false}, {".hidden", false}, {"trailing.", false}, {".md", false},
		{"a.b.c", true}, {".a.b", true}, {"雪.图", true}, {".雪", false}, {"a..", false},
		{"文．件", false}, {"é.md", true}, {"a.雪", true}, {" .hidden", true}, {"a. ", true}} {
		// Persist boundary/legacy originals directly: the current writer's
		// stricter title validation is not the reader's Linkable predicate.
		n := f.asset(t, f.eng, &empty.ID, fmt.Sprintf("fixture-%d.bin", i), float64(i))
		f.exec(t, "UPDATE nodes SET name=$2,name_key=$3 WHERE id=$1", n.ID, tt.name, shared.TitleKey(tt.name))
		want[n.ID] = tt.linkable
		originalNames[n.ID] = tt.name
	}
	reader := page.NewLintNodes(f.pool)
	got, err := reader.Nodes(t.Context(), f.eng)
	if err != nil || len(got) != len(want) {
		t.Fatalf("live compact metadata count=%d/%v; want %d including root/source/AGENTS/stale/empty", len(got), err, len(want))
	}
	seen := map[uuid.UUID]bool{}
	for _, node := range got {
		linkable, ok := want[node.ID]
		if !ok || seen[node.ID] || node.Linkable != linkable || node.CreatedAt != now() {
			t.Errorf("compact fact=%+v; expected member/linkable=%t", node, linkable)
		}
		seen[node.ID] = true
		if name, asset := originalNames[node.ID]; asset {
			if node.Kind != page.KindAsset || node.ParentID == nil || *node.ParentID != empty.ID || node.NameKey != shared.TitleKey(name) {
				t.Errorf("compact attachment identity/parent/key=%+v", node)
			}
			candidate := linkdomain.Node{Asset: true, Path: []linkdomain.Step{{Name: name}}}
			if node.Linkable != candidate.Linkable() {
				t.Errorf("stored original %q: metadata Linkable=%t differs from real linking Node.Linkable", name, node.Linkable)
			}
		}
		if node.ID == empty.ID && (node.Kind != page.KindPage || node.ParentID == nil || *node.ParentID != raw.ID) ||
			node.ID == raw.ID && node.ParentID != nil {
			t.Errorf("compact page parent=%+v", node)
		}
		if node.ID == agents.ID && node.NameKey != "agents.md" || node.ID == stale.ID && node.PageBytes != 3 {
			t.Errorf("original metadata changed: %+v", node)
		}
	}
	t.Run("snapshot", func(t *testing.T) {
		source := page.NewSourceArea(f.pool)
		err := postgres.NewTxManager(f.pool, 5*time.Second).WithinSnapshot(t.Context(), func(ctx context.Context) error {
			first, err := reader.Nodes(ctx, f.eng)
			if err != nil {
				return err
			}
			firstSources, err := source.Nodes(ctx, f.eng, raw.ID)
			if err != nil || len(firstSources) != len(originalNames)+1 {
				t.Fatalf("snapshot source positive control=%d/%v", len(firstSources), err)
			}
			f.write(t, stale.ID, "a newer body", 3, now().Add(3*time.Hour))
			f.write(t, empty.ID, "now filled", 2, now().Add(3*time.Hour))
			second, err := reader.Nodes(ctx, f.eng)
			if err != nil {
				return err
			}
			secondSources, err := source.Nodes(ctx, f.eng, raw.ID)
			slices.SortFunc(first, func(a, b page.LintNode) int { return a.ID.Compare(b.ID) })
			slices.SortFunc(second, func(a, b page.LintNode) int { return a.ID.Compare(b.ID) })
			slices.SortFunc(firstSources, func(a, b page.SourceNode) int { return a.ID.Compare(b.ID) })
			slices.SortFunc(secondSources, func(a, b page.SourceNode) int { return a.ID.Compare(b.ID) })
			if err == nil && (!reflect.DeepEqual(first, second) || !reflect.DeepEqual(firstSources, secondSources)) {
				t.Error("compact or source reader escaped the caller's snapshot")
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		current, err := reader.Nodes(t.Context(), f.eng)
		if err != nil {
			t.Fatal(err)
		}
		wantBytes := map[uuid.UUID]int64{stale.ID: int64(len("a newer body")), empty.ID: int64(len("now filled"))}
		for _, node := range current {
			if bytes, ok := wantBytes[node.ID]; ok {
				if node.PageBytes != bytes {
					t.Errorf("new snapshot did not observe committed current bytes: %+v", node)
				}
				delete(wantBytes, node.ID)
			}
		}
		if len(wantBytes) != 0 {
			t.Errorf("new snapshot omitted committed pages: %v", wantBytes)
		}
	})
	for _, deleted := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing-content/%t", deleted), func(t *testing.T) {
			broken := newFixture(t)
			n := broken.page(t, broken.eng, nil, "DO NOT LEAK THIS DISPLAY NAME", 0)
			if deleted {
				broken.exec(t, "UPDATE page_contents SET deleted_at=$2 WHERE node_id=$1", n.ID, now())
			} else {
				broken.exec(t, "DELETE FROM page_contents WHERE node_id=$1", n.ID)
			}
			rows, err := page.NewLintNodes(broken.pool).Nodes(t.Context(), broken.eng)
			wantError := fmt.Sprintf("lint metadata: live page has no content (notebook_id=%s node_id=%s)", broken.eng, n.ID)
			if rows != nil || err == nil || err.Error() != wantError {
				t.Fatalf("missing content = %+v/%v; want sanitized invariant %q", rows, err, wantError)
			}
		})
	}
}

func TestLintMetadataReadsNoBody(t *testing.T) {
	f := newFixture(t)
	raw := f.page(t, f.eng, nil, "Raw", 0)
	large := f.page(t, f.eng, &raw.ID, "Large", 0)
	random := rand.New(rand.NewPCG(20261030, 1))
	bytes := make([]byte, 3<<18)
	for i := range bytes {
		bytes[i] = byte(random.Uint32())
	}
	body := base64.StdEncoding.EncodeToString(bytes)
	f.write(t, large.ID, body, 2, now())
	const toastBlocks = `SELECT coalesce(sum(pg_stat_get_xact_blocks_fetched(x.r)),0)::bigint FROM (
        SELECT reltoastrelid AS r FROM pg_class WHERE oid='page_contents'::regclass
        UNION ALL SELECT i.indexrelid FROM pg_index i JOIN pg_class c ON c.reltoastrelid=i.indrelid
        WHERE c.oid='page_contents'::regclass) x`
	err := postgres.NewTxManager(f.pool, 5*time.Second).WithinSnapshot(t.Context(), func(ctx context.Context) error {
		db := postgres.DB(ctx, f.pool)
		var before, metadata, actual int64
		if err := db.QueryRow(ctx, toastBlocks).Scan(&before); err != nil {
			return err
		}
		sources, err := page.NewSourceArea(f.pool).Nodes(ctx, f.eng, raw.ID)
		if err != nil || len(sources) != 1 || sources[0].ID != large.ID || sources[0].PageBytes != int64(len(body)) {
			t.Fatalf("nonempty metadata source control=%+v/%v", sources, err)
		}
		nodes, err := page.NewLintNodes(f.pool).Nodes(ctx, f.eng)
		if err != nil || len(nodes) != 2 {
			t.Fatalf("nonempty whole-book metadata control=%+v/%v", nodes, err)
		}
		if err := db.QueryRow(ctx, toastBlocks).Scan(&metadata); err != nil {
			return err
		}
		content, err := f.s.PageContent(ctx, large.ID)
		if err != nil || content.Content != body {
			t.Fatalf("actual body positive control returned %d bytes/%v", len(content.Content), err)
		}
		if err := db.QueryRow(ctx, toastBlocks).Scan(&actual); err != nil {
			return err
		}
		t.Logf("TOAST blocks: two real metadata calls=%d; real full-body positive control=%d", metadata-before, actual-metadata)
		if metadata != before || actual-metadata < 100 {
			t.Errorf("body TOAST work: metadata=%d, positive=%d; want 0 and >=100", metadata-before, actual-metadata)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
