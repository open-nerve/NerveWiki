package app_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Only data-port observations are added; the stream and Prepare stay real.
type lintPresenceData struct {
	*lintData
	probes [][]uuid.UUID
	probe  func(context.Context)
}

func (f *lintPresenceData) HasSourceProperties(ctx context.Context, book uuid.UUID, ids []uuid.UUID) (bool, error) {
	has, err := f.lintData.HasSourceProperties(ctx, book, ids)
	f.probes = append(f.probes, slices.Clone(ids))
	if f.probe != nil {
		f.probe(ctx)
	}
	return has, err
}

func TestLintSourcePresence(t *testing.T) {
	t.Run("absence-skips-property-reads", func(t *testing.T) {
		base, _ := lintFixture(lintNode(20, nil, "p", domain.NodePage, 1))
		f := &lintPresenceData{lintData: base}
		stream := app.NewLintReads(f, 5<<20).SourceValues(f.nb, []uuid.UUID{lintID(20)})
		if len(f.calls) != 0 {
			t.Fatal("constructing a stream performed I/O")
		}
		for range 3 {
			rows, err := stream.Next(t.Context())
			if err != nil || len(rows) != 0 {
				t.Fatalf("absence stream=%v, %v", rows, err)
			}
		}
		if !slices.Equal(f.calls, []string{"source-exists"}) || len(f.probes) != 1 {
			t.Fatalf("absence must probe once without property heads: calls=%v probes=%v", f.calls, f.probes)
		}
	})
	t.Run("absence-keeps-body-digestion", func(t *testing.T) {
		base, in := lintSourceFixture()
		base.properties = nil
		f := &lintPresenceData{lintData: base}
		p, err := app.NewRules(app.NewLintReads(f, 5<<20), f, f).Prepare(t.Context(), in, app.RuleQuery{})
		if err != nil {
			t.Fatal(err)
		}
		pending, _ := lintAnswer(t, p, domain.UndigestedSources, 3, false)
		var ids []uuid.UUID
		for _, item := range pending {
			ids = append(ids, *item.NodeID)
		}
		if !slices.Equal(ids, []uuid.UUID{lintID(30), lintID(33), lintID(42)}) {
			t.Fatalf("body-only digestion lost its independent pending result: %v", ids)
		}
		lintAnswer(t, p, domain.InvalidSources, 0, false)
		if !slices.Contains(f.calls, "bodies") || slices.Contains(f.calls, "source-heads") || slices.Contains(f.calls, "slot-heads") || len(f.probes) != 1 {
			t.Fatalf("absence must end only property flow: calls=%v probes=%v", f.calls, f.probes)
		}
	})
	t.Run("complete-owned-relation-once-and-zero", func(t *testing.T) {
		base, _ := lintFixture()
		ids := make([]uuid.UUID, 1025)
		for i := range ids {
			ids[i] = lintID(uint64(i))
			base.heads = append(base.heads, app.IndexedHead{ID: ids[i], Extractor: domain.Extractor, FrontmatterValid: true})
		}
		base.properties = []app.SourceProperty{{SourcePropertyPos: app.SourcePropertyPos{PageID: ids[0]}, Key: "Sources", JSON: []byte(`"zero"`)}, {SourcePropertyPos: app.SourcePropertyPos{PageID: ids[1024]}, Key: "sources", JSON: []byte(`"last"`)}}
		f := &lintPresenceData{lintData: base}
		stream := app.NewLintReads(f, 5<<20).SourceValues(f.nb, ids)
		ids[0], ids[1024] = lintID(90000), lintID(90001)
		var got []uuid.UUID
		for {
			rows, err := stream.Next(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) == 0 {
				break
			}
			for _, p := range rows {
				got = append(got, p.PageID)
			}
		}
		if !slices.Equal(got, []uuid.UUID{lintID(0), lintID(1024)}) {
			t.Fatalf("complete owned relation lost zero or final page: %v", got)
		}
		if len(f.probes) != 1 || len(f.probes[0]) != 1025 || f.probes[0][0] != lintID(0) || f.probes[0][1024] != lintID(1024) {
			t.Fatalf("presence must see complete owned 1025 IDs once: %v", f.probes)
		}
	})
	t.Run("last-only-is-not-absence", func(t *testing.T) {
		base, _ := lintFixture()
		ids := make([]uuid.UUID, 1025)
		for i := range ids {
			ids[i] = lintID(uint64(i))
			base.heads = append(base.heads, app.IndexedHead{ID: ids[i], Extractor: domain.Extractor, FrontmatterValid: true})
		}
		base.properties = []app.SourceProperty{{SourcePropertyPos: app.SourcePropertyPos{PageID: ids[1024]}, Key: "sources", JSON: []byte(`"last"`)}}
		f := &lintPresenceData{lintData: base}
		rows, err := app.NewLintReads(f, 5<<20).SourceValues(f.nb, ids).Next(t.Context())
		if err != nil || len(rows) != 1 || rows[0].PageID != lintID(1024) {
			t.Fatalf("presence truncated complete eligibility before final source: %+v %v", rows, err)
		}
		if len(f.probes) != 1 || len(f.probes[0]) != 1025 {
			t.Fatalf("final-only presence relation=%v", f.probes)
		}
	})
	t.Run("empty-and-invalid-input", func(t *testing.T) {
		for _, ids := range [][]uuid.UUID{nil, {lintID(2), lintID(1)}, {lintID(1), lintID(1)}} {
			base, _ := lintFixture()
			f := &lintPresenceData{lintData: base}
			_, err := app.NewLintReads(f, 5<<20).SourceValues(f.nb, ids).Next(t.Context())
			if (err != nil) != (len(ids) > 0) || len(f.calls) != 0 {
				t.Fatalf("invalid/empty eligibility performed I/O or accepted unsorted IDs: ids=%v calls=%v err=%v", ids, f.calls, err)
			}
		}
	})
	t.Run("probe-error-and-post-call-cancellation", func(t *testing.T) {
		for _, cancelAfter := range []bool{false, true} {
			base, _ := lintFixture(lintNode(20, nil, "p", domain.NodePage, 1))
			f := &lintPresenceData{lintData: base}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			want := f.boom
			if cancelAfter {
				f.probe = func(context.Context) { cancel() }
				want = context.Canceled
			} else {
				f.fail = "source-exists"
			}
			stream := app.NewLintReads(f, 5<<20).SourceValues(f.nb, []uuid.UUID{lintID(20)})
			for range 2 {
				rows, err := stream.Next(ctx)
				if len(rows) != 0 || !errors.Is(err, want) {
					t.Fatalf("presence failure/cancellation became absence: cancel=%t rows=%v err=%v want=%v", cancelAfter, rows, err, want)
				}
			}
			if !reflect.DeepEqual(f.calls, []string{"source-exists"}) {
				t.Fatalf("failed presence continued another read: %v", f.calls)
			}
		}
	})
	t.Run("canceled-empty-before-and-after-absence", func(t *testing.T) {
		for _, empty := range []bool{false, true} {
			for _, drained := range []bool{false, true} {
				base, _ := lintFixture(lintNode(20, nil, "p", domain.NodePage, 1))
				f := &lintPresenceData{lintData: base}
				ids := []uuid.UUID{lintID(20)}
				if empty {
					ids = nil
				}
				stream := app.NewLintReads(f, 5<<20).SourceValues(f.nb, ids)
				if drained {
					if rows, err := stream.Next(t.Context()); err != nil || len(rows) != 0 {
						t.Fatalf("absence drain: %v %v", rows, err)
					}
				}
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				rows, err := stream.Next(ctx)
				if len(rows) != 0 || !errors.Is(err, context.Canceled) {
					t.Fatalf("empty=%t drained=%t ignored cancellation: %v %v", empty, drained, rows, err)
				}
				wantCalls := 0
				if drained && !empty {
					wantCalls = 1
				}
				if len(f.calls) != wantCalls {
					t.Fatalf("canceled stream performed I/O: calls=%v want=%d", f.calls, wantCalls)
				}
			}
		}
	})
}
