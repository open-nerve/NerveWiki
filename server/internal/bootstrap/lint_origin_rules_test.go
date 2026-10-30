package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// lintOriginAssets converts only public module-root DTOs and records the actual
// final-ID projection. It delegates every byte lookup to the real asset reader.
type lintOriginAssets struct {
	infos asset.AttachmentInfos
	nb    uuid.UUID
	calls [][]uuid.UUID
}

func (a *lintOriginAssets) BytesOf(ctx context.Context, nb uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	if nb != a.nb {
		return nil, errors.New("compat asset reader received a foreign notebook")
	}
	a.calls = append(a.calls, slices.Clone(ids))
	infos, err := a.infos.Of(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int64, len(infos))
	for id, info := range infos {
		out[id] = info.Bytes
	}
	return out, nil
}

func lintOriginInput(ctx context.Context, tm acmeTeam, nb, root uuid.UUID) (linking.ReadInput, error) {
	nodes, err := page.NewLintNodes(tm.pool).Nodes(ctx, nb)
	if err != nil {
		return linking.ReadInput{}, err
	}
	sources, err := page.NewSourceArea(tm.pool).Nodes(ctx, nb, root)
	if err != nil {
		return linking.ReadInput{}, err
	}
	in := linking.ReadInput{NotebookID: nb, Explicit: true, EffectiveMode: linking.ModeLLM, AreaRootID: &root}
	for _, n := range nodes {
		var kind linking.NodeKind
		switch n.Kind {
		case page.KindPage:
			kind = linking.NodePage
		case page.KindAsset:
			kind = linking.NodeAsset
		default:
			return linking.ReadInput{}, errors.New("compat metadata has an unknown kind")
		}
		in.Nodes = append(in.Nodes, linking.CompactNode{ID: n.ID, ParentID: n.ParentID, Kind: kind,
			NameKey: n.NameKey, Linkable: n.Linkable, CreatedAt: n.CreatedAt, PageBytes: n.PageBytes})
	}
	for _, n := range sources {
		in.SourceIDs = append(in.SourceIDs, n.ID)
	}
	return in, nil
}

