package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func resolverCacheID(n int) uuid.UUID { return uuid.UUID{14: byte(n >> 8), 15: byte(n)} }

// A cache at its raw-key bound must still retain each uncached valid parse,
// including multiple overflow blocks; invalid entries must not shift readers.
func TestResolverTargetRowsKeepLiteralOverflow(t *testing.T) {
	for _, extra := range []int{0, 1, 255, 256, 257, 513} {
		t.Run(fmt.Sprintf("overflow=%d", extra), func(t *testing.T) {
			raw := []string{"./x", "", "a//b"}
			want := []domain.Target{{Relative: true, Keys: []string{"x"}, Name: "x"}, {}, {}}
			valid := []bool{true, false, false}
			for i := 3; i < 10000; i++ {
				s := fmt.Sprintf("missing%05d", i)
				raw = append(raw, s)
				want = append(want, domain.Target{Keys: []string{s}, Name: s})
				valid = append(valid, true)
			}
			for i := range extra {
				s := fmt.Sprintf("tail%05d", i)
				raw = append(raw, s, "./x", "")
				want = append(want, domain.Target{Keys: []string{s}, Name: s}, domain.Target{Relative: true, Keys: []string{"x"}, Name: "x"}, domain.Target{})
				valid = append(valid, true, true, false)
			}
			rows := newParsedTargetRows(len(raw))
			for i, s := range raw {
				got, ok := rows.add(i, s)
				if ok != valid[i] || !reflect.DeepEqual(got, want[i]) {
					t.Fatalf("add row%d=%+v/%v want%+v/%v", i, got, ok, want[i], valid[i])
				}
			}
			if len(rows.cache) != 10000 || len(rows.values) != 9998 {
				t.Fatalf("bounded raw cache=%d valid values=%d", len(rows.cache), len(rows.values))
			}
			held := 0
			for i, b := range rows.overflow {
				if len(b) == 0 || len(b) > 256 || cap(b) != 256 || i < len(rows.overflow)-1 && len(b) != 256 {
					t.Fatalf("overflow block%d len/cap=%d/%d", i, len(b), cap(b))
				}
				held += len(b)
			}
			if held != extra {
				t.Fatalf("overflow retained %d values, want %d", held, extra)
			}
			for pass := range 2 {
				spill := 0
				for i := range raw {
					got, ok := rows.at(i, &spill)
					if ok != valid[i] || !reflect.DeepEqual(got, want[i]) {
						t.Fatalf("pass%d row%d=%+v/%v want%+v/%v", pass, i, got, ok, want[i], valid[i])
					}
				}
				if spill != extra {
					t.Fatalf("pass%d read%d overflow values want%d", pass, spill, extra)
				}
			}
		})
	}
}

// No row cap or independently allocated Target per repeated row: a first raw
// entry at position zero is a real cached value, and an invalid parse is not it.
func TestResolverTargetRowsShareValuesWithoutCappingRows(t *testing.T) {
	const count = 65537
	rows := newParsedTargetRows(count)
	for i := range count {
		raw := "../X.MD"
		if i%3 == 1 {
			raw = "a//b"
		}
		rows.add(i, raw)
	}
	if len(rows.refs) != count || len(rows.values) != 1 || len(rows.cache) != 2 || len(rows.overflow) != 0 {
		t.Fatal("repeated rows acquired per-row Target values or were capped")
	}
	want := domain.Target{Relative: true, Up: 1, Keys: []string{"x"}, AltLast: "x.md", Name: "X"}
	spill := 0
	for i := range count {
		got, ok := rows.at(i, &spill)
		if i%3 == 1 {
			if ok || !reflect.DeepEqual(got, domain.Target{}) {
				t.Fatal("invalid target became cached entry zero")
			}
		} else if !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("row%d=%+v/%v", i, got, ok)
		}
	}
	if spill != 0 {
		t.Fatal("cached rows consumed overflow")
	}
}

