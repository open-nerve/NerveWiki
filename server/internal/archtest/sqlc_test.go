package archtest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/knadh/koanf/parsers/yaml"
)

// TestSQLCSchemaScope holds server/sqlc.yaml to v0.1 design 7.1: a module's
// queries see only its own migrations, so a query can touch only its own
// tables; sqlc itself then rejects the rest.
func TestSQLCSchemaScope(t *testing.T) {
	registerSources(t)
	entries := readSQLCEntries(t, filepath.Join(moduleRoot, "sqlc.yaml"))
	migrations := readMigrations(t, filepath.Join(moduleRoot, "migrations", "sql"))
	queryModules, err := filepath.Glob(filepath.Join(moduleRoot, "internal", "modules", "*", "adapter", "postgres", "queries"))
	if err != nil {
		t.Fatal(err)
	}
	var modules []string
	for _, dir := range queryModules {
		modules = append(modules, filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(dir)))))
	}
	if len(entries) == 0 || len(migrations) == 0 || len(modules) == 0 {
		t.Fatalf("read %d sqlc entries, %d migrations, %d modules with queries; want some of each", len(entries), len(migrations), len(modules))
	}
	for _, v := range sqlcScopeViolations(entries, migrations, modules) {
		t.Error(v)
	}
}

// sqlcEntry is one sql entry of sqlc.yaml, paths relative to server/.
type sqlcEntry struct {
	schema  []string
	queries string
	out     string
}

func readSQLCEntries(t *testing.T, path string) []sqlcEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := yaml.Parser().Unmarshal(data)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	raw, _ := doc["sql"].([]any)
	var entries []sqlcEntry
	for i, item := range raw {
		m, _ := item.(map[string]any)
		gen, _ := m["gen"].(map[string]any)
		goGen, _ := gen["go"].(map[string]any)
		e := sqlcEntry{queries: fmt.Sprint(m["queries"]), out: fmt.Sprint(goGen["out"])}
		schema, ok := m["schema"].([]any)
		if !ok {
			t.Fatalf("%s: sql[%d].schema is not a list of migration files", path, i)
		}
		for _, s := range schema {
			e.schema = append(e.schema, fmt.Sprint(s))
		}
		entries = append(entries, e)
	}
	return entries
}

// migrationFile is one migration: its file name and SQL.
type migrationFile struct {
	name string
	sql  string
}

func readMigrations(t *testing.T, dir string) []migrationFile {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	var out []migrationFile
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, migrationFile{name: filepath.Base(f), sql: string(data)})
	}
	return out
}

// A table name as a migration writes it: unquoted (folded to lower case) or
// quoted, in the schema public or without one. ident is any other name, e.g.
// an index's or a trigger's.
const (
	ident     = `(?:"[^"]+"|[a-z_][a-z0-9_$]*)`
	tableName = `(?:(?:public|"public")\.)?("[^"]+"|[a-z_][a-z0-9_]*)`
)

var (
	migrationFileName = regexp.MustCompile(`^\d{5}_([a-z][a-z0-9]*)_[a-z0-9_]+\.sql$`)
	sqlComment        = regexp.MustCompile(`--[^\n]*`)
	createTable       = regexp.MustCompile(`(?i)\bCREATE\s+(?:UNLOGGED\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + tableName)
	renameTable       = regexp.MustCompile(`(?i)\bALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?` + tableName + `\s+RENAME\s+TO\s+` + tableName)
	moduleQueries     = regexp.MustCompile(`^internal/modules/([a-z][a-z0-9]*)/adapter/postgres/queries$`)
)

// The statements that act on a table, which must be one the migration's
// module creates (v0.1 design 7.1, 13.1). REFERENCES is not among
// them: a foreign key to another module's table is allowed, since no query
// can read across modules anyway. A DROP TABLE may name several tables.
//
// The rule reads these forms only. It does not see a table touched by a
// policy (CREATE, ALTER or DROP POLICY … ON), COMMENT ON, TRUNCATE, a CREATE
// TABLE … LIKE, INHERITS or PARTITION OF, a statement that names an index
// without its table (ALTER INDEX, DROP INDEX), a data statement (INSERT,
// UPDATE, DELETE) or a DO block; review catches those.
func tableStatements() []tableStatement {
	return []tableStatement{
		{verb: "alters", re: alterTable},
		{verb: "indexes", re: createIndex},
		{verb: "puts a trigger on", re: createTrigger},
		{verb: "alters a trigger on", re: alterTrigger},
		{verb: "drops a trigger on", re: dropTrigger},
		{verb: "drops", re: dropTable, more: true},
	}
}

// tableStatement is one kind of statement that acts on a table.
type tableStatement struct {
	verb string
	re   *regexp.Regexp
	// more: the regexp's second group is the ", t2, t3" after the first
	// table, which DROP TABLE may name.
	more bool
}

var (
	alterTable  = regexp.MustCompile(`(?i)\bALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?` + tableName)
	createIndex = regexp.MustCompile(`(?i)\bCREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:CONCURRENTLY\s+)?(?:IF\s+NOT\s+EXISTS\s+)?(?:` + ident +
		`\s+)?ON\s+(?:ONLY\s+)?` + tableName)
	createTrigger = regexp.MustCompile(`(?is)\bCREATE\s+(?:OR\s+REPLACE\s+)?(?:CONSTRAINT\s+)?TRIGGER\s+` + ident +
		`\s.*?\bON\s+(?:ONLY\s+)?` + tableName)
	alterTrigger = regexp.MustCompile(`(?i)\bALTER\s+TRIGGER\s+` + ident + `\s+ON\s+` + tableName)
	dropTrigger  = regexp.MustCompile(`(?i)\bDROP\s+TRIGGER\s+(?:IF\s+EXISTS\s+)?` + ident + `\s+ON\s+` + tableName)
	dropTable    = regexp.MustCompile(`(?i)\bDROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?` + tableName + `((?:\s*,\s*` + tableName + `)*)`)
)

// moreTables reads the ", t2, t3" after the first table of a DROP TABLE.
var moreTables = regexp.MustCompile(`(?i)` + tableName)

// table is a table name as PostgreSQL resolves it: a quoted name as written,
// an unquoted one in lower case.
func table(name string) string {
	if unquoted, ok := strings.CutPrefix(name, `"`); ok {
		return strings.TrimSuffix(unquoted, `"`)
	}
	return strings.ToLower(name)
}

