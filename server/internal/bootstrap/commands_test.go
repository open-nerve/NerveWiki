package bootstrap

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

func runCommand(t *testing.T, dbURL string, cmd migrationCommand) string {
	t.Helper()
	var out bytes.Buffer
	if err := runMigration(context.Background(), testConfig(t, dbURL, false), sampleMigrations, &out, cmd); err != nil {
		t.Fatalf("command error = %v", err)
	}
	return out.String()
}

func TestMigrateCommandsOutput(t *testing.T) {
	db := pgtest.NewEmptyDatabase(t)

	status := runCommand(t, db, migrateStatus)
	wantPending := "VERSION  STATE    APPLIED AT  SOURCE\n" +
		"1        pending  -           00001_probe_create_widgets.sql\n" +
		"2        pending  -           00002_probe_create_gadgets.sql\n"
	if status != wantPending {
		t.Errorf("status before up:\n%s\nwant:\n%s", status, wantPending)
	}

	if got, want := runCommand(t, db, migrateUp), "applied 00001_probe_create_widgets.sql\napplied 00002_probe_create_gadgets.sql\n"; got != want {
		t.Errorf("up = %q, want %q", got, want)
	}
	if got, want := runCommand(t, db, migrateUp), "no pending migrations\n"; got != want {
		t.Errorf("second up = %q, want %q", got, want)
	}

	applied := regexp.MustCompile(`^VERSION +STATE +APPLIED AT +SOURCE\n` +
		`1 +applied +\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ +00001_probe_create_widgets\.sql\n` +
		`2 +applied +\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ +00002_probe_create_gadgets\.sql\n$`)
	if status := runCommand(t, db, migrateStatus); !applied.MatchString(status) {
		t.Errorf("status after up:\n%s", status)
	}

	if got, want := runCommand(t, db, migrateDown), "rolled back 00002_probe_create_gadgets.sql\n"; got != want {
		t.Errorf("down = %q, want %q", got, want)
	}
	runCommand(t, db, migrateDown)
	if got, want := runCommand(t, db, migrateDown), "no applied migrations to roll back\n"; got != want {
		t.Errorf("down with nothing applied = %q, want %q", got, want)
	}
}

func TestMigrateUpReportsMigrationsAppliedBeforeAFailure(t *testing.T) {
	files := fstest.MapFS{
		"00001_probe_create_widgets.sql": sampleMigrations["00001_probe_create_widgets.sql"],
		"00002_probe_broken.sql":         {Data: []byte("-- +goose Up\nCREATE TABLE broken (;\n")},
	}
	var out bytes.Buffer

	err := runMigration(context.Background(), testConfig(t, pgtest.NewEmptyDatabase(t), false), files, &out, migrateUp)

	if err == nil {
		t.Error("migrate up error = nil, want the failing migration's error")
	}
	if got, want := out.String(), "applied 00001_probe_create_widgets.sql\n"; got != want {
		t.Errorf("migrate up output = %q, want %q", got, want)
	}
}

func TestMigrateCommandsWithoutMigrations(t *testing.T) {
	cfg := testConfig(t, pgtest.NewDatabase(t), false)
	tests := []struct {
		name string
		cmd  migrationCommand
		want string
	}{
		{"status", migrateStatus, "no migrations\n"},
		{"up", migrateUp, "no pending migrations\n"},
		{"down", migrateDown, "no applied migrations to roll back\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			// An empty set of its own: the production set is not empty.
			if err := runMigration(context.Background(), cfg, fstest.MapFS{}, &out, tt.cmd); err != nil {
				t.Fatalf("error = %v", err)
			}
			if out.String() != tt.want {
				t.Errorf("output = %q, want %q", out.String(), tt.want)
			}
		})
	}
}

func TestMigrateCommandReportsBadURL(t *testing.T) {
	err := MigrateStatus(context.Background(), testConfig(t, "postgres://nervewiki:secret@localhost:notaport/nervewiki", false), &bytes.Buffer{})
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Errorf("MigrateStatus() = %v, want a database.url error without the password", err)
	}
}

// migrate up checks the database after migrating, and reports what it
// applied before refusing.
func TestMigrateUpChecksTheDatabase(t *testing.T) {
	var out bytes.Buffer

	err := runMigration(context.Background(), testConfig(t, pgtest.NewEmptyDatabaseWith(t, ctypeC), false), sampleMigrations, &out, migrateUp)

	if err == nil || !strings.Contains(err.Error(), "LC_CTYPE is \"C\", want C.UTF-8") {
		t.Errorf("migrate up error = %v, want the database check's error", err)
	}
	if got, want := out.String(), "applied 00001_probe_create_widgets.sql\napplied 00002_probe_create_gadgets.sql\n"; got != want {
		t.Errorf("migrate up output = %q, want %q", got, want)
	}
}
