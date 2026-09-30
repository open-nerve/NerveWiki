package archtest

import (
	"slices"
	"strings"
	"testing"
)

// A layout of the rule's cases: identity creates users; a later
// module, asset, creates assets, which references users and has an index;
// identity's own later migration alters users to reference assets; River's
// migration (a platform one, owned by no module) belongs to no entry, and
// alters an unlogged table it creates,
// puts a trigger on its table and drops it, and renames a table and drops
// it, as its real one does.
func sqlcBase() ([]sqlcEntry, []migrationFile, []string) {
	entries := []sqlcEntry{
		{
			schema:  []string{"migrations/sql/00001_identity_users.sql", "migrations/sql/00021_identity_users_avatar_asset.sql"},
			queries: "internal/modules/identity/adapter/postgres/queries",
			out:     "internal/modules/identity/adapter/postgres/gen",
		},
		{
			schema:  []string{"migrations/sql/00020_asset_assets.sql"},
			queries: "internal/modules/asset/adapter/postgres/queries",
			out:     "internal/modules/asset/adapter/postgres/gen",
		},
	}
	migrations := []migrationFile{
		{"00001_identity_users.sql", "-- +goose Up\nCREATE TABLE users (id uuid PRIMARY KEY);\n-- +goose Down\nDROP TABLE users;\n"},
		{"00005_river_main_v2_to_v7.sql", "-- +goose Up\nCREATE TABLE river_job (id bigint);\nCREATE UNLOGGED TABLE river_leader (name text);\n" +
			"ALTER TABLE river_job ADD COLUMN x int;\nALTER TABLE river_leader ADD COLUMN y int;\n" +
			"CREATE TRIGGER river_notify\n    AFTER INSERT ON river_job\n    FOR EACH ROW EXECUTE PROCEDURE river_job_notify();\n" +
			"DROP TRIGGER river_notify ON river_job;\n" +
			"CREATE TABLE river_migration (version bigint);\nALTER TABLE river_migration\n    RENAME TO river_migration_old;\n" +
			"CREATE UNIQUE INDEX ON river_job USING btree(id);\nDROP TABLE river_migration_old;\n"},
		{"00020_asset_assets.sql", "-- +goose Up\nCREATE TABLE IF NOT EXISTS assets (id uuid PRIMARY KEY, created_by_id uuid REFERENCES users);\n" +
			"CREATE INDEX assets_created_by_id_idx ON assets (created_by_id);\n-- +goose Down\nDROP TABLE assets;\n"},
		{"00021_identity_users_avatar_asset.sql", "-- +goose Up\n-- ALTER TABLE assets would be wrong; a comment is not SQL\n" +
			"ALTER TABLE users ADD COLUMN avatar_asset_id uuid REFERENCES assets ON DELETE SET NULL;\n" +
			"-- +goose Down\nALTER TABLE ONLY public.users DROP COLUMN avatar_asset_id;\n"},
	}
	return entries, migrations, []string{"identity", "asset"}
}

func TestSQLCScopeOfTheBaseLayoutPasses(t *testing.T) {
	if got := sqlcScopeViolations(sqlcBase()); len(got) != 0 {
		t.Errorf("violations = %q, want none", got)
	}
}