// touched lists the tables each kind of statement in sql acts on, once each
// per kind: Up and Down both act on them.
func touched(sql string) map[string][]string {
	out := map[string][]string{}
	for _, st := range tableStatements() {
		for _, m := range st.re.FindAllStringSubmatch(sql, -1) {
			names := []string{m[1]}
			if st.more {
				for _, more := range moreTables.FindAllStringSubmatch(m[2], -1) {
					names = append(names, more[1])
				}
			}
			for _, n := range names {
				if t := table(n); !slices.Contains(out[st.verb], t) {
					out[st.verb] = append(out[st.verb], t)
				}
			}
		}
	}
	return out
}

// sqlcScopeViolations checks (v0.1 design 7.1):
//   - migration files are named <version>_<module>_<content>.sql;
//   - the target of every ALTER TABLE, CREATE [UNIQUE] INDEX, CREATE, ALTER
//     and DROP TRIGGER, and DROP TABLE, quoted or not, is created by the
//     file name's module, so a migration belongs to the module that owns the
//     table it changes; a table renamed into existence belongs to the module
//     that renames it;
//   - each sqlc entry is a module's (queries in its adapter/postgres) and
//     lists exactly that module's migrations: no other module's, and never
//     River's, which no module queries;
//   - every module with queries has an entry.
func sqlcScopeViolations(entries []sqlcEntry, migrations []migrationFile, queryModules []string) []string {
	var found []string
	report := func(format string, args ...any) { found = append(found, fmt.Sprintf(format, args...)) }

	moduleOf := map[string]string{}   // migration file → module
	byModule := map[string][]string{} // module → its migrations, as sqlc.yaml lists them
	owner := map[string]string{}      // table → module that creates it
	for _, m := range migrations {
		match := migrationFileName.FindStringSubmatch(m.name)
		if match == nil {
			report("migration %s is not named <version>_<module>_<content>.sql", m.name)
			continue
		}
		moduleOf[m.name] = match[1]
		byModule[match[1]] = append(byModule[match[1]], "migrations/sql/"+m.name)
		sql := sqlComment.ReplaceAllString(m.sql, "")
		var created []string
		for _, c := range createTable.FindAllStringSubmatch(sql, -1) {
			created = append(created, table(c[1]))
		}
		for _, r := range renameTable.FindAllStringSubmatch(sql, -1) {
			created = append(created, table(r[2]))
		}
		for _, t := range created {
			if prev, ok := owner[t]; ok && prev != match[1] {
				report("tables: %s is created by both %s and %s", t, prev, match[1])
				continue
			}
			owner[t] = match[1]
		}
	}
	for _, m := range migrations {
		module, ok := moduleOf[m.name]
		if !ok {
			continue
		}
		byVerb := touched(sqlComment.ReplaceAllString(m.sql, ""))
		for _, st := range tableStatements() {
			for _, t := range byVerb[st.verb] {
				switch tableOwner, known := owner[t]; {
				case !known:
					report("migration %s %s %s, which no migration creates", m.name, st.verb, t)
				case tableOwner != module:
					report("migration %s %s %s, which module %s creates: the migration belongs to %s", m.name, st.verb, t, tableOwner, tableOwner)
				}
			}
		}
	}

	covered := map[string]bool{}
	for _, e := range entries {
		match := moduleQueries.FindStringSubmatch(e.queries)
		if match == nil {
			report("sqlc entry %s: queries must be internal/modules/<module>/adapter/postgres/queries", e.queries)
			continue
		}
		module := match[1]
		covered[module] = true
		if want := "internal/modules/" + module + "/adapter/postgres/gen"; e.out != want {
			report("sqlc entry %s: out is %s, want %s", module, e.out, want)
		}
		for _, s := range e.schema {
			if other, ok := moduleOf[filepath.Base(s)]; ok && other != module {
				report("sqlc entry %s lists %s, a migration of %s", module, s, other)
			} else if !ok {
				report("sqlc entry %s lists %s, which is not a migration", module, s)
			}
		}
		for _, own := range byModule[module] {
			if !slices.Contains(e.schema, own) {
				report("sqlc entry %s does not list its migration %s", module, own)
			}
		}
	}
	for _, module := range slices.Sorted(slices.Values(queryModules)) {
		if !covered[module] {
			report("module %s has adapter/postgres/queries but no sqlc entry", module)
		}
	}
	return found
}
