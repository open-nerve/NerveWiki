package bootstrap

import (
	"archive/zip"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// The exports through serve (M7/P5 design 6; v0.1 design 13.1, item 21):
// started through the API, run by River in their queue, their archives
// downloaded and read back. Each module's part reaches the export through
// the composition root: a part left out fails these tests.

// transferJob is a TransferJobDetail answer, as much as the tests read.
type transferJob struct {
	ID        string  `json:"id"`
	State     string  `json:"state"`
	Name      string  `json:"name"`
	RootID    *string `json:"root_id"`
	CreatedBy struct {
		DisplayName string `json:"display_name"`
	} `json:"created_by"`
	Progress struct {
		Done  int `json:"done"`
		Total int `json:"total"`
	} `json:"progress"`
	Report *struct {
		Failure *string        `json:"failure"`
		Counts  map[string]int `json:"counts"`
	} `json:"report"`
	Download *struct {
		URL string `json:"url"`
	} `json:"download"`
	Problems []struct {
		Path string `json:"path"`
		Code string `json:"code"`
	} `json:"problems"`
}

// startExport starts by's export of the notebook nb, or of its page root
// when root is not "", and answers its job, queued.
func (tm acmeTeam) startExport(t *testing.T, by, nb, root string) transferJob {
	t.Helper()
	body := `{}`
	if root != "" {
		body = `{"root_id":"` + root + `"}`
	}
	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/exports", tm.tokens[by], body)
	if status != http.StatusAccepted {
		t.Fatalf("export %s as %s = %d %s", nb, by, status, answer)
	}
	var j transferJob
	decodeAnswer(t, answer, &j)
	return j
}

// transferJobOf reads the job id as by: its answer's status, and the job.
func (tm acmeTeam) transferJobOf(t *testing.T, by, id string) (int, transferJob) {
	t.Helper()
	status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/transfer-jobs/"+id, tm.tokens[by], "")
	var j transferJob
	if status == http.StatusOK {
		decodeAnswer(t, answer, &j)
	}
	return status, j
}

// endedJob waits until the job id has ended, as by reads it.
func (tm acmeTeam) endedJob(t *testing.T, by, id string) transferJob {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		status, j := tm.transferJobOf(t, by, id)
		if status != http.StatusOK {
			t.Fatalf("read the job %s as %s = %d", id, by, status)
		}
		if j.State != "queued" && j.State != "running" {
			return j
		}
		if time.Now().After(deadline) {
			t.Fatalf("the job %s is still %s", id, j.State)
		}
	}
}

// zipEntry is an archive's entry as read back.
type zipEntry struct {
	data   string
	method uint16
}

