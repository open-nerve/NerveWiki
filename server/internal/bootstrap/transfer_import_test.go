package bootstrap

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// The imports through serve (M7/P6 design 3.16): started through the API,
// run by River in their queue, their nodes written through the page
// module's units, their files through the asset module's, their pages
// indexed by the linking module's observers, the statistics refreshed.
// Each module's part reaches the import through the composition root: a
// part left out fails these tests.

// vaultFile is a file of a vault a test zips: a folder when its name ends
// in "/".
type vaultFile struct {
	name, data string
}

// vaultZip zips files, deflated, in their order.
func vaultZip(t *testing.T, files ...vaultFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		method := zip.Deflate
		if strings.HasSuffix(f.name, "/") {
			method = zip.Store
		}
		fw, err := w.CreateHeader(&zip.FileHeader{Name: f.name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(f.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// importBody is an import's form: under parent when it is not "", the
// archive named name.
func importBody(t *testing.T, parent, name string, archive []byte) (contentType, body string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if parent != "" {
		if err := w.WriteField("parent_id", parent); err != nil {
			t.Fatal(err)
		}
	}
	fw, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(archive); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w.FormDataContentType(), buf.String()
}

// startImport starts by's import of archive into the notebook nb, under
// its page parent when it is not "", and answers its status and body.
func (tm acmeTeam) startImport(t *testing.T, by, nb, parent string, archive []byte) (int, string) {
	t.Helper()
	contentType, body := importBody(t, parent, "vault.zip", archive)
	return askTyped(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/imports", tm.tokens[by], contentType, body)
}

// imported runs by's import of archive into nb, under parent, to its end.
func (tm acmeTeam) imported(t *testing.T, by, nb, parent string, archive []byte) transferJob {
	t.Helper()
	status, answer := tm.startImport(t, by, nb, parent, archive)
	if status != http.StatusAccepted {
		t.Fatalf("import into %s as %s = %d %s", nb, by, status, answer)
	}
	var j transferJob
	decodeAnswer(t, answer, &j)
	return tm.endedJob(t, by, j.ID)
}

// storedImports are the imports' archives in the store at dir, the files
// still being written left out.
func storedImports(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(filepath.Join(dir, "imports"), func(path string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil
		case err != nil:
			return err
		case d.IsDir() && d.Name() == ".tmp":
			return fs.SkipDir
		case d.Type().IsRegular():
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// analyzed are those of tables whose statistics an ANALYZE refreshed,
// once the server's statistics show them, within seconds.
func analyzed(t *testing.T, pool *pgxpool.Pool, tables []string) []string {
	t.Helper()
	var got []string
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		got = queryStrings(t, pool, "SELECT relname FROM pg_stat_user_tables WHERE last_analyze IS NOT NULL AND relname = ANY($1) ORDER BY relname",
			tables)
		if len(got) == len(tables) || time.Now().After(deadline) {
			return got
		}
	}
}

// A vault imports through serve, in the imports' queue, one attempt: each
// page with its content, a folder without its page as a page without
// content, an attachment with its file and its row, a name mended and
// reported; the links resolve both ways, those of the pages there before
// to the pages imported (the linking module's observers, through the
// composition root). One changeset of the import's kind holds it; the
// archive is deleted, the statistics refreshed.
func TestAnImportRunsThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	spec := tm.createPageWith(t, "alice", nb, "", "Spec", "[[Notes]]")
	tm.resolves(t, spec, "")
	archive := vaultZip(t, vaultFile{"V/", ""}, vaultFile{"V/.obsidian/app.json", "{}"}, vaultFile{"V/Notes.md", "see [[Spec]] ![[pic.png]]"},
		vaultFile{"V/Notes/pic.png", pngHead + "pixels"}, vaultFile{"V/Notes/Sub.md", "sub"}, vaultFile{"V/D/e.md", "e"},
		vaultFile{"V/n:m.md", "x"})

	j := tm.imported(t, "alice", nb, "", archive)

	if j.State != "succeeded" || j.Name != "vault.zip" || j.RootID != nil || j.Report == nil || j.Report.Failure != nil ||
		j.Report.Counts["pages"] != 5 || j.Report.Counts["attachments"] != 1 || j.Report.Counts["renamed"] != 1 || j.Progress.Done != 6 ||
		j.Progress.Total != 6 || len(j.Problems) != 1 || j.Problems[0].Code != "renamed" || j.Problems[0].Path != "n:m.md" {
		t.Fatalf("the import = %+v, report %+v, want succeeded, 5 pages, an attachment, n:m.md renamed", j, describe(j.Report))
	}
	awaitJob(t, tm.pool, "transfer.import")
	if got := queryStrings(t, tm.pool, "SELECT queue || ' ' || max_attempts FROM river_job WHERE kind = $1",
		"transfer.import"); !slices.Equal(got, []string{"transfer_import 1"}) {
		t.Errorf("the import's River job = %q, want in transfer_import, one attempt", got)
	}
	tree := queryStrings(t, tm.pool, `SELECT coalesce(p.name, '-') || '/' || n.name || ' ' || n.kind || ' ' || coalesce(c.content, '')
		FROM nodes n LEFT JOIN nodes p ON p.id = n.parent_id LEFT JOIN page_contents c ON c.node_id = n.id
		WHERE n.notebook_id = $1 AND n.deleted_at IS NULL ORDER BY n.created_at, n.id`, nb)
	// By name: D, n_m, then Notes; each level's by its parents' order.
	want := []string{"-/Spec page [[Notes]]", "-/D page ", "-/n_m page x", "-/Notes page see [[Spec]] ![[pic.png]]", "D/e page e",
		"Notes/pic.png asset ", "Notes/Sub page sub"}
	if !slices.Equal(tree, want) {
		t.Errorf("the tree = %q, want %q", tree, want)
	}
	notes := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE name = 'Notes'")[0]
	pic := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE name = 'pic.png'")[0]
	tm.resolves(t, spec, notes)
	tm.resolves(t, notes, spec, pic)
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
	if got := queryStrings(t, tm.pool, "SELECT mime || ' ' || byte_size FROM asset_blobs WHERE node_id = $1", pic); !slices.Equal(got,
		[]string{"image/png " + strconv.Itoa(len(pngHead+"pixels"))}) {
		t.Errorf("pic.png's row = %q, want a PNG of its bytes", got)
	}
	if got := queryStrings(t, tm.pool, "SELECT kind || ' ' || client FROM changesets WHERE notebook_id = $1 ORDER BY created_at", nb); !slices.Equal(got,
		[]string{"edit web", "import web"}) {
		t.Errorf("the changesets = %q, want the page's edit, then the import", got)
	}
	if files := storedImports(t, tm.storage); len(files) != 0 {
		t.Errorf("imports stored %q, want the archive deleted", files)
	}
	if got := analyzed(t, tm.pool, importAnalyzes()); len(got) != len(importAnalyzes()) {
		t.Errorf("analyzed %q, want %q", got, importAnalyzes())
	}
}

// The imports' bounds are the configured: an archive past
// transfer.import_max_bytes is 413; one of more entries than
// transfer.import_max_entries, or unpacking to more than
// transfer.import_max_unpacked_bytes, fails, nothing written; an
// attachment past asset.max_bytes is skipped.
func TestTheImportsLimitsAreTheConfigured(t *testing.T) {
	tm := newAcmeTeamWith(t, "", "", func(c *config.Config) {
		c.Asset.MaxBytes, c.Transfer.ImportMaxBytes, c.Transfer.ImportMaxEntries, c.Transfer.ImportMaxUnpackedBytes = 1<<10, 1<<20, 3, 1<<20
	})
	nb := tm.openNotebook(t, "alice", "Eng")
	if status, answer := tm.startImport(t, "alice", nb, "", bytes.Repeat([]byte("z"), 1<<20+1)); status != http.StatusRequestEntityTooLarge {
		t.Errorf("an import of more than 1 MiB = %d %s, want 413", status, answer)
	}
	for _, tt := range []struct {
		name    string
		archive []byte
		want    string
	}{
		{"four entries", vaultZip(t, vaultFile{"a.md", "a"}, vaultFile{"b.md", "b"}, vaultFile{"c.md", "c"}, vaultFile{"d.md", "d"}), "too_many_entries"},
		{"unpacking to more than 1 MiB", vaultZip(t, vaultFile{"a.md", strings.Repeat("0", 1<<20+1)}), "unpacked_too_large"},
	} {
		if j := tm.imported(t, "alice", nb, "", tt.archive); j.State != "failed" || j.Report == nil || j.Report.Failure == nil ||
			*j.Report.Failure != tt.want {
			t.Errorf("an import of %s = %+v, want failed %s", tt.name, j, tt.want)
		}
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1", nb); n != 0 {
		t.Errorf("%d nodes, want none written", n)
	}
	j := tm.imported(t, "alice", nb, "", vaultZip(t, vaultFile{"big.bin", strings.Repeat("b", 1<<10+1)}, vaultFile{"ok.bin", "ok"}))
	if j.State != "succeeded" || j.Report.Counts["attachments"] != 1 || len(j.Problems) != 1 || j.Problems[0].Code != "too_large" {
		t.Errorf("an import of an attachment past 1 KiB = %+v, want it skipped too_large", j)
	}
	if files := storedImports(t, tm.storage); len(files) != 0 {
		t.Errorf("imports stored %q, want each archive deleted", files)
	}
}

// One import at once in a notebook, anyone's: a second is transfer.busy
// while the first is queued.
func TestOneImportAtOnceThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	ctx := context.Background()
	if _, err := tm.pool.Exec(ctx, `INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, created_at)
		SELECT gen_random_uuid(), $1, 'import', 'queued', 'vault.zip', id, 'web', now() FROM users WHERE email = 'alice@example.com'`, nb); err != nil {
		t.Fatal(err)
	}
	if status, answer := tm.startImport(t, "bob", nb, "", vaultZip(t, vaultFile{"a.md", "a"})); status != http.StatusConflict ||
		!strings.Contains(answer, `"transfer.busy"`) {
		t.Errorf("a second import = %d %s, want 409 transfer.busy", status, answer)
	}
	if files := storedImports(t, tm.storage); len(files) != 0 {
		t.Errorf("imports stored %q, want none: refused before the file", files)
	}
}

// describe is a report as the tests tell it.
func describe(r *struct {
	Failure *string        `json:"failure"`
	Counts  map[string]int `json:"counts"`
},
) string {
	if r == nil {
		return "none"
	}
	failure := "none"
	if r.Failure != nil {
		failure = *r.Failure
	}
	return "failure " + failure + ", counts " + fmt.Sprint(r.Counts)
}
