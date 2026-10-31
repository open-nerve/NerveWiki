package domain_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func TestLintSources(t *testing.T) {
	t.Run("first-key-and-json-items", func(t *testing.T) {
		id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		for _, tt := range []struct {
			key, value, path, target string
			array                    bool
			items                    int32
			slots                    bool
			want                     []domain.SourceJudgment
		}{
			{"Sources", `"[[raw/a]]"`, "Sources", "raw/a", false, 0, true, []domain.SourceJudgment{{FullText: "raw/a", ResolvedID: &id}}},
			{"SoUrCeS", `["[[raw/a]]"]`, "sources.0", "raw/a", true, 1, true, []domain.SourceJudgment{{NotLink: true, FullText: `"[[raw/a]]"`}}},
			{"sources", `"plain"`, "", "", false, 0, true, []domain.SourceJudgment{{NotLink: true, FullText: `"plain"`}}},
			{"sources", `[]`, "", "", true, 0, false, []domain.SourceJudgment{}},
			{"sources", `[42,true,null,["[[raw/a]]"],{"0":"[[raw/a]]"}]`, "sources.3", "raw/a", true, 5, false, []domain.SourceJudgment{{NotLink: true, FullText: `42`}, {NotLink: true, FullText: `true`}, {NotLink: true, FullText: `null`}, {NotLink: true, FullText: `["[[raw/a]]"]`}, {NotLink: true, FullText: `{"0":"[[raw/a]]"}`}}},
			{"sources", `null`, "sources", "raw/a", false, 0, false, []domain.SourceJudgment{{NotLink: true, FullText: `null`}}},
		} {
			s, err := domain.NewSourceSlots(t.Context(), tt.key, []byte(tt.value))
			if err != nil {
				t.Fatal(err)
			}
			a, n, has := s.QueryShape()
			if a != tt.array || n != tt.items || has != tt.slots {
				t.Fatalf("shape %s = %t,%d,%t", tt.value, a, n, has)
			}
			if tt.path != "" {
				if err = s.Observe(&tt.path, tt.target, &id, false); err != nil {
					t.Fatal(err)
				}
			}
			got, err := s.Finish(t.Context())
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("sources JSON %s key %s = %+v,%v; want %+v", tt.value, tt.key, got, err, tt.want)
			}
		}
	})
	t.Run("mixed-string-and-nested-slots", func(t *testing.T) {
		s, err := domain.NewSourceSlots(t.Context(), "sources", []byte(`["plain",["[[raw/a]]"]]`))
		if err != nil {
			t.Fatal(err)
		}
		array, items, slots := s.QueryShape()
		if !array || items != 2 || !slots {
			t.Fatalf("mixed array must select its real edges: %t,%d,%t", array, items, slots)
		}
		path := "sources.1"
		if err = s.Observe(&path, "raw/a", nil, false); err != nil {
			t.Fatal(err)
		}
		got, err := s.Finish(t.Context())
		want := []domain.SourceJudgment{{NotLink: true, FullText: `"plain"`}, {NotLink: true, FullText: `["[[raw/a]]"]`}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("nested slot must remain J despite its observed edge: %+v,%v", got, err)
		}
	})
	t.Run("slot-collision-unfiltered-cardinality", func(t *testing.T) {
		page := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		foreign := uuid.MustParse("00000000-0000-0000-0000-000000000002")
		for _, tt := range []struct {
			name  string
			id    *uuid.UUID
			asset bool
		}{{"NULL", nil, false}, {"outside", &foreign, false}, {"asset", &foreign, true}} {
			t.Run(tt.name, func(t *testing.T) {
				s, err := domain.NewSourceSlots(t.Context(), "Sources", []byte(`["[[raw/a]]"]`))
				if err != nil {
					t.Fatal(err)
				}
				path := "Sources.0"
				if err = s.Observe(&path, "raw/a", &page, false); err != nil {
					t.Fatal(err)
				}
				// A later batch contains the colliding dotted key's real edge.
				if err = s.Observe(&path, "raw/b", tt.id, tt.asset); err != nil {
					t.Fatal(err)
				}
				got, err := s.Finish(t.Context())
				want := []domain.SourceJudgment{{NotLink: true, FullText: `"[[raw/a]]"`}}
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("unfiltered collision %s = %+v,%v; want J and no resolved target", tt.name, got, err)
				}
			})
		}
	})
	t.Run("old-qualified-json-oracle", func(t *testing.T) {
		id := uuid.UUID{}
		path := "sources.0"
		for _, tt := range []struct {
			value string
			want  domain.SourceJudgment
			slots bool
		}{{`{"0":"[[raw/s]]"}`, domain.SourceJudgment{NotLink: true, FullText: `{"0":"[[raw/s]]"}`}, false}, {`["[[raw/s]]"]`, domain.SourceJudgment{FullText: "raw/s", ResolvedID: &id}, true}} {
			s, err := domain.NewSourceSlots(t.Context(), "sources", []byte(tt.value))
			if err != nil {
				t.Fatal(err)
			}
			_, _, slots := s.QueryShape()
			if err = s.Observe(&path, "raw/s", &id, false); err != nil {
				t.Fatal(err)
			}
			got, err := s.Finish(t.Context())
			if err != nil || !reflect.DeepEqual(got, []domain.SourceJudgment{tt.want}) {
				t.Fatalf("old/new independent oracle %s = %+v,%v", tt.value, got, err)
			}
			if slots != tt.slots {
				t.Fatalf("object/array query shape %s = %t", tt.value, slots)
			}
		}
	})
	t.Run("stable-json-number-and-order", func(t *testing.T) {
		s, err := domain.NewSourceSlots(t.Context(), "sources", []byte(`{"z":9007199254740993,"a":1e+30,"nested":{"b":1.230,"a":null}}`))
		if err != nil {
			t.Fatal(err)
		}
		got, err := s.Finish(t.Context())
		want := []domain.SourceJudgment{{NotLink: true, FullText: `{"a":1e+30,"nested":{"a":null,"b":1.230},"z":9007199254740993}`}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("stable JSON = %+v,%v", got, err)
		}
	})
	t.Run("path-and-owned-results", func(t *testing.T) {
		id := uuid.UUID{}
		s, err := domain.NewSourceSlots(t.Context(), "Sources", []byte(`["[[x]]","same","same"]`))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"Sources.00", "Sources.-1", "Sources.3", "Sources.0.x", "sources.0", "Sources." + strings.Repeat("x", 5000)} {
			if err = s.Observe(&path, "ignored", &id, false); err != nil {
				t.Fatal(err)
			}
		}
		if err = s.Observe(nil, "body", &id, false); err != nil {
			t.Fatal(err)
		}
		path := "Sources.0"
		if err = s.Observe(&path, "x", &id, false); err != nil {
			t.Fatal(err)
		}
		id[15] = 1
		got, err := s.Finish(t.Context())
		zero := uuid.UUID{}
		want := []domain.SourceJudgment{{FullText: "x", ResolvedID: &zero}, {NotLink: true, FullText: `"same"`}, {NotLink: true, FullText: `"same"`}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("slot ownership/path = %+v,%v", got, err)
		}
		if _, err = s.Finish(t.Context()); err == nil {
			t.Fatal("Finish reentry accepted")
		}
		if err = s.Observe(&path, "x", nil, false); err == nil {
			t.Fatal("Observe after Finish accepted")
		}
	})
	t.Run("cancel-parse-and-stable-text", func(t *testing.T) {
		base, cancel := context.WithCancel(t.Context())
		ctx := &lintCancelContext{Context: base, cancel: cancel, left: 3}
		if s, err := domain.NewSourceSlots(ctx, "sources", []byte(`"`+strings.Repeat("a", 40000)+`"`)); s != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("parse cancel = %v,%v", s, err)
		}
		s, err := domain.NewSourceSlots(t.Context(), "sources", []byte(`{"a":"`+strings.Repeat("中", 20000)+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		base, cancel = context.WithCancel(t.Context())
		ctx = &lintCancelContext{Context: base, cancel: cancel, left: 4}
		if got, err := s.Finish(ctx); got != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("stable text cancel = %d,%v", len(got), err)
		}
	})
}