// This is the S1-2 consumer extension of the accepted literal-v1 witness. It
// calls the actual linking module root in one real snapshot; it deliberately
// does not claim a wiki Execute, authorization, flight, or HTTP lint operation.
func testLintOriginRules(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	root := tm.createPageWith(t, "alice", nb, "", "raw", "")
	a := tm.createPageWith(t, "alice", nb, root, "A", "source a\n")
	b := tm.createPageWith(t, "alice", nb, root, "B", "old B\n")
	old := tm.createPageWith(t, "alice", nb, root, "OldSource", "old source\n")
	var originalB time.Time
	if err := tm.pool.QueryRow(t.Context(), "SELECT created_at FROM nodes WHERE id=$1", b).Scan(&originalB); err != nil {
		t.Fatal(err)
	}
	tm.send(t, contentWrite("alice", b, "source b\n", 1, ""), http.StatusOK)
	attached := tm.upload(t, "alice", nb, a, "a.pdf", "attached to A")
	legacyPayload := "legacy payload"
	legacyAsset := tm.upload(t, "alice", nb, root, "legacy.pdf", legacyPayload)
	direct := tm.upload(t, "alice", nb, b, "direct.pdf", "direct payload")
	independentPayload := "independent payload"
	independent := tm.upload(t, "alice", nb, root, "independent.pdf", independentPayload)
	tm.send(t, contentWrite("alice", a, "---\n\"\": '[[independent.pdf]]'\n---\n[[a.pdf]]\n", 1, ""), http.StatusOK)
	tm.send(t, contentWrite("alice", old, "[[legacy.pdf]]\n", 1, ""), http.StatusOK)
	current := tm.createPageWith(t, "alice", nb, "", "Wiki",
		"---\ntype: note\nsummary: current\nSources: ['[[raw/A]]', '[[raw/OldSource]]', 0, '[[0]]']\n---\n[[raw/B/direct.pdf]]\n")
	legacy := tm.createPageWith(t, "alice", nb, "", "LegacyWiki",
		"---\n\"\": '[[raw/B]]'\n---\n[[raw/B]] [[legacy-only]]\n")
	empty := tm.createPageWith(t, "alice", nb, "", "EmptyProperty",
		"---\ntype: note\nsummary: current\n\"\": '[[raw/B]]'\n---\n[[needed]] [[needed]]\n")
	tm.originLegacy(t, old, legacy)
	for _, id := range []string{a, b, current, empty} {
		tm.originVersion(t, id, 2)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM page_links WHERE source_id=ANY($1::uuid[]) AND property_key=''", []string{a, empty}); n != 2 {
		t.Fatalf("current empty paths = %d, want two non-NULL property witnesses", n)
	}
	var mode string
	if err := tm.pool.QueryRow(t.Context(), "SELECT mode FROM notebooks WHERE id=$1", nb).Scan(&mode); err != nil || mode != "standard" {
		t.Fatalf("stored mode=%q error=%v, want standard with an explicit LLM evaluation", mode, err)
	}
	before := tm.originIndex(t, nb)
	notebookID, rootID := uuid.MustParse(nb), uuid.MustParse(root)
	assets := &lintOriginAssets{infos: asset.NewAttachmentInfos(tm.pool), nb: notebookID}
	rules := linking.NewLintRules(tm.pool, page.MaxContentBytes, assets)
	var prepared *linking.PreparedRules
	err := postgres.NewTxManager(tm.pool, time.Second).WithinSnapshot(t.Context(), func(ctx context.Context) error {
		in, err := lintOriginInput(ctx, tm, notebookID, rootID)
		if err != nil {
			return err
		}
		prepared, err = rules.Prepare(ctx, in, linking.RuleQuery{})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.StalePages() != 2 {
		t.Fatalf("mixed-v1 stale_pages=%d, want exactly the old source and old wiki emitter", prepared.StalePages())
	}
	// These counts and exact IDs are derived from the fixture above, not from
	// another rule implementation or the sources matcher being tested.
	counts := []struct {
		rule linking.Rule
		n    int64
	}{{linking.Wanted, 2}, {linking.Orphans, 2}, {linking.Ambiguous, 0}, {linking.InvalidFrontmatter, 0},
		{linking.MissingType, 0}, {linking.MissingSummary, 0}, {linking.UndigestedSources, 3}, {linking.InvalidSources, 2}}
	for _, want := range counts {
		got, err := prepared.RuleCount(t.Context(), want.rule)
		items, more, itemErr := prepared.RuleItems(t.Context(), want.rule)
		if err != nil || itemErr != nil || got.Value != want.n || got.Capped || more || int64(len(items)) != want.n {
			t.Fatalf("%s count=%+v items=%+v more=%t errors=%v/%v, want %d", want.rule, got, items, more, err, itemErr, want.n)
		}
	}
	wanted, _, _ := prepared.RuleItems(t.Context(), linking.Wanted)
	if len(wanted) != 2 || wanted[0].Target != "0" || wanted[1].Target != "needed" || wanted[0].Pages != 1 || wanted[1].Pages != 1 {
		t.Fatalf("wanted=%+v; stale emitter and duplicate spelling must not change the two real groups", wanted)
	}
	bad, _, _ := prepared.RuleItems(t.Context(), linking.InvalidSources)
	if len(bad) != 2 || bad[0].Target != "0" || bad[1].Target != "0" || bad[0].NotLink == bad[1].NotLink ||
		bad[0].ItemKey == bad[1].ItemKey || *bad[0].PageID != uuid.MustParse(current) || *bad[1].PageID != uuid.MustParse(current) {
		t.Fatalf("invalid sources=%+v, want separate 0 J/L and no error for the stale source target", bad)
	}
	pending, _, _ := prepared.RuleItems(t.Context(), linking.UndigestedSources)
	wantBytes := map[uuid.UUID]int64{uuid.MustParse(b): 9, uuid.MustParse(legacyAsset.ID): int64(len(legacyPayload)),
		uuid.MustParse(independent.ID): int64(len(independentPayload))}
	for i, item := range pending {
		if item.NodeID == nil || item.Cursor == "" || item.ItemKey == "" {
			t.Fatalf("pending omitted identity: %+v", item)
		}
		size, expected := wantBytes[*item.NodeID]
		if !expected || item.Bytes != size {
			t.Fatalf("pending=%+v, want only B/legacy.pdf/independent.pdf with bytes %v", pending, wantBytes)
		}
		delete(wantBytes, *item.NodeID)
		if *item.NodeID == uuid.MustParse(b) && (item.Kind != linking.NodePage || !item.CreatedAt.Equal(originalB)) {
			t.Fatalf("B lost its original creation time or page kind: %+v", item)
		}
		if *item.NodeID != uuid.MustParse(b) && item.Kind != linking.NodeAsset {
			t.Fatalf("attachment lost its kind: %+v", item)
		}
		if i > 0 {
			prev := pending[i-1]
			if prev.CreatedAt.After(item.CreatedAt) || prev.CreatedAt.Equal(item.CreatedAt) && prev.NodeID.Compare(*item.NodeID) >= 0 {
				t.Fatalf("pending order is not creation time then ID: %+v", pending)
			}
		}
	}
	if len(wantBytes) != 0 {
		t.Fatalf("pending omitted fixture IDs: %v", wantBytes)
	}
	finalAssets := []uuid.UUID{uuid.MustParse(legacyAsset.ID), uuid.MustParse(independent.ID)}
	slices.SortFunc(finalAssets, func(a, b uuid.UUID) int { return a.Compare(b) })
	if len(assets.calls) != 1 || !slices.Equal(assets.calls[0], finalAssets) {
		t.Fatalf("asset projection=%v, want only final two IDs once; followed=%s direct=%s", assets.calls, attached.ID, direct.ID)
	}
	if after := tm.originIndex(t, nb); !reflect.DeepEqual(after, before) {
		t.Fatal("rule reads rewrote literal-v1 or current index facts")
	}
	t.Logf("real root consumer: stale=2, wanted=2, invalid_sources=2 with separate L/J, pending=3; final asset IDs read once")
}
