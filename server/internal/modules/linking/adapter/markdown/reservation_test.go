package markdownadapter_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func reservedWholeBudget(t *testing.T, b *markdown.Budget) {
	t.Helper()
	h, err := b.TakeNow(t.Context(), b.Size())
	if err != nil {
		t.Fatalf("RESERVED_RELEASE: whole original budget was not returned: %v", err)
	}
	h.Release()
}

func TestReservedParserRunsTwoRealParsesWithinOriginalBudget(t *testing.T) {
	entered, release := make(chan struct{}, 2), make(chan struct{})
	var opened sync.Once
	var workers sync.WaitGroup
	unblock := func() { opened.Do(func() { close(release) }) }
	t.Cleanup(func() { unblock(); workers.Wait() })
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{}), {Name: "reservation-barrier", Extract: func(markdown.Tree) any { entered <- struct{}{}; <-release; return nil }}})
	if err != nil {
		t.Fatal(err)
	}
	b := markdown.NewBudget(65536, time.Second, slog.New(slog.DiscardHandler))
	p := markdownadapter.NewParser(md, b)
	a, okA := p.TryReserve(8192)
	c, okC := p.TryReserve(8192)
	if !okA || !okC {
		t.Fatal("two real reservations")
	}
	type outcome struct {
		parsed app.Parsed
		err    error
	}
	done := make(chan outcome, 2)
	contents := []string{strings.Repeat("[[A]] ", 1000), strings.Repeat("[[B]] ", 513)}
	workers.Go(func() { v, e := a.ParseNow(t.Context(), contents[0]); done <- outcome{v, e} })
	workers.Go(func() { v, e := c.ParseNow(t.Context(), contents[1]); done <- outcome{v, e} })
	for range 2 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("real parse did not enter barrier")
		}
	}
	rest, err := b.TakeNow(t.Context(), 49152)
	if err != nil {
		unblock()
		t.Fatal(err)
	}
	if h, e := b.TakeNow(t.Context(), 1); e == nil {
		h.Release()
		unblock()
		t.Fatal("RESERVED_GLOBAL: parses used a separate budget")
	}
	var busy *shared.Error
	if _, e := p.ParseNow(t.Context(), "[[ordinary]]"); !errors.As(e, &busy) || busy.Code != "server_busy" {
		unblock()
		t.Fatalf("ordinary real busy changed: %v", e)
	}
	rest.Release()
	unblock()
	got := map[string]app.Parsed{}
	for range 2 {
		x := <-done
		if x.err != nil || len(x.parsed.Facts.Links) == 0 {
			t.Fatalf("real reserved parse: %v", x.err)
		}
		got[x.parsed.Facts.Links[0].Target] = x.parsed
	}
	if len(got) != 2 || len(got["A"].Facts.Links) != 1000 || len(got["B"].Facts.Links) != 513 || got["A"].Kept != 614 || got["B"].Kept != 322 {
		t.Fatalf("RESERVED_ACTUAL: literal results/Kept lost: %+v", got)
	}
	got["A"].Facts.Links[0].Target = "changed"
	if got["B"].Facts.Links[0].Target != "B" {
		t.Fatal("different parse facts alias")
	}
	a.Close()
	c.Close()                                  // Transfer the two actual Kept shares; no re-acquisition.
	tail, err := b.TakeNow(t.Context(), 64600) // 65536-614-322.
	if err != nil {
		t.Fatal(err)
	}
	if h, e := b.TakeNow(t.Context(), 1); e == nil {
		h.Release()
		t.Fatal("RESERVED_KEEP: Close released live facts")
	}
	tail.Release()
	for _, v := range got {
		v.Release()
		v.Release()
	}
	a.Close()
	c.Close()
	reservedWholeBudget(t, b)
}

func TestReservedParserOptionalMissIsNotAParseError(t *testing.T) {
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	b := markdown.NewBudget(8192, time.Second, slog.New(slog.DiscardHandler))
	p := markdownadapter.NewParser(md, b)
	all, err := b.TakeNow(t.Context(), 8192)
	if err != nil {
		t.Fatal(err)
	}
	if r, ok := p.TryReserve(1); ok || r != nil {
		t.Fatal("optional miss returned a reservation")
	}
	var busy *shared.Error
	if _, err := p.ParseNow(t.Context(), "[[A]]"); !errors.As(err, &busy) || busy.Code != "server_busy" {
		t.Fatalf("real busy hidden: %v", err)
	}
	all.Release()
	r, ok := p.TryReserve(5)
	if !ok {
		t.Fatal("reserve")
	}
	old, err := r.ParseNow(t.Context(), "[[A]]")
	if err != nil || len(old.Facts.Links) != 1 {
		t.Fatal("old parse")
	}
	other, err := b.TakeNow(t.Context(), 4096)
	if err != nil {
		t.Fatal(err)
	}
	if r.TryGrow(8192) {
		t.Fatal("grow borrowed another request's credit")
	}
	old.Release()
	r.Close()
	if _, err := p.ParseNow(t.Context(), strings.Repeat("a", 5000)); !errors.As(err, &busy) || busy.Code != "server_busy" {
		t.Fatalf("actual fallback parse busy was retried/hidden: %v", err)
	}
	other.Release()
	reservedWholeBudget(t, b)
}

func TestReservedParserPanicAndFactsErrorReleaseThroughParent(t *testing.T) {
	for _, mode := range []string{"panic", "facts_error"} {
		t.Run(mode, func(t *testing.T) {
			var exts []markdown.Extension
			if mode == "panic" {
				exts = []markdown.Extension{obsidian.Extension(obsidian.Options{}), {Name: "panic", Extract: func(markdown.Tree) any { panic("reserved original panic") }}}
			}
			md, err := markdown.New(exts)
			if err != nil {
				t.Fatal(err)
			}
			b := markdown.NewBudget(16384, time.Second, slog.New(slog.DiscardHandler))
			p := markdownadapter.NewParser(md, b)
			r, ok := p.TryReserve(8192)
			if !ok {
				t.Fatal("reserve")
			}
			var caught any
			var parsed app.Parsed
			func() {
				defer func() { caught = recover() }()
				parsed, err = r.ParseNow(context.Background(), strings.Repeat("[[A]] ", 1000))
			}()
			if mode == "panic" && caught != "reserved original panic" || mode == "facts_error" && (caught != nil || err == nil) || parsed.Release != nil {
				t.Fatalf("original panic/error lost: %v / %v", caught, err)
			}
			if h, e := b.TakeNow(t.Context(), 8193); e == nil {
				h.Release()
				t.Fatal("parent closed while its future slot was still owned")
			}
			r.Close()
			r.Close()
			reservedWholeBudget(t, b)
		})
	}
}
