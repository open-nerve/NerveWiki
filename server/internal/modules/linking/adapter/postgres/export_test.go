package postgresadapter_test

import (
	"context"
	"slices"
	"testing"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// An export asks which of its pages without content a link of its pages
// leads to: a link of a page outside its sources does not count, nor one
// to an attachment; a property's link does. The sources grow with the
// notebook: the statement is planned with them, never cached.
func TestAnExportFindsThePagesLinksLeadTo(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := postgresadapter.New(pool)
	s1, s2, outside := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	body, other, property, asset, none := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	for i, l := range []struct {
		source, target uuid.UUID
		property       *string
		asset          bool
	}{
		{s1, body, nil, false},
		{s1, body, nil, false},
		{outside, other, nil, false},
		{s2, property, new("sources.0"), false},
		{s1, asset, nil, true},
	} {
		_, err := f.pool.Exec(ctx, `INSERT INTO page_links (source_id, range_start, range_end, notebook_id, kind, property_key, target,
			target_key, resolved_id, ambiguous, resolved_asset) VALUES ($1, $2, $2 + 1, $3, 'wikilink', $4, 'x', 'x', $5, false, $6)`,
			l.source, i*10, f.eng, l.property, l.target, l.asset)
		if err != nil {
			t.Fatal(err)
		}
	}
	for range 8 {
		got, err := s.LinkedPages(ctx, []uuid.UUID{s1, s2}, []uuid.UUID{body, other, property, asset, none})
		slices.SortFunc(got, func(a, b uuid.UUID) int { return a.Compare(b) })
		want := []uuid.UUID{body, property}
		slices.SortFunc(want, func(a, b uuid.UUID) int { return a.Compare(b) })
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("LinkedPages() = %v, %v; want %v", got, err, want)
		}
	}
	if got, err := s.LinkedPages(ctx, nil, []uuid.UUID{body}); err != nil || len(got) != 0 {
		t.Errorf("LinkedPages() of no source = %v, %v", got, err)
	}
	if cached := pgtest.CachedStatements(t, pool); slices.Contains(cached, "LinkedPages") {
		t.Errorf("the cached statements are %v: want LinkedPages planned each time", cached)
	}
}