func TestSQLCScopeReportsViolations(t *testing.T) {
	tests := []struct {
		name   string
		change func(entries []sqlcEntry, migrations []migrationFile, modules []string) ([]sqlcEntry, []migrationFile, []string)
		want   string
	}{
		{"ALTER TABLE in a file of another module", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[3].name = "00021_asset_users_avatar.sql"
			e[0].schema = e[0].schema[:1]
			e[1].schema = append(e[1].schema, "migrations/sql/00021_asset_users_avatar.sql")
			return e, m, mods
		}, "migration 00021_asset_users_avatar.sql alters users, which module identity creates: the migration belongs to identity"},
		{"ALTER TABLE of an unlogged table in a file of another module", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += "CREATE UNLOGGED TABLE asset_uploads (id uuid);\n"
			m = append(m, migrationFile{"00022_identity_asset_uploads.sql", "-- +goose Up\nALTER TABLE asset_uploads SET LOGGED;\n"})
			e[0].schema = append(e[0].schema, "migrations/sql/00022_identity_asset_uploads.sql")
			return e, m, mods
		}, "migration 00022_identity_asset_uploads.sql alters asset_uploads, which module asset creates: the migration belongs to asset"},
		{"ALTER TABLE of an unknown table", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[0].sql = "CREATE TABLE people (id uuid);"
			return e, m, mods
		}, "migration 00021_identity_users_avatar_asset.sql alters users, which no migration creates"},
		{"a migration of another module", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			e[1].schema = append(e[1].schema, "migrations/sql/00001_identity_users.sql")
			return e, m, mods
		}, "sqlc entry asset lists migrations/sql/00001_identity_users.sql, a migration of identity"},
		{"River's migration", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			e[0].schema = append(e[0].schema, "migrations/sql/00005_river_main_v2_to_v7.sql")
			return e, m, mods
		}, "sqlc entry identity lists migrations/sql/00005_river_main_v2_to_v7.sql, a migration of river"},
		{"an own migration missing", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			e[0].schema = e[0].schema[:1]
			return e, m, mods
		}, "sqlc entry identity does not list its migration migrations/sql/00021_identity_users_avatar_asset.sql"},
		{"not a migration", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			e[1].schema = append(e[1].schema, "migrations/sql/schema.sql")
			return e, m, mods
		}, "sqlc entry asset lists migrations/sql/schema.sql, which is not a migration"},
		{"a module with queries but no entry", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			return e, m, append(mods, "workspace")
		}, "module workspace has adapter/postgres/queries but no sqlc entry"},
		{"queries outside a module's adapter", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			e[1].queries = "queries/asset"
			return e, m, mods[:1]
		}, "sqlc entry queries/asset: queries must be internal/modules/<module>/adapter/postgres/queries"},
		{"out outside the module's adapter", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			e[1].out = "internal/db"
			return e, m, mods
		}, "sqlc entry asset: out is internal/db, want internal/modules/asset/adapter/postgres/gen"},
		{"a misnamed migration", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m = append(m, migrationFile{"00030-assets.sql", ""})
			return e, m, mods
		}, "migration 00030-assets.sql is not named <version>_<module>_<content>.sql"},
		{"a table created twice", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += "CREATE TABLE users (id uuid);"
			return e, m, mods
		}, "tables: users is created by both identity and asset"},
		// The four forms of the rule, each on another module's table.
		{"ALTER TABLE of a quoted name", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += `ALTER TABLE "users" ADD COLUMN asset_id uuid;`
			return e, m, mods
		}, "migration 00020_asset_assets.sql alters users, which module identity creates: the migration belongs to identity"},
		{"ALTER TABLE of a quoted name in the schema public", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += `ALTER TABLE IF EXISTS ONLY "public"."users" DROP COLUMN x;`
			return e, m, mods
		}, "migration 00020_asset_assets.sql alters users, which module identity creates: the migration belongs to identity"},
		{"CREATE INDEX on another module's table", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += "CREATE INDEX CONCURRENTLY IF NOT EXISTS assets_users_email_idx ON users (email);"
			return e, m, mods
		}, "migration 00020_asset_assets.sql indexes users, which module identity creates: the migration belongs to identity"},
		{"CREATE UNIQUE INDEX without a name", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += "CREATE UNIQUE INDEX ON ONLY public.users (lower(email));"
			return e, m, mods
		}, "migration 00020_asset_assets.sql indexes users, which module identity creates: the migration belongs to identity"},
		{"CREATE TRIGGER on another module's table", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += "CREATE OR REPLACE TRIGGER assets_touch\n    AFTER UPDATE OF email\n    ON users\n    FOR EACH ROW EXECUTE FUNCTION touch();"
			return e, m, mods
		}, "migration 00020_asset_assets.sql puts a trigger on users, which module identity creates: the migration belongs to identity"},
		{"DROP TABLE of another module's table", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql = strings.Replace(m[2].sql, "DROP TABLE assets;", "DROP TABLE IF EXISTS assets, users CASCADE;", 1)
			return e, m, mods
		}, "migration 00020_asset_assets.sql drops users, which module identity creates: the migration belongs to identity"},
		{"a quoted name that differs from users only in case", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += `DROP TABLE "Users";`
			return e, m, mods
		}, "migration 00020_asset_assets.sql drops Users, which no migration creates"},
		{"an unquoted name in capitals", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += "ALTER TABLE USERS ADD COLUMN x int;"
			return e, m, mods
		}, "migration 00020_asset_assets.sql alters users, which module identity creates: the migration belongs to identity"},
		{"CREATE CONSTRAINT TRIGGER on another module's table", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += "CREATE CONSTRAINT TRIGGER assets_check AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION check_assets();"
			return e, m, mods
		}, "migration 00020_asset_assets.sql puts a trigger on users, which module identity creates: the migration belongs to identity"},
		{"a quoted table created twice", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += `CREATE TABLE "users" (id uuid);`
			return e, m, mods
		}, "tables: users is created by both identity and asset"},
		{"a rename of another module's table", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += "ALTER TABLE users RENAME TO people;"
			return e, m, mods
		}, "migration 00020_asset_assets.sql alters users, which module identity creates: the migration belongs to identity"},
		{"DROP TRIGGER on another module's table", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += "DROP TRIGGER IF EXISTS assets_touch ON public.users CASCADE;"
			return e, m, mods
		}, "migration 00020_asset_assets.sql drops a trigger on users, which module identity creates: the migration belongs to identity"},
		{"DROP TRIGGER of a quoted name, without IF EXISTS", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += `DROP TRIGGER "assets_touch" ON "users";`
			return e, m, mods
		}, "migration 00020_asset_assets.sql drops a trigger on users, which module identity creates: the migration belongs to identity"},
		{"ALTER TRIGGER on another module's table", func(e []sqlcEntry, m []migrationFile, mods []string) ([]sqlcEntry, []migrationFile, []string) {
			m[2].sql += "ALTER TRIGGER assets_touch ON users RENAME TO assets_touch_email;"
			return e, m, mods
		}, "migration 00020_asset_assets.sql alters a trigger on users, which module identity creates: the migration belongs to identity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sqlcScopeViolations(tt.change(sqlcBase()))
			if !slices.Equal(got, []string{tt.want}) {
				t.Errorf("violations = %q, want %q", got, tt.want)
			}
		})
	}
}
