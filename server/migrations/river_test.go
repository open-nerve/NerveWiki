package migrations_test

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/open-nerve/NerveWiki/server/migrations"
)

// riverMigration holds River's own tables (M1/P4 design 3.2).
const riverMigration = "00005_river_main_v2_to_v7.sql"

// The River migration holds, between StatementBegin and StatementEnd, what
// river migrate-get --line main --all --exclude-version 1 prints for the
// River of go.mod, byte for byte: an edit of the file fails, and so does a
// River upgrade that brings a version the file lacks, until a migration of
// its own adds it.
func TestTheRiverMigrationIsRiversOwn(t *testing.T) {
	migrator, err := rivermigrate.New[pgx.Tx](riverpgxv5.New(nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	versions := slices.DeleteFunc(migrator.AllVersions(), func(v rivermigrate.Migration) bool {
		return v.Version == 1 // river_migration: version 5 does without it
	})
	data, err := fs.ReadFile(migrations.FS(), riverMigration)
	if err != nil {
		t.Fatal(err)
	}
	up, down := statementBlocks(t, string(data))

	if want := riverPrint(versions, "up"); up != want {
		t.Errorf("Up differs from River's SQL of versions 2–%d; first difference:\n%s", versions[len(versions)-1].Version, firstDifference(up, want))
	}
	slices.Reverse(versions)
	if want := riverPrint(versions, "down"); down != want {
		t.Errorf("Down differs from River's SQL; first difference:\n%s", firstDifference(down, want))
	}
}

// riverPrint is what river migrate-get prints for versions, in order, in
// direction.
func riverPrint(versions []rivermigrate.Migration, direction string) string {
	parts := make([]string, 0, len(versions))
	for _, v := range versions {
		sql := v.SQLUp
		if direction == "down" {
			sql = v.SQLDown
		}
		// Without --schema, the CLI puts nothing where the schema goes.
		sql = strings.ReplaceAll(sql, "/* TEMPLATE: schema */", "")
		parts = append(parts, fmt.Sprintf("-- River main migration %03d [%s]\n%s\n", v.Version, direction, strings.TrimSpace(sql)))
	}
	return strings.Join(parts, "\n")
}

// statementBlocks returns what the file's two StatementBegin … StatementEnd
// blocks hold, Up then Down.
func statementBlocks(t *testing.T, file string) (string, string) {
	t.Helper()
	const begin, end = "-- +goose StatementBegin\n", "-- +goose StatementEnd\n"
	var blocks []string
	for rest := file; ; {
		_, after, ok := strings.Cut(rest, begin)
		if !ok {
			break
		}
		block, next, ok := strings.Cut(after, end)
		if !ok {
			t.Fatalf("%s: a StatementBegin without its StatementEnd", riverMigration)
		}
		blocks, rest = append(blocks, block), next
	}
	if len(blocks) != 2 {
		t.Fatalf("%s has %d StatementBegin blocks, want 2 (Up, Down)", riverMigration, len(blocks))
	}
	return blocks[0], blocks[1]
}

// firstDifference shows the first line where got and want part.
func firstDifference(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := range min(len(g), len(w)) {
		if g[i] != w[i] {
			return fmt.Sprintf("line %d: got %q, want %q", i+1, g[i], w[i])
		}
	}
	return fmt.Sprintf("got %d lines, want %d", len(g), len(w))
}
