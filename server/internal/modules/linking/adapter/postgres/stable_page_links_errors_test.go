package postgresadapter_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

type stableAdapterErrorIdentity struct {
	Code, Constraint, Table, Column string
}

func stableAdapterCopyFacts(in domain.Facts) domain.Facts {
	out := in
	out.Links = slices.Clone(in.Links)
	out.Tags = slices.Clone(in.Tags)
	out.Aliases = slices.Clone(in.Aliases)
	out.Properties = slices.Clone(in.Properties)
	for i := range out.Properties {
		out.Properties[i].Value = slices.Clone(in.Properties[i].Value)
	}
	return out
}

// Run the original public DeletePages/AddPage sequence and the new Replace
// against the same committed starting state. Each real database error rolls
// back through the original manager; compare the actual first failing SQL,
// SQLSTATE, constraint and preserved error object, never just non-nil errors.
func TestStablePageLinksPreserveLegacyErrorsAndTheirOrder(t *testing.T) {
	for _, tc := range []struct {
		name, code, constraint, failedSQL string
		prepare                           func(*app.Page, *domain.Facts)
		fast                              bool
	}{
		{name: "duplicate_identical", code: "23505", constraint: "page_links_pkey", failedSQL: "InsertLinks",
			prepare: func(_ *app.Page, f *domain.Facts) { f.Links = append(f.Links, f.Links[0]) }},
		{name: "duplicate_different", code: "23505", constraint: "page_links_pkey", failedSQL: "InsertLinks",
			prepare: func(_ *app.Page, f *domain.Facts) {
				l := f.Links[0]
				l.Target = "Other.md"
				f.Links = append(f.Links, l)
			}},
		{name: "int32_collision", code: "23505", constraint: "page_links_pkey", failedSQL: "InsertLinks",
			prepare: func(_ *app.Page, f *domain.Facts) {
				l := f.Links[0]
				large := int64(1) << 32
				l.Start, l.End = int(large)+20, int(large)+27
				f.Links = append(f.Links, l)
			}},
		{name: "head_before_link_and_json", code: "23514", constraint: "indexed_pages_revision_check", failedSQL: "InsertIndexedPage",
			prepare: func(p *app.Page, f *domain.Facts) {
				p.Revision = 0
				f.Links[0].Kind = "bad"
				f.Properties[0].Value = []byte(`{`)
			}},
		{name: "negative_start", code: "23514", constraint: "page_links_range_start_check", failedSQL: "InsertLinks",
			prepare: func(_ *app.Page, f *domain.Facts) { f.Links[0].Start = -1 }},
		{name: "empty_range", code: "23514", constraint: "page_links_range_check", failedSQL: "InsertLinks",
			prepare: func(_ *app.Page, f *domain.Facts) { f.Links[0].End = f.Links[0].Start }},
		{name: "kind_before_json", code: "23514", constraint: "page_links_kind_check", failedSQL: "InsertLinks",
			prepare: func(_ *app.Page, f *domain.Facts) { f.Links[0].Kind = "bad"; f.Properties[0].Value = []byte(`{`) }},
		{name: "empty_target_before_json", code: "23514", constraint: "page_links_target_check", failedSQL: "InsertLinks",
			prepare: func(_ *app.Page, f *domain.Facts) { f.Links[0].Target = ""; f.Properties[0].Value = []byte(`{`) }},
		{name: "target_nul", code: "22021", failedSQL: "InsertLinks",
			prepare: func(_ *app.Page, f *domain.Facts) { f.Links[0].Target = "N\x00ote" }},
		{name: "target_invalid_utf8", code: "22021", failedSQL: "InsertLinks",
			prepare: func(_ *app.Page, f *domain.Facts) { f.Links[0].Target = string([]byte{0xff}) }},
		{name: "duplicate_tags_before_json", code: "23505", constraint: "page_tags_pkey", failedSQL: "InsertTags",
			prepare: func(_ *app.Page, f *domain.Facts) {
				f.Tags = append(f.Tags, f.Tags[0])
				f.Properties[0].Value = []byte(`{`)
			}},
		{name: "zero_tag_count", code: "23514", constraint: "page_tags_count_check", failedSQL: "InsertTags",
			prepare: func(_ *app.Page, f *domain.Facts) { f.Tags[0].Count = 0 }},
		{name: "duplicate_aliases", code: "23505", constraint: "page_aliases_pkey", failedSQL: "InsertAliases",
			prepare: func(_ *app.Page, f *domain.Facts) { f.Aliases = append(f.Aliases, f.Aliases[0]) }},
		{name: "empty_alias_key", code: "23514", constraint: "page_aliases_alias_key_check", failedSQL: "InsertAliases",
			prepare: func(_ *app.Page, f *domain.Facts) { f.Aliases[0].Key = "" }},
		{name: "postgres_jsonb_unicode", code: "22P05", failedSQL: "InsertProperties", fast: true,
			prepare: func(_ *app.Page, f *domain.Facts) { f.Properties[0].Value = []byte(`"\u0000"`) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStableAdapterFixture(t)
			p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
			a := uuid.NewV7()
			facts := domain.Facts{FrontmatterValid: true, Links: []domain.Link{stableAdapterInput(20)},
				Tags:       []domain.Tag{{Key: "tag", Name: "Tag", Count: 2}},
				Properties: []domain.Property{{Key: "p", Value: []byte(`{"old":true}`)}},
				Aliases:    []domain.Alias{{Key: "old", Name: "Old"}},
			}
			f.seed(t, p, facts, []app.Link{{SourceID: p.ID, Start: 20, Resolution: domain.Resolution{ID: a, Ambiguous: true, Asset: true}}})
			before := f.snapshot(t, t.Context(), nil)
			p.Revision = 2
			tc.prepare(&p, &facts)
			var legacyIdentity stableAdapterErrorIdentity
			for _, legacy := range []bool{true, false} {
				incoming := stableAdapterCopyFacts(facts)
				pid, err := f.run(t, p.NotebookID, func(ctx context.Context) error {
					if legacy {
						if _, err := f.s.DeletePages(ctx, []uuid.UUID{p.ID}); err != nil {
							return err
						}
						return f.s.AddPage(ctx, p, incoming)
					}
					dropped, err := f.s.ReplacePage(ctx, p, incoming)
					if err != nil && !reflect.DeepEqual(dropped, app.Dropped{}) {
						t.Errorf("failed Replace returned a partial drop: %+v", dropped)
					}
					return err
				})
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) {
					t.Fatalf("legacy=%t returned %v, want a real PostgreSQL %s error", legacy, err, tc.code)
				}
				identity := stableAdapterErrorIdentity{Code: pgErr.Code, Constraint: pgErr.ConstraintName, Table: pgErr.TableName, Column: pgErr.ColumnName}
				if identity.Code != tc.code || identity.Constraint != tc.constraint || !strings.HasPrefix(err.Error(), "index page "+p.ID.String()+": ") {
					t.Errorf("legacy=%t error identity=%+v, wrapped error=%v; want code=%s constraint=%q in original index-page stage", legacy, identity, err, tc.code, tc.constraint)
				}
				if legacy {
					legacyIdentity = identity
				} else if identity != legacyIdentity {
					t.Errorf("error changed from legacy %+v to %+v", legacyIdentity, identity)
				}
				want := stableAdapterLegacySQL()
				if !legacy && tc.fast {
					want = stableAdapterFastSQL()
				}
				at := slices.Index(want, tc.failedSQL)
				if at < 0 {
					t.Fatal("fixture failure SQL is absent from its literal expected sequence")
				}
				f.assertSQL(t, want[:at+1], pid, tc.failedSQL, err)
				if after := f.snapshot(t, t.Context(), nil); after != before {
					t.Errorf("legacy=%t did not restore complete tables: before=%s after=%s", legacy, before, after)
				}
				if !reflect.DeepEqual(incoming, facts) {
					t.Errorf("legacy=%t mutated the original Facts input", legacy)
				}
			}
		})
	}
}
