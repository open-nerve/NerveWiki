package app_test

import (
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Virtual files precede every real sort order, including negative orders;
// a one-item page can stop between the two virtual files without losing nodes.
func TestVirtualListing(t *testing.T) {
	t.Run("negative_orders", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionList)
		f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki}}
		var wantIDs []uuid.UUID
		for i, order := range []float64{-3, -2, -1, 0, 1} {
			wantIDs = append(wantIDs, f.page(fmt.Sprintf("Real%d", i), nil, order).ID)
		}
		want := []string{"index.md", "log.md", "Real0.md", "Real1.md", "Real2.md", "Real3.md", "Real4.md"}
		for _, limit := range []int{1, 2} {
			var got []string
			var gotIDs []uuid.UUID
			var cursor *string
			ended := false
			for range 10 {
				page, err := f.lister(domain.MaxGlobWork).Execute(f.asAlice(), f.eng, "", &limit, cursor)
				if err != nil {
					t.Fatalf("limit %d: %v", limit, err)
				}
				got = append(got, listed(page)...)
				for _, item := range page.Items {
					if item.Node.ID != (uuid.UUID{}) {
						gotIDs = append(gotIDs, item.Node.ID)
					}
				}
				if page.Next == "" {
					ended = true
					break
				}
				cursor = &page.Next
			}
			if !slices.Equal(got, want) {
				t.Errorf("limit %d: listed %q, want virtual prefix and all five real nodes %q", limit, got, want)
			}
			if !ended || !slices.Equal(gotIDs, wantIDs) {
				t.Errorf("limit %d: ended=%v, real IDs=%v, want terminal cursor and %v", limit, ended, gotIDs, wantIDs)
			}
		}
	})
	t.Run("list_directory", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionList)
		f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki}}
		real := f.page("Real", nil, 0)
		child := f.page("index", &real.ID, 0)
		for _, path := range []string{"index/", "log/", "index.md/", "log.md/", "index/Child/"} {
			if _, err := f.lister(domain.MaxGlobWork).Execute(f.asAlice(), f.eng, path, nil, nil); !errors.Is(err, domain.ErrVirtualFile) {
				t.Errorf("directory %q: %v, want virtual_file", path, err)
			}
		}
		got, err := f.lister(domain.MaxGlobWork).Execute(f.asAlice(), f.eng, "Real/", nil, nil)
		if err != nil || len(got.Items) != 1 || got.Items[0].Node.ID != child.ID {
			t.Fatalf("real directory: %+v, %v", got, err)
		}
	})
	t.Run("after_log", func(t *testing.T) {
		for _, mode := range []domain.Mode{domain.ModeLLMWiki, domain.ModeStandard} {
			f := newFixture()
			f.grant(domain.ActionList)
			f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: mode}}
			first := f.page("First", nil, -3)
			parent := f.page("Parent", nil, 0)
			child := f.page("Child", &parent.ID, -3)
			for pattern, want := range map[string]uuid.UUID{"": first.ID, "Parent/": child.ID} {
				for _, v := range []int{1, 2} {
					if v == 1 && mode == domain.ModeLLMWiki && pattern == "" {
						continue
					}
					cursor, err := shared.EncodeCursor(struct {
						O float64   `json:"o"`
						I uuid.UUID `json:"i"`
						V int       `json:"v,omitempty"`
					}{V: v})
					if err != nil {
						t.Fatal(err)
					}
					one := 1
					got, err := f.lister(domain.MaxGlobWork).Execute(f.asAlice(), f.eng, pattern, &one, &cursor)
					if err != nil || len(got.Items) != 1 || got.Items[0].Node.ID != want {
						t.Errorf("mode %s pattern %q after log: %+v %v; want real first %s", mode, pattern, got, err, want)
					}
				}
			}
		}
	})
	t.Run("glob_order", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionList)
		f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki}}
		a := f.page("A", nil, 9)
		f.page("index", &a.ID, 0)
		f.page("Z", nil, -3)
		for pattern, want := range map[string][]string{"*.md": {"A.md", "index.md", "log.md", "Z.md"}, "**/*.md": {"A.md", "A/index.md", "index.md", "log.md", "Z.md"}, "index.md": {"index.md"}, "log.md": {"log.md"}} {
			var got []string
			var cursor *string
			ended := false
			one := 1
			for range 10 {
				page, err := f.lister(domain.MaxGlobWork).Execute(f.asAlice(), f.eng, pattern, &one, cursor)
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, listed(page)...)
				if page.Next == "" {
					ended = true
					break
				}
				cursor = &page.Next
			}
			if !ended || !slices.Equal(got, want) {
				t.Errorf("glob %q ended=%v: %q want %q", pattern, ended, got, want)
			}
		}
	})
	t.Run("overview", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionList)
		f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki}}
		var root []domain.Node
		for i := range 100 {
			root = append(root, f.page(fmt.Sprintf("P%03d", i), nil, float64(i)))
		}
		for _, n := range []int{98, 99, 100} {
			o := &fakeOverview{recorder: f.rec, pages: int64(n), attachments: 1, root: root[:n]}
			got, err := f.opener(o).Execute(f.asAlice(), f.eng)
			if err != nil || len(got.Root) != 100 || got.Root[0].Path != "index.md" || got.Root[1].Path != "log.md" || got.Root[99].Path != "P097.md" || got.MoreRoot != (n > 98) || got.Pages != int64(n) || got.Attachments != 1 {
				t.Errorf("%d real roots: %+v %v; want prefix within 100, true counts and more=%v", n, got, err, n > 98)
			}
		}
	})

	t.Run("canonical", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionList)
		for _, payload := range []domain.ChildrenCursor{{V: -1}, {V: 3}, {V: 1, O: 1}, {V: 2, I: uuid.NewV7()}} {
			c, err := shared.EncodeCursor(payload)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.lister(domain.MaxGlobWork).Execute(f.asAlice(), f.eng, "", nil, &c); codeOf(err) != "bad_request" {
				t.Errorf("%+v: %v want bad_request", payload, err)
			}
		}
		const old = `{"v":1,"p":{"o":-3,"i":"00000000-0000-0000-0000-000000000000"}}`
		oldBytes := base64.RawURLEncoding.EncodeToString([]byte(old))
		encoded, err := shared.EncodeCursor(domain.ChildrenCursor{O: -3})
		if err != nil || encoded != oldBytes {
			t.Errorf("old cursor bytes changed: %q %v", encoded, err)
		}
		for _, raw := range []string{`{"v":1,"p":{"o":0,"i":"00000000-0000-0000-0000-000000000000","v":1,"v":1}}`, `{"v":1,"p":{"o":0,"i":"00000000-0000-0000-0000-000000000000","v":1,"extra":0}}`, `{"v":1,"p":{"o":0, "i":"00000000-0000-0000-0000-000000000000","v":1}}`} {
			c := base64.RawURLEncoding.EncodeToString([]byte(raw))
			if _, err := f.lister(domain.MaxGlobWork).Execute(f.asAlice(), f.eng, "", nil, &c); codeOf(err) != "bad_request" {
				t.Errorf("noncanonical %s: %v", raw, err)
			}
		}
	})

	t.Run("literal_alias_hint", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionList)
		f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki}}
		parent := f.page("Real", nil, 0)
		f.page("index", &parent.ID, 0)
		for _, pattern := range []string{"index", "log"} {
			got, err := f.lister(domain.MaxGlobWork).Execute(f.asAlice(), f.eng, pattern, nil, nil)
			if err != nil || len(got.Items) != 0 || got.Hint != "" {
				t.Errorf("virtual alias %q: %+v %v; want no misleading children hint", pattern, got, err)
			}
		}
		got, err := f.lister(domain.MaxGlobWork).Execute(f.asAlice(), f.eng, "Real/index", nil, nil)
		if err != nil || got.Hint == "" {
			t.Errorf("real nested alias lost hint: %+v %v", got, err)
		}
		f.notebooks.states[f.eng] = domain.NotebookState{Mode: domain.ModeStandard}
		f.page("index", nil, 0)
		got, err = f.lister(domain.MaxGlobWork).Execute(f.asAlice(), f.eng, "index", nil, nil)
		if err != nil || got.Hint == "" {
			t.Errorf("standard alias lost hint: %+v %v", got, err)
		}
	})

}
