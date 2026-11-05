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

type rebuildReadOrder struct {
	*contents
	ids   []uuid.UUID
	reads []uuid.UUID
}

func (c *rebuildReadOrder) PageIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return slices.Clone(c.ids), nil
}

func (c *rebuildReadOrder) Content(ctx context.Context, id uuid.UUID) (string, int, bool, error) {
	c.reads = append(c.reads, id)
	return c.contents.Content(ctx, id)
}

type rebuildCountParser struct {
	inputs []string
}

func (p *rebuildCountParser) Facts(ctx context.Context, content string) (domain.Facts, error) {
	p.inputs = append(p.inputs, content)
	return (parser{}).Facts(ctx, content)
}

type rebuildWriteCall struct {
	kind string
	id   uuid.UUID
}

type rebuildWritesRecorded struct {
	*store
	calls []rebuildWriteCall
	fail  rebuildWriteCall
	cause error
	reads int
	sets  int
}

func (s *rebuildWritesRecorded) enter(kind string, id uuid.UUID) error {
	call := rebuildWriteCall{kind, id}
	s.calls = append(s.calls, call)
	if call == s.fail {
		return s.cause
	}
	return nil
}

func (s *rebuildWritesRecorded) AddPage(ctx context.Context, p app.Page, f domain.Facts) error {
	if err := s.enter("ordinary", p.ID); err != nil {
		return err
	}
	return s.store.AddPage(ctx, p, f)
}

func (s *rebuildWritesRecorded) AddRebuildMetadata(ctx context.Context, p app.Page, f app.RebuildMetadata) error {
	if err := s.enter("metadata", p.ID); err != nil {
		return err
	}
	return s.store.AddRebuildMetadata(ctx, p, f)
}

func (s *rebuildWritesRecorded) AddRebuildUnresolvedLinks(ctx context.Context, p app.Page, links []domain.Link) error {
	if err := s.enter("drain", p.ID); err != nil {
		return err
	}
	return s.store.AddRebuildUnresolvedLinks(ctx, p, links)
}

func (s *rebuildWritesRecorded) AddRebuildResolvedLinks(ctx context.Context, p app.Page, links []domain.Link, rs []domain.Resolution) error {
	if err := s.enter("final", p.ID); err != nil {
		return err
	}
	return s.store.AddRebuildResolvedLinks(ctx, p, links, rs)
}

func (s *rebuildWritesRecorded) Links(ctx context.Context, notebook uuid.UUID, reach domain.Reach) ([]app.Link, error) {
	s.reads++
	return s.store.Links(ctx, notebook, reach)
}

func (s *rebuildWritesRecorded) SetResolutions(ctx context.Context, links []app.Link) error {
	s.sets++
	return s.store.SetResolutions(ctx, links)
}

func rebuildFlowFixture(t *testing.T) (*world, app.Rebuild, *rebuildReadOrder, *rebuildCountParser, *rebuildWritesRecorded) {
	w := newWorld(t, "early", "late", "tail")
	r, c, _ := newRebuild(w, map[string]string{"early": "only", "late": "alias:only", "tail": "only"})
	read := &rebuildReadOrder{contents: c, ids: []uuid.UUID{w.id("early"), w.id("late"), w.id("tail")}}
	parse := &rebuildCountParser{}
	writes := &rebuildWritesRecorded{store: w.store}
	r.Contents, r.Parser, r.Writes, r.Index.Store = read, parse, writes, writes
	return w, r, read, parse, writes
}

func rebuildOneRowLimit() uint64 {
	// Three source slots/order entries and one four-byte-target link fit;
	// an empty aliases-only page costs no extra staged link payload.
	return uint64(3*(reflect.TypeFor[[]domain.Link]().Size()+reflect.TypeFor[int]().Size()) + reflect.TypeFor[domain.Link]().Size() + 16)
}

