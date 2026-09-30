// Package migrations embeds the SQL schema migrations. Files live in sql/ and
// are named NNNNN_<owner>_<description>.sql. The owner is the module whose
// table the migration changes, platform for what belongs to no module, such
// as an extension, or river for River's own tables, which platform/jobs uses
// (v0.1 design 7.1).
package migrations

import (
	"embed"
	"io/fs"
)

//go:embed sql/*.sql
var files embed.FS

// FS returns the migrations, with the files at its root.
func FS() fs.FS {
	sub, err := fs.Sub(files, "sql")
	if err != nil {
		panic(err) // unreachable: "sql" is a valid path embedded above
	}
	return sub
}