// archiveAt downloads the archive at address, with no token, and answers
// its entries' names in order, each entry by name, and the answer's
// Content-Disposition.
func (tm acmeTeam) archiveAt(t *testing.T, address string) ([]string, map[string]zipEntry, string) {
	t.Helper()
	req := newRequest(t, http.MethodGet, tm.base+address, "", nil)
	res, body := sendRequest(t, req)
	tm.contract.CheckResponse(t, req, res)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("download %s = %d %s %q", address, res.StatusCode, res.Header.Get("Content-Type"), body)
	}
	r, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	entries := map[string]zipEntry{}
	for _, f := range r.File {
		names = append(names, f.Name)
		if f.NonUTF8 {
			t.Errorf("%s is not named in UTF-8", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		entries[f.Name] = zipEntry{data: string(data), method: f.Method}
	}
	return names, entries, res.Header.Get("Content-Disposition")
}

// storedArchives are the exports' archives in the store at dir: each
// file's path, by its job's id.
func storedArchives(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(filepath.Join(dir, "exports"), func(path string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil
		case err != nil:
			return err
		case d.IsDir() && d.Name() == ".tmp":
			return fs.SkipDir
		case d.Type().IsRegular():
			files[strings.TrimSuffix(d.Name(), ".zip")] = path
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// metaPaths are the nodes' paths a meta.json lists, and its root's name.
func metaPaths(t *testing.T, data string) ([]string, string) {
	t.Helper()
	var meta struct {
		Format int `json:"format"`
		Root   *struct {
			Name string `json:"name"`
		} `json:"root"`
		Nodes []struct {
			Path string `json:"path"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(data), &meta); err != nil || meta.Format != 1 {
		t.Fatalf("meta.json %s: %v", data, err)
	}
	var paths []string
	for _, n := range meta.Nodes {
		paths = append(paths, n.Path)
	}
	root := ""
	if meta.Root != nil {
		root = meta.Root.Name
	}
	return paths, root
}

// A notebook exports through serve as an Obsidian vault, in the exports'
// queue, one attempt: each page with content as its file, its children in
// its folder; an empty page with children as its folder alone, unless a
// link leads to it (the linking module's part, through the composition
// root); an empty page without children as an empty file; an attachment
// stored as it is, its file read through the asset module; meta.json
// last, which lists the nodes in the vault's order. A subtree then exports at the vault's root; succeeding, it
// expires the notebook's export, whose archive is deleted.
func TestAnExportRunsThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	spec := tm.createPageWith(t, "alice", nb, "", "Spec", "# Spec\n\n[[Linked]]")
	folder := tm.createPage(t, "alice", nb, spec, "Folder")
	tm.createPageWith(t, "alice", nb, folder, "Deep", "deep")
	linked := tm.createPage(t, "alice", nb, spec, "Linked")
	tm.createPageWith(t, "alice", nb, linked, "Child", "child")
	tm.upload(t, "alice", nb, spec, "x.png", pngHead+"pixels")
	tm.createPage(t, "alice", nb, "", "Café")
	tm.resolves(t, spec, linked)

	whole := tm.endedJob(t, "alice", tm.startExport(t, "alice", nb, "").ID)
	if whole.State != "succeeded" || whole.Name != "Eng" || whole.Report == nil || whole.Report.Failure != nil ||
		whole.Report.Counts["pages"] != 6 || whole.Report.Counts["attachments"] != 1 || whole.Progress.Done != 7 || whole.Progress.Total != 7 ||
		whole.Download == nil || len(whole.Problems) != 0 {
		t.Fatalf("the notebook's export = %+v, want succeeded, 6 pages and an attachment", whole)
	}
	// Its starter by name, through identity's directory.
	if want := queryStrings(t, tm.pool, "SELECT display_name FROM users WHERE email = 'alice@example.com'"); whole.CreatedBy.DisplayName != want[0] {
		t.Errorf("the export's starter = %q, want %q", whole.CreatedBy.DisplayName, want[0])
	}
	// River completes its job once the use case has returned, a moment
	// after the row ended.
	awaitJob(t, tm.pool, "transfer.export")
	if got := queryStrings(t, tm.pool, "SELECT queue || ' ' || max_attempts FROM river_job WHERE kind = $1",
		"transfer.export"); !slices.Equal(got, []string{"transfer_export 1"}) {
		t.Errorf("the export's River job = %q, want in transfer_export, one attempt", got)
	}
	names, entries, disposition := tm.archiveAt(t, whole.Download.URL)
	// The pages in the snapshot, then the attachments outside it, then
	// meta.json, each in the vault's order.
	want := []string{"Eng/Spec.md", "Eng/Spec/Folder/", "Eng/Spec/Folder/Deep.md", "Eng/Spec/Linked.md", "Eng/Spec/Linked/Child.md",
		"Eng/Café.md", "Eng/Spec/x.png", "Eng/.nerve/meta.json"}
	if !slices.Equal(names, want) {
		t.Fatalf("the notebook's archive = %q, want %q", names, want)
	}
	for name, e := range map[string]zipEntry{
		"Eng/Spec.md":              {"# Spec\n\n[[Linked]]", zip.Deflate},
		"Eng/Spec/Folder/Deep.md":  {"deep", zip.Deflate},
		"Eng/Spec/Linked.md":       {"", zip.Deflate},
		"Eng/Spec/Linked/Child.md": {"child", zip.Deflate},
		"Eng/Spec/x.png":           {pngHead + "pixels", zip.Store},
		"Eng/Café.md":              {"", zip.Deflate},
	} {
		if got := entries[name]; got != e {
			t.Errorf("%s = %q (method %d), want %q (method %d)", name, got.data, got.method, e.data, e.method)
		}
	}
	if paths, root := metaPaths(t, entries["Eng/.nerve/meta.json"].data); root != "" || !slices.Equal(paths, []string{"Spec.md",
		"Spec/Folder/", "Spec/Folder/Deep.md", "Spec/Linked.md", "Spec/Linked/Child.md", "Spec/x.png", "Café.md"}) {
		t.Errorf("meta.json lists %q, root %q; want every node, no root", paths, root)
	}
	if disposition != `attachment; filename="Eng.zip"; filename*=UTF-8''Eng.zip` {
		t.Errorf("Content-Disposition = %q, want Eng.zip", disposition)
	}

	sub := tm.endedJob(t, "alice", tm.startExport(t, "alice", nb, spec).ID)
	if sub.State != "succeeded" || sub.Name != "Spec" || sub.RootID == nil || *sub.RootID != spec {
		t.Fatalf("the subtree's export = %+v, want succeeded, named Spec", sub)
	}
	names, entries, _ = tm.archiveAt(t, sub.Download.URL)
	if want := []string{"Spec/Spec.md", "Spec/Spec/Folder/", "Spec/Spec/Folder/Deep.md", "Spec/Spec/Linked.md", "Spec/Spec/Linked/Child.md",
		"Spec/Spec/x.png", "Spec/.nerve/meta.json"}; !slices.Equal(names, want) {
		t.Errorf("the subtree's archive = %q, want %q", names, want)
	}
	if paths, root := metaPaths(t, entries["Spec/.nerve/meta.json"].data); root != "Spec" || len(paths) != 6 || paths[0] != "Spec.md" {
		t.Errorf("the subtree's meta.json lists %q, root %q; want its six nodes, under Spec", paths, root)
	}
	if _, j := tm.transferJobOf(t, "alice", whole.ID); j.State != "expired" || j.Download != nil {
		t.Errorf("the notebook's export = %s, download %+v; want expired, none", j.State, j.Download)
	}
	if status, _ := tm.download(t, whole.Download.URL); status != http.StatusNotFound {
		t.Errorf("the expired archive's address = %d, want 404", status)
	}
	// The expired archive is deleted once the success commits, before
	// River completes the job.
	awaitExports(t, tm)
	if got := storedArchives(t, tm.storage); len(got) != 1 || got[sub.ID] == "" {
		t.Errorf("archives stored %v, want the subtree's alone", got)
	}
}

// A notebook's deletion deletes its jobs at its time (the transfer
// module's subscriber, through the composition root), on each of the
// deletion's three paths through serve: they read and download as not
// found.
func TestANotebooksDeletionDeletesItsJobs(t *testing.T) {
	for _, tt := range []struct {
		name     string
		setup    func(t *testing.T) (tm acmeTeam, nb, starter string)
		before   func(t *testing.T, tm acmeTeam)
		deletion func(nb string) step
	}{
		{
			name: "by its admin",
			setup: func(t *testing.T) (acmeTeam, string, string) {
				tm := newAcmeTeam(t, "", "")
				return tm, tm.openNotebook(t, "alice", "Eng"), "alice"
			},
			deletion: func(nb string) step { return request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, "") },
		},
		{
			name: "with its workspace",
			setup: func(t *testing.T) (acmeTeam, string, string) {
				tm := newAcmeTeam(t, "member", "")
				return tm, tm.createNotebook(t, "bob", "Eng"), "bob"
			},
			deletion: func(string) step { return request("alice", http.MethodDelete, "/api/v0/workspaces/acme", "") },
		},
		{
			name: "ownerless, by the workspace's admin",
			setup: func(t *testing.T) (acmeTeam, string, string) {
				tm := newAcmeTeam(t, "admin", "member")
				return tm, tm.createNotebook(t, "carol", "Plans"), "carol"
			},
			before:   func(t *testing.T, tm acmeTeam) { tm.send(t, tm.removal("alice", "carol"), http.StatusNoContent) },
			deletion: func(nb string) step { return ownerlessDeletion("alice", nb) },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm, nb, starter := tt.setup(t)
			tm.createPageWith(t, starter, nb, "", "Spec", "spec")
			j := tm.endedJob(t, starter, tm.startExport(t, starter, nb, "").ID)
			if j.Download == nil {
				t.Fatalf("export = %+v, want succeeded", j)
			}
			if tt.before != nil {
				tt.before(t, tm)
			}
			tm.send(t, tt.deletion(nb), http.StatusNoContent)
			if n := count(t, tm.pool, `SELECT count(*) FROM transfer_jobs j JOIN notebooks n ON n.id = j.notebook_id
				WHERE j.id = $1 AND j.deleted_at = n.deleted_at`, j.ID); n != 1 {
				t.Errorf("the job deleted at the notebook's time: %d rows, want 1", n)
			}
			if status, _ := tm.transferJobOf(t, starter, j.ID); status != http.StatusNotFound {
				t.Errorf("read the job = %d, want 404", status)
			}
			if status, _ := tm.download(t, j.Download.URL); status != http.StatusNotFound {
				t.Errorf("download = %d, want 404", status)
			}
		})
	}
}

// The download's addresses are signed with the signing key's derivation
// for transfer.DownloadKeyInfo, through the composition root: another
// key, or none, would sign addresses that anyone could forge.
func TestTheDownloadsAreSignedWithTheDerivedKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signing.pem")
	if err := os.WriteFile(path, []byte(openSSLKey), 0o600); err != nil {
		t.Fatal(err)
	}
	tm := newAcmeTeamWith(t, "", "", func(c *config.Config) { c.Auth.JWT.PrivateKeyFile = path })
	nb := tm.openNotebook(t, "alice", "Eng")
	tm.createPageWith(t, "alice", nb, "", "Spec", "spec")
	j := tm.endedJob(t, "alice", tm.startExport(t, "alice", nb, "").ID)
	if j.Download == nil {
		t.Fatalf("export = %+v, want succeeded", j)
	}
	u, err := url.Parse(j.Download.URL)
	if err != nil {
		t.Fatal(err)
	}
	e, err := strconv.ParseInt(u.Query().Get("e"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := identity.LoadSigningKeys([]byte(openSSLKey), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.MustParse(j.ID)
	mac := hmac.New(sha256.New, keys.Derive(transfer.DownloadKeyInfo))
	mac.Write([]byte("export-download"))
	mac.Write(id[:])
	mac.Write(binary.BigEndian.AppendUint64(nil, uint64(e))) //nolint:gosec // a time after 1970
	if want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16]); u.Query().Get("s") != want {
		t.Errorf("the address's signature = %q, want %q, the derived key's", u.Query().Get("s"), want)
	}
}

// storage.min_free_bytes reaches the exports: a store that keeps no more
// room refuses one, storage_full.
func TestTheStoresMinimumIsTheConfigured(t *testing.T) {
	tm := newAcmeTeamWith(t, "", "", func(c *config.Config) { c.Storage.MinFreeBytes = 1 << 60 })
	nb := tm.openNotebook(t, "alice", "Eng")
	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/exports", tm.tokens["alice"], `{}`)
	if status != http.StatusInsufficientStorage || problemCode(t, answer) != "storage_full" {
		t.Errorf("export = %d %s, want 507 storage_full", status, answer)
	}
}

// The module's settings reach it: an export of a notebook as many jobs as
// transfer.max_queued are queued is server_busy.
func TestTheQueuesLimitIsTheConfigured(t *testing.T) {
	tm := newAcmeTeamWith(t, "", "", func(c *config.Config) { c.Transfer.MaxQueued = 1 })
	nb := tm.openNotebook(t, "alice", "Eng")
	if _, err := tm.pool.Exec(t.Context(), `INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, created_at)
		SELECT gen_random_uuid(), n.id, 'export', 'queued', n.name, u.id, 'web', now() FROM notebooks n, users u
		WHERE n.id = $1 AND u.email = 'bob@example.com'`, nb); err != nil {
		t.Fatal(err)
	}
	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/exports", tm.tokens["alice"], `{}`)
	if status != http.StatusServiceUnavailable || problemCode(t, answer) != "server_busy" {
		t.Errorf("export = %d %s, want 503 server_busy", status, answer)
	}
}

// An attachment whose file is not in the store is reported, file_missing
// (the asset module's ErrNoFile, through the composition root), the
// export succeeding without it; meta.json leaves it out.
func TestAnExportReportsAnAttachmentWithoutItsFile(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	tm.upload(t, "alice", nb, "", "kept.txt", "kept")
	gone := tm.upload(t, "alice", nb, "", "gone.txt", "gone")
	if err := os.Remove(fileOf(t, tm.storage, uuid.MustParse(gone.Blob))); err != nil {
		t.Fatal(err)
	}
	j := tm.endedJob(t, "alice", tm.startExport(t, "alice", nb, "").ID)
	if j.State != "succeeded" || j.Report == nil || j.Report.Counts["attachments"] != 1 || j.Report.Counts["missing"] != 1 ||
		len(j.Problems) != 1 || j.Problems[0].Path != "gone.txt" || j.Problems[0].Code != "file_missing" {
		t.Fatalf("the export = %+v, want succeeded, gone.txt missing", j)
	}
	names, entries, _ := tm.archiveAt(t, j.Download.URL)
	if !slices.Equal(names, []string{"Eng/kept.txt", "Eng/.nerve/meta.json"}) || entries["Eng/kept.txt"].data != "kept" {
		t.Errorf("the archive = %q, want kept.txt and meta.json", names)
	}
	if paths, _ := metaPaths(t, entries["Eng/.nerve/meta.json"].data); !slices.Equal(paths, []string{"kept.txt"}) {
		t.Errorf("meta.json lists %q, want kept.txt", paths)
	}
}