func TestRebuildUsesAllLateAliasesAndFallsBackWithoutParsingAgain(t *testing.T) {
	for _, mode := range []string{"buffered", "last_page_fallback", "streaming"} {
		t.Run(mode, func(t *testing.T) {
			w, r, read, parse, writes := rebuildFlowFixture(t)
			switch mode {
			case "last_page_fallback":
				r.StagingLimit = rebuildOneRowLimit()
			case "streaming":
				r.StagingLimit = 1
			}
			got, err := r.Notebook(t.Context(), w.notebook)
			if err != nil || got.Pages != 3 || got.Links != 2 || got.Unresolved != 0 || len(got.Clashes) != 0 {
				t.Fatalf("complete rebuild=%+v err=%v", got, err)
			}
			if !slices.Equal(read.reads, read.ids) || !slices.Equal(parse.inputs, []string{"only", "alias:only", "only"}) {
				t.Fatalf("content or parse repeated: reads=%v parses=%v", read.reads, parse.inputs)
			}
			w.resolves("early", 0, "late", false)
			w.resolves("tail", 0, "late", false)
			if len(w.published.events) != 1 || w.published.events[0].NotebookID != w.notebook || len(w.published.events[0].Pages) != 0 || len(w.published.events[0].Targets) != 0 {
				t.Fatalf("complete event=%+v", w.published.events)
			}
			a, b, c := read.ids[0], read.ids[1], read.ids[2]
			var want []rebuildWriteCall
			switch mode {
			case "buffered":
				want = []rebuildWriteCall{{"metadata", a}, {"metadata", b}, {"metadata", c}, {"final", a}, {"final", b}, {"final", c}}
			case "last_page_fallback":
				want = []rebuildWriteCall{{"metadata", a}, {"metadata", b}, {"drain", a}, {"drain", b}, {"ordinary", c}}
			default:
				want = []rebuildWriteCall{{"ordinary", a}, {"ordinary", b}, {"ordinary", c}}
			}
			if !slices.Equal(writes.calls, want) {
				t.Fatalf("actual complete writes=%v want%v", writes.calls, want)
			}
			wantLegacy := 1
			if mode == "buffered" {
				wantLegacy = 0
			}
			if writes.reads != wantLegacy || writes.sets != wantLegacy {
				t.Fatalf("complete legacy read/set=%d/%d want%d", writes.reads, writes.sets, wantLegacy)
			}
		})
	}
}

func TestRebuildErrorsStopBeforePublishAndPreserveCause(t *testing.T) {
	for _, kind := range []string{"metadata", "drain", "ordinary", "final"} {
		t.Run(kind, func(t *testing.T) {
			w, r, read, parse, writes := rebuildFlowFixture(t)
			writes.cause = errors.New("literal rebuild write cause")
			writes.fail = rebuildWriteCall{kind, read.ids[0]}
			if kind == "metadata" || kind == "final" {
				writes.fail.id = read.ids[1]
			} else {
				r.StagingLimit = rebuildOneRowLimit()
				if kind == "ordinary" {
					writes.fail.id = read.ids[2]
				}
			}
			_, err := r.Notebook(t.Context(), w.notebook)
			if !errors.Is(err, writes.cause) || len(w.published.events) != 0 || writes.sets != 0 || writes.reads != 0 {
				t.Fatalf("failure ordering/cause: err=%v events=%v legacy=%d/%d", err, w.published.events, writes.reads, writes.sets)
			}
			if len(writes.calls) == 0 || writes.calls[len(writes.calls)-1] != writes.fail {
				t.Fatalf("work continued after failed write: %v", writes.calls)
			}
			wantParses := 3
			if kind == "metadata" {
				wantParses = 2
			}
			if len(parse.inputs) != wantParses || len(read.reads) != wantParses {
				t.Fatalf("failure reparsed or read beyond its position: %v %v", parse.inputs, read.reads)
			}
		})
	}
}

func TestRebuildRequiresExplicitWrites(t *testing.T) {
	w, r, read, parse, _ := rebuildFlowFixture(t)
	r.Writes = nil
	if _, err := r.Notebook(t.Context(), w.notebook); err == nil || len(read.reads) != 0 || len(parse.inputs) != 0 || len(w.store.locked) != 0 {
		t.Fatalf("missing private wiring silently selected the old path: %v", err)
	}
}
