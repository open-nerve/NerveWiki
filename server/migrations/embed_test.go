package migrations

import (
	"io/fs"
	"regexp"
	"testing"
)

var migrationName = regexp.MustCompile(`^\d{5}_[a-z][a-z0-9]*_[a-z0-9_]+\.sql$`)

func TestMigrationFilesFollowNamingConvention(t *testing.T) {
	entries, err := fs.ReadDir(FS(), ".")
	if err != nil || len(entries) == 0 {
		t.Fatalf("read migrations = %d entries, %v; want some", len(entries), err)
	}
	for _, e := range entries {
		if e.IsDir() || !migrationName.MatchString(e.Name()) {
			t.Errorf("%s: want NNNNN_<owner>_<description>.sql, e.g. 00001_platform_pg_trgm.sql", e.Name())
		}
	}
}