func TestResolverFolderMemoSharesOnlyExactRuns(t *testing.T) {
	a := []domain.Step{{ID: resolverCacheID(10), Key: "a", Name: "A"}, {ID: resolverCacheID(20), Key: "box", Name: "Box"}}
	source := func(folder []domain.Step, id int) []domain.Step {
		return append(slices.Clone(folder), domain.Step{ID: resolverCacheID(id), Key: "source", Name: "Source"})
	}
	target := domain.Target{Relative: true, Keys: []string{"x"}}.Key()
	result := domain.Resolution{ID: resolverCacheID(90), Asset: true, Ambiguous: true}
	for _, mode := range []string{"same", "ancestor_key", "folder_id", "folder_name", "depth", "missing"} {
		t.Run(mode, func(t *testing.T) {
			m := newResolutionMemo(true)
			m.begin(source(a, 1), true)
			m.put(resolverCacheID(1), target, result)
			folder := slices.Clone(a)
			exists := true
			switch mode {
			case "ancestor_key":
				folder[0].Key = "b"
			case "folder_id":
				folder[1].ID = resolverCacheID(21)
			case "folder_name":
				folder[1].Name = "BOX"
			case "depth":
				folder = folder[1:]
			case "missing":
				exists = false
			}
			m.begin(source(folder, 2), exists)
			got, ok := m.get(resolverCacheID(2), target)
			if mode == "same" {
				if !ok || got != result {
					t.Fatalf("same complete folder missed fresh result: %+v/%v", got, ok)
				}
				if len(m.source) != 0 {
					t.Fatal("shared hit needlessly copied a stable run entry into source cache")
				}
			} else if ok || len(m.shared) != 0 {
				t.Fatalf("different or missing folder reused result: %+v/%v", got, ok)
			}
		})
	}
	t.Run("known_empty_and_zero_result", func(t *testing.T) {
		m := newResolutionMemo(true)
		m.begin(nil, true)
		zero := domain.Resolution{Ambiguous: true, Asset: true}
		m.put(uuid.UUID{}, target, zero)
		m.begin([]domain.Step{{ID: resolverCacheID(1), Key: "leaf", Name: "leaf"}}, true)
		if got, ok := m.get(resolverCacheID(1), target); !ok || got != zero {
			t.Fatal("known empty folder/zero resolution confused with absence")
		}
		m.begin(nil, false)
		if _, ok := m.get(resolverCacheID(2), target); ok {
			t.Fatal("missing source inherited known empty folder")
		}
	})
}

func TestResolverFolderMemoKeepsSourceCachePastSharedBound(t *testing.T) {
	path := []domain.Step{{ID: resolverCacheID(10), Key: "folder", Name: "Folder"}, {ID: resolverCacheID(1), Key: "source", Name: "Source"}}
	m := newResolutionMemo(true)
	m.begin(path, true)
	for i := range 10000 {
		key := domain.Target{Keys: []string{fmt.Sprintf("target%05d", i)}}.Key()
		m.put(resolverCacheID(1), key, domain.Resolution{})
	}
	m.begin(path, true)
	overflow := domain.Target{Keys: []string{"overflow"}}.Key()
	want := domain.Resolution{ID: resolverCacheID(77)}
	m.put(resolverCacheID(2), overflow, want)
	for range 3 {
		if got, ok := m.get(resolverCacheID(2), overflow); !ok || got != want {
			t.Fatal("shared bound lost same-source canonical reuse")
		}
	}
	if len(m.shared) != 10000 || len(m.source) != 1 {
		t.Fatal("shared grew past bound or source cache was truncated")
	}
	if _, ok := m.get(resolverCacheID(3), overflow); ok {
		t.Fatal("overflow leaked into bounded shared cache")
	}
	// The original unordered behavior must keep revisited sources independently.
	m = newResolutionMemo(false)
	m.put(resolverCacheID(2), overflow, want)
	m.put(uuid.UUID{}, overflow, domain.Resolution{ID: resolverCacheID(78)})
	if got, ok := m.get(resolverCacheID(2), overflow); !ok || got != want || len(m.source) != 2 || len(m.shared) != 0 {
		t.Fatal("unordered source cache was cleared or shared")
	}
}

// Keep the already allocated map across different and absent folders. A held
// map alias observes clearing/refill without depending on runtime map pointers.
func TestResolverFolderMemoReusesItsBoundedTable(t *testing.T) {
	m := newResolutionMemo(true)
	folder := []domain.Step{{ID: resolverCacheID(10), Key: "left", Name: "Left"}, {ID: resolverCacheID(1), Key: "source", Name: "Source"}}
	m.begin(folder, true)
	for i := range 10000 {
		key := domain.Target{Keys: []string{fmt.Sprintf("old%05d", i)}}.Key()
		m.put(resolverCacheID(1), key, domain.Resolution{ID: resolverCacheID(90)})
	}
	held := m.shared
	old := domain.Target{Keys: []string{"old00000"}}.Key()
	next := slices.Clone(folder)
	next[0] = domain.Step{ID: resolverCacheID(11), Key: "right", Name: "Right"}
	m.begin(next, true)
	if len(held) != 0 || len(m.shared) != 0 {
		t.Fatal("folder switch did not clear the existing shared table")
	}
	if _, ok := m.get(resolverCacheID(2), old); ok {
		t.Fatal("old-folder value survived")
	}
	for i := range 10000 {
		key := domain.Target{Keys: []string{fmt.Sprintf("new%05d", i)}}.Key()
		m.put(resolverCacheID(2), key, domain.Resolution{ID: resolverCacheID(91)})
	}
	key := domain.Target{Keys: []string{"new00000"}}.Key()
	if got, ok := held[key]; !ok || got.ID != resolverCacheID(91) || len(held) != 10000 {
		t.Fatal("refill replaced the bounded shared table")
	}
	m.put(resolverCacheID(2), domain.Target{Keys: []string{"overflow"}}.Key(), domain.Resolution{})
	if len(held) != 10000 || len(m.source) != 1 {
		t.Fatal("refill lost the shared/source bounds")
	}
	m.begin(nil, false)
	if len(held) != 0 {
		t.Fatal("absent source abandoned the old shared table")
	}
	m.begin(next, true)
	m.put(resolverCacheID(3), key, domain.Resolution{ID: resolverCacheID(92)})
	if got, ok := held[key]; !ok || got.ID != resolverCacheID(92) {
		t.Fatal("source after absence allocated another shared table")
	}
}

