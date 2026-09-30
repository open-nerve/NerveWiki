package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// The encoding and locale nervewiki relies on (v0.1 design 7.1).
const (
	wantEncoding = "UTF8"
	wantLocale   = "C.UTF-8"
)

// providerName spells out pg_database.datlocprovider.
func providerName(code string) string {
	switch code {
	case "b":
		return "builtin"
	case "c":
		return "libc"
	case "i":
		return "icu"
	default:
		return code
	}
}

// CheckDatabase verifies the settings that ordering, case mapping and trigram
// search rely on (v0.1 design 7.1), and reports every problem at once, with
// the statement that creates a correct database:
//
//   - the encoding is UTF8;
//   - the locale provider is builtin with locale C.UTF-8: ordering and case
//     mapping (lower, ILIKE, regular expressions) then do not change with
//     glibc or ICU;
//   - LC_COLLATE and LC_CTYPE are C.UTF-8: pg_trgm tells letters by LC_CTYPE,
//     and splits no trigrams out of Chinese text under C;
//   - once pg_trgm is installed, it does split Chinese text: the letter
//     classes also come from the image's C library.
//
// The first migration installs pg_trgm. Before it has run, which only happens
// when prod serves without migrate up, the last check is skipped and /readyz
// reports the pending migrations.
func CheckDatabase(ctx context.Context, db Querier) error {
	var (
		name, encoding, provider, collate, ctype string
		locale, trgmSchema                       *string // null for the libc provider; null until pg_trgm is installed
	)
	err := db.QueryRow(ctx, `
		SELECT datname, pg_encoding_to_char(encoding), datlocprovider::text, datlocale, datcollate, datctype,
			(SELECT extnamespace::regnamespace::text FROM pg_extension WHERE extname = 'pg_trgm')
		FROM pg_database WHERE datname = current_database()`,
	).Scan(&name, &encoding, &provider, &locale, &collate, &ctype, &trgmSchema)
	if err != nil {
		return fmt.Errorf("check database: %w", err)
	}

	var problems []string
	if encoding != wantEncoding {
		problems = append(problems, fmt.Sprintf("encoding is %s, want %s", encoding, wantEncoding))
	}
	if provider != "b" || locale == nil || *locale != wantLocale {
		problems = append(problems, fmt.Sprintf("locale provider is %s with locale %q, want builtin with %s",
			providerName(provider), deref(locale), wantLocale))
	}
	if collate != wantLocale {
		problems = append(problems, fmt.Sprintf("LC_COLLATE is %q, want %s", collate, wantLocale))
	}
	if ctype != wantLocale {
		problems = append(problems, fmt.Sprintf("LC_CTYPE is %q, want %s", ctype, wantLocale))
	}
	if trgmSchema != nil {
		var trigrams int
		// regnamespace's text form is already quoted where it needs to be.
		query := "SELECT cardinality(" + *trgmSchema + ".show_trgm('中文'))"
		if err := db.QueryRow(ctx, query).Scan(&trigrams); err != nil {
			return fmt.Errorf("check database: %w", err)
		}
		if trigrams == 0 {
			problems = append(problems, "pg_trgm splits no trigrams out of Chinese text: show_trgm('中文') is empty")
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("database %s is not set up for nervewiki:\n- %s\n"+
		"create it with: CREATE DATABASE %s TEMPLATE template0 ENCODING 'UTF8' LOCALE_PROVIDER builtin LOCALE 'C.UTF-8'\n"+
		"(with the official image, initialise the cluster with POSTGRES_INITDB_ARGS=\"--locale-provider=builtin --locale=C.UTF-8\")",
		name, strings.Join(problems, "\n- "), pgx.Identifier{name}.Sanitize())
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
