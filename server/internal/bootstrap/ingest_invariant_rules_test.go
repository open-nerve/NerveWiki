package bootstrap

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The purge has no tombstone on a source. Its transient count gap is legal only
// for the exact before used by that run, after both kinds of history are gone.
func ingestViolations(t testing.TB, pool *pgxpool.Pool, before *time.Time) []string {
	t.Helper()
	type rule struct {
		name, sql string
		args      []any
	}
	var cutoff any
	if before != nil {
		cutoff = *before
	}
	rules := []rule{
		{"ingest sources without a changeset", `SELECT DISTINCT s.changeset_id::text FROM changeset_sources s
			LEFT JOIN changesets c ON c.id=s.changeset_id WHERE c.id IS NULL ORDER BY 1`, nil},
		{"ingest count differs from persistent sources", `SELECT c.id::text FROM changesets c
			WHERE c.ingest_count <> (SELECT count(*) FROM changeset_sources s WHERE s.changeset_id=c.id)
			AND NOT ($1::timestamptz IS NOT NULL AND
				((c.deleted_at IS NOT NULL AND c.deleted_at < $1) OR (c.trimmed AND c.deleted_at IS NULL))
				AND NOT EXISTS (SELECT 1 FROM changeset_items i WHERE i.changeset_id=c.id)
				AND NOT EXISTS (SELECT 1 FROM page_revisions r WHERE r.changeset_id=c.id))
			ORDER BY 1`, []any{cutoff}},
		{"ingest sources on a non-edit changeset", `SELECT DISTINCT c.id::text FROM changesets c
			JOIN changeset_sources s ON s.changeset_id=c.id WHERE c.kind <> 'edit' ORDER BY 1`, nil},
	}
	var out []string
	for _, r := range rules {
		if ids := queryStrings(t, pool, r.sql, r.args...); len(ids) > 0 {
			out = append(out, fmt.Sprintf("%s: changesets [%s]", r.name, strings.Join(ids, ", ")))
		}
	}
	return out
}
