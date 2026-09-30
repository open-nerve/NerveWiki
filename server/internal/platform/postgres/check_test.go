package postgres_test

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// createDatabase creates the database name with options in the cluster of
// the pgtest database behind via, and returns its URL. The database is
// dropped when the test ends.
func createDatabase(t *testing.T, via, name, options string) string {
	t.Helper()
	ctx := context.Background()
	admin := newPool(t, via)
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoted+" "+options); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
	})
	u, err := url.Parse(via)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}

func TestCheckDatabaseAcceptsTheTestDatabase(t *testing.T) {
	pool := newPool(t, pgtest.NewDatabase(t)) // migrated: pg_trgm is installed

	if err := postgres.CheckDatabase(context.Background(), pool); err != nil {
		t.Errorf("CheckDatabase() = %v, want nil", err)
	}
}

func TestCheckDatabaseReportsEveryProblem(t *testing.T) {
	const fix = "create it with: CREATE DATABASE %s TEMPLATE template0 ENCODING 'UTF8' LOCALE_PROVIDER builtin LOCALE 'C.UTF-8'\n" +
		`(with the official image, initialise the cluster with POSTGRES_INITDB_ARGS="--locale-provider=builtin --locale=C.UTF-8")`
	tests := []struct {
		name     string
		options  string
		trgm     bool // install pg_trgm before the check
		problems []string
	}{
		{
			// Ordering and case mapping are right, but pg_trgm cannot tell
			// that Chinese characters are letters (M0/P1 experiment ①).
			name:    "check_ctype_c",
			options: "TEMPLATE template0 LOCALE_PROVIDER builtin BUILTIN_LOCALE 'C.UTF-8' LC_COLLATE 'C' LC_CTYPE 'C'",
			trgm:    true,
			problems: []string{
				`LC_COLLATE is "C", want C.UTF-8`,
				`LC_CTYPE is "C", want C.UTF-8`,
				"pg_trgm splits no trigrams out of Chinese text: show_trgm('中文') is empty",
			},
		},
		{
			// pg_trgm is not installed yet: the other checks still run.
			name:     "check_libc",
			options:  "TEMPLATE template0 LOCALE_PROVIDER libc LOCALE 'C.UTF-8'",
			problems: []string{`locale provider is libc with locale "", want builtin with C.UTF-8`},
		},
		{
			// The builtin C locale maps the case of ASCII letters only.
			name:     "check_builtin_c",
			options:  "TEMPLATE template0 LOCALE_PROVIDER builtin BUILTIN_LOCALE 'C' LC_COLLATE 'C.UTF-8' LC_CTYPE 'C.UTF-8'",
			trgm:     true,
			problems: []string{`locale provider is builtin with locale "C", want builtin with C.UTF-8`},
		},
		{
			name:    "Check SQL_ASCII",
			options: "TEMPLATE template0 ENCODING 'SQL_ASCII' LOCALE_PROVIDER libc LOCALE 'C'",
			problems: []string{
				"encoding is SQL_ASCII, want UTF8",
				`locale provider is libc with locale "", want builtin with C.UTF-8`,
				`LC_COLLATE is "C", want C.UTF-8`,
				`LC_CTYPE is "C", want C.UTF-8`,
			},
		},
	}
	via := pgtest.NewEmptyDatabase(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := newPool(t, createDatabase(t, via, tt.name, tt.options))
			if tt.trgm {
				if _, err := pool.Exec(context.Background(), "CREATE EXTENSION pg_trgm"); err != nil {
					t.Fatal(err)
				}
			}

			err := postgres.CheckDatabase(context.Background(), pool)
			want := "database " + tt.name + " is not set up for nervewiki:\n- " + strings.Join(tt.problems, "\n- ") + "\n" +
				fmt.Sprintf(fix, `"`+tt.name+`"`) // quoted as an identifier
			if err == nil || err.Error() != want {
				t.Errorf("CheckDatabase() =\n%v\nwant\n%s", err, want)
			}
		})
	}
}