type resolverCacheWorld struct {
	Store
	Pages
	candidates, nodes []domain.Node
	aliases           []Alias
	calls             []string
	keys              []string
	ids               []uuid.UUID
	fail              string
	cause             error
}

func (w *resolverCacheWorld) ByKeys(_ context.Context, _ uuid.UUID, keys []string) ([]domain.Node, error) {
	w.calls = append(w.calls, "keys")
	w.keys = slices.Clone(keys)
	if w.fail == "keys" {
		return nil, w.cause
	}
	return w.candidates, nil
}
func (w *resolverCacheWorld) Aliases(_ context.Context, _ uuid.UUID, keys []string) ([]Alias, error) {
	w.calls = append(w.calls, "aliases")
	if !slices.Equal(keys, w.keys) {
		return nil, errors.New("aliases received other keys")
	}
	if w.fail == "aliases" {
		return nil, w.cause
	}
	return w.aliases, nil
}
func (w *resolverCacheWorld) Paths(_ context.Context, _ uuid.UUID, ids []uuid.UUID) ([]domain.Node, error) {
	w.calls = append(w.calls, "paths")
	w.ids = slices.Clone(ids)
	if w.fail == "paths" {
		return nil, w.cause
	}
	var out []domain.Node
	for _, n := range w.nodes {
		if slices.Contains(ids, n.ID) {
			out = append(out, n)
		}
	}
	return out, nil
}
func resolverCacheNode(id int, folder []domain.Step, key string) domain.Node {
	return domain.Node{ID: resolverCacheID(id), Path: append(slices.Clone(folder), domain.Step{ID: resolverCacheID(id), Key: key, Name: key})}
}

// A wrong parent-only key reuses ./x from a known historical path. Missing
// aliases must still be recorded before a shared hit; gone aliases are absent.
func TestResolverFreshFolderResultsKeepKnownAndMissing(t *testing.T) {
	a := []domain.Step{{ID: resolverCacheID(10), Key: "a", Name: "A"}, {ID: resolverCacheID(20), Key: "box", Name: "Box"}}
	b := slices.Clone(a)
	b[0].Key, b[0].Name = "b", "B"
	liveB := slices.Clone(b)
	liveB[0].ID = resolverCacheID(11)
	liveB[1].ID = resolverCacheID(21)
	xa, xb := resolverCacheNode(50, a, "x"), resolverCacheNode(51, liveB, "x")
	photo := resolverCacheNode(52, a, "photo.png")
	photo.Asset = true
	w := &resolverCacheWorld{candidates: []domain.Node{xa, xb, photo}, nodes: []domain.Node{resolverCacheNode(0, a, "zero"), resolverCacheNode(1, a, "one"), resolverCacheNode(2, a, "two"), xa}, aliases: []Alias{{PageID: xa.ID, Key: "alias"}, {PageID: resolverCacheID(2), Key: "alias"}, {PageID: resolverCacheID(80), Key: "alias"}, {PageID: resolverCacheID(81), Key: "alias"}}}
	known := []domain.Node{resolverCacheNode(2, b, "two")}
	var links []Link
	var want []domain.Resolution
	for _, id := range []int{0, 1, 2} {
		for j, target := range []string{"./x", "./X", "../box/x", "/a/box/x", "alias", "missing", "photo.png"} {
			links = append(links, Link{SourceID: resolverCacheID(id), Start: j, Target: target, Resolution: domain.Resolution{ID: resolverCacheID(99)}})
			r := domain.Resolution{ID: xa.ID}
			if j < 3 && id == 2 {
				r.ID = xb.ID
			}
			if j == 5 {
				r = domain.Resolution{}
			}
			if j == 6 {
				r = domain.Resolution{ID: photo.ID, Asset: true}
			}
			want = append(want, r)
		}
	}
	links = append(links, Link{SourceID: resolverCacheID(3), Target: "./x"}, Link{SourceID: resolverCacheID(4), Target: ""})
	want = append(want, domain.Resolution{}, domain.Resolution{})
	got, missing, err := resolutions(t.Context(), w, w, resolverCacheID(100), links, known, map[uuid.UUID]bool{resolverCacheID(81): true})
	if err != nil || !slices.Equal(got, want) || !slices.Equal(missing, []uuid.UUID{resolverCacheID(3), resolverCacheID(80)}) {
		t.Fatalf("fresh results=%+v missing=%v err=%v want=%+v", got, missing, err, want)
	}
	if !slices.Equal(w.calls, []string{"keys", "aliases", "paths"}) || !slices.Equal(w.keys, []string{"alias", "missing", "photo.png", "x"}) {
		t.Fatalf("query order/keys %v %v", w.calls, w.keys)
	}
	if !slices.Contains(w.ids, resolverCacheID(2)) || slices.Contains(w.ids, resolverCacheID(81)) {
		t.Fatal("known alias override or gone-alias fixture was not exercised")
	}
}

