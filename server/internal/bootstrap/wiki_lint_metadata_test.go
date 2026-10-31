package bootstrap

import (
	"errors"
	"reflect"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

func TestLintMetadataBundle(t *testing.T) {
	t.Run("real-nonempty-and-next-snapshot", func(t *testing.T) {
		f := newLintRuntime(t, nil)
		nb, outside, asset := f.seedPending(t)
		const body = "source🙂"
		source := f.tm.createPageWith(t, "alice", nb.id, nb.raw, "Source", body)
		scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}
		want := map[uuid.UUID]struct {
			bytes int64
			path  string
		}{uuid.MustParse(source): {int64(len(body)), "raw/Source.md"}, uuid.MustParse(asset): {12, "raw/pending.txt"}}
		assert := func() uint64 {
			t.Helper()
			before, _ := f.trace.copy()
			got, err := f.module.Execute(f.actor(t, "alice"), scope, wiki.LintQuery{})
			if err != nil {
				t.Fatalf("real bundle lost nonempty source/node projection: %v", err)
			}
			pending := lintResultRule(t, got, wiki.RuleUndigestedSources)
			if pending.Count.Value != int64(len(want)) || pending.Count.Capped || len(pending.Items) != len(want) {
				t.Fatalf("real bundle lost complete pending sources: %+v, want %d", pending, len(want))
			}
			seen := map[uuid.UUID]bool{}
			for _, item := range pending.Items {
				if item.NodeID == nil {
					t.Fatalf("real bundle omitted source identity: %+v", item)
				}
				entry, ok := want[*item.NodeID]
				if !ok || seen[*item.NodeID] || item.Bytes != entry.bytes || item.Path == nil || *item.Path != entry.path || item.CreatedAt.IsZero() {
					t.Fatalf("real bundle changed source bytes/path/identity: %+v", item)
				}
				seen[*item.NodeID] = true
			}
			found := false
			for _, item := range lintResultRule(t, got, wiki.RuleOrphans).Items {
				if item.PageID != nil && *item.PageID == uuid.MustParse(outside) && item.Path != nil && *item.Path == "Outside.md" {
					found = true
				}
			}
			if !found {
				t.Fatal("real bundle lost outside-source notebook metadata")
			}
			after, _ := f.trace.copy()
			var snapshot uint64
			var calls int
			for _, q := range after[len(before):] {
				if q.Name == "LintNodeMetadata" {
					calls++
					snapshot = q.Snapshot
				}
			}
			if calls != 1 || snapshot == 0 {
				t.Fatalf("real bundle did not read one snapshot's metadata: calls=%d snapshot=%d", calls, snapshot)
			}
			return snapshot
		}
		first := assert()
		added := f.tm.createPageWith(t, "alice", nb.id, nb.raw, "Added", "next")
		want[uuid.MustParse(added)] = struct {
			bytes int64
			path  string
		}{4, "raw/Added.md"}
		if second := assert(); second == first {
			t.Fatal("next invocation reused the prior metadata snapshot")
		}
		f.invariants(t)
	})
	t.Run("compact-projection-owns-parent", func(t *testing.T) {
		parent := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		id := uuid.MustParse("00000000-0000-0000-0000-000000000002")
		original := []page.LintNode{{ID: id, ParentID: &parent, Kind: page.KindPage, NameKey: "node", PageBytes: 7}}
		got, err := wikiCompactNodes(original)
		if err != nil || len(got) != 1 || got[0].ParentID == nil || *got[0].ParentID != parent || got[0].PageBytes != 7 {
			t.Fatalf("compact projection changed metadata: %+v/%v", got, err)
		}
		got[0].NameKey = "caller"
		*got[0].ParentID = uuid.UUID{}
		if parent != uuid.MustParse("00000000-0000-0000-0000-000000000001") || original[0].NameKey != "node" {
			t.Fatal("compact projection borrowed caller metadata")
		}
		invalid := append(original, page.LintNode{ID: parent, Kind: "unknown"})
		partial, err := wikiCompactNodes(invalid)
		if !errors.Is(err, errWikiLintValue) || !reflect.DeepEqual(partial, []wiki.CompactNode(nil)) {
			t.Fatalf("invalid kind exposed partial compact metadata: %+v/%v", partial, err)
		}
	})
}