func TestResolverOverflowAdvancesBeforeMissingSource(t *testing.T) {
	source, tail := resolverCacheNode(0, nil, "source"), resolverCacheNode(50, nil, "tail")
	w := &resolverCacheWorld{candidates: []domain.Node{tail}, nodes: []domain.Node{source, resolverCacheNode(4, nil, "last")}}
	links := make([]Link, 10000, 10003)
	for i := range links {
		links[i] = Link{SourceID: source.ID, Start: i, Target: fmt.Sprintf("missing%05d", i)}
	}
	links = append(links, Link{SourceID: resolverCacheID(3), Target: "first-overflow"}, Link{SourceID: resolverCacheID(4), Target: "tail"}, Link{SourceID: resolverCacheID(4), Target: "tail"})
	got, missing, err := resolutions(t.Context(), w, w, resolverCacheID(100), links, nil, nil)
	if err != nil || len(got) != 10003 || !slices.Equal(missing, []uuid.UUID{resolverCacheID(3)}) {
		t.Fatalf("complete overflow len=%d missing=%v err=%v", len(got), missing, err)
	}
	for i, r := range got {
		want := domain.Resolution{}
		if i >= 10001 {
			want.ID = tail.ID
		}
		if r != want {
			t.Fatalf("row%d=%+v want%+v", i, r, want)
		}
	}
}

type resolverCacheCancel struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (c *resolverCacheCancel) Err() error {
	c.checks++
	if c.checks == 2 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestResolverCachesKeepErrorsAndCancellation(t *testing.T) {
	source, target := resolverCacheNode(1, nil, "source"), resolverCacheNode(50, nil, "x")
	for _, at := range []string{"keys", "aliases", "paths"} {
		t.Run(at, func(t *testing.T) {
			cause := errors.New(at + " failed")
			w := &resolverCacheWorld{candidates: []domain.Node{target}, nodes: []domain.Node{source}, fail: at, cause: cause}
			got, missing, err := resolutions(t.Context(), w, w, resolverCacheID(100), []Link{{SourceID: source.ID, Target: "x"}}, nil, nil)
			want := []string{"keys"}
			if at != "keys" {
				want = append(want, "aliases")
			}
			if at == "paths" {
				want = append(want, "paths")
			}
			//nolint:errorlint // The resolver must return the exact native error object.
			if got != nil || missing != nil || err != cause || !slices.Equal(w.calls, want) {
				t.Fatalf("error order: %v %v %v calls%v", got, missing, err, w.calls)
			}
		})
	}
	t.Run("shared_hit_at_4096", func(t *testing.T) {
		base, cancel := context.WithCancel(t.Context())
		defer cancel()
		ctx := &resolverCacheCancel{Context: base, cancel: cancel}
		w := &resolverCacheWorld{candidates: []domain.Node{target}, nodes: []domain.Node{source, resolverCacheNode(2, nil, "other")}}
		links := make([]Link, 4097)
		for i := range links {
			links[i] = Link{SourceID: source.ID, Start: i, Target: "x"}
		}
		links[4096].SourceID = resolverCacheID(2)
		got, missing, err := resolutions(ctx, w, w, resolverCacheID(100), links, nil, nil)
		if got != nil || missing != nil || !errors.Is(err, context.Canceled) || ctx.checks != 2 || base.Err() != context.Canceled {
			t.Fatalf("cached boundary hid cancellation: %v %v %v checks%d", got, missing, err, ctx.checks)
		}
	})
	t.Run("all_invalid_needs_no_source_or_queries", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		w := &resolverCacheWorld{}
		got, missing, err := resolutions(ctx, w, w, resolverCacheID(100), []Link{{Target: ""}, {Target: "a//b"}}, nil, nil)
		if err != nil || missing != nil || !slices.Equal(got, []domain.Resolution{{}, {}}) || len(w.calls) != 0 {
			t.Fatal("all-invalid early return changed")
		}
	})
}
