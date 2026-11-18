package drive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/open-nerve/NerveWiki/server/tools/perf/client"
)

// Transfer is B8's (design 3.6, 3.4.5): the vault's zip imported under
// the page Root at the notebook's root (made when it is not there),
// asked for every Poll until it succeeds, then the background awaited
// quiet; or the page Root exported, likewise, and its zip downloaded to
// Download. Around each, psql's WAL position and database size and the
// storage directory's sizes; the job's time as the client saw it, to a
// poll, and as the server kept it (review B's N3).
type Transfer struct {
	Client   *client.Client
	PSQL     client.PSQL
	Notebook string
	Root     string
	Poll     time.Duration
	Storage  string // the serve's storage.dir
	Out      io.Writer
}

// Footprint is the database's WAL position and size and the storage
// directory's bytes, its imports/ and exports/ among them.
type Footprint struct {
	LSN                       string
	Database                  int64
	Storage, Imports, Exports int64
}

// dirBytes is the bytes of the files under dir, 0 when it is not there.
func dirBytes(dir string) (int64, error) {
	var n int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			n += info.Size()
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	return n, err
}

func (t Transfer) footprint(ctx context.Context) (Footprint, error) {
	rows, err := t.PSQL.Query(ctx, "SELECT pg_current_wal_lsn(), pg_database_size(current_database())")
	if err != nil {
		return Footprint{}, err
	}
	if len(rows) != 1 || len(rows[0]) != 2 {
		return Footprint{}, fmt.Errorf("the WAL read as %v", rows)
	}
	f := Footprint{LSN: rows[0][0]}
	if _, err := fmt.Sscan(rows[0][1], &f.Database); err != nil {
		return Footprint{}, err
	}
	for _, d := range []struct {
		dir string
		n   *int64
	}{{t.Storage, &f.Storage}, {filepath.Join(t.Storage, "imports"), &f.Imports}, {filepath.Join(t.Storage, "exports"), &f.Exports}} {
		if *d.n, err = dirBytes(d.dir); err != nil {
			return Footprint{}, err
		}
	}
	return f, nil
}

// report writes what a transfer took and wrote.
func (t Transfer) report(ctx context.Context, what string, before Footprint, times map[string]time.Duration) error {
	after, err := t.footprint(ctx)
	if err != nil {
		return err
	}
	wal, err := t.PSQL.Int(ctx, "SELECT pg_wal_lsn_diff("+client.Literal(after.LSN)+", "+client.Literal(before.LSN)+")::bigint")
	if err != nil {
		return err
	}
	line := map[string]any{"transfer": what, "wal_bytes": wal, "before": before, "after": after}
	for k, d := range times {
		line[k+"_ms"] = float64(d) / float64(time.Millisecond)
	}
	b, err := json.Marshal(line)
	if err != nil {
		return err
	}
	_, err = t.Out.Write(append(b, '\n'))
	return err
}

// root is the page Root's id at the notebook's root, made when create says
// and it is not there.
func (t Transfer) root(ctx context.Context, create bool) (string, error) {
	var tree struct {
		Data []struct {
			ID       string  `json:"id"`
			ParentID *string `json:"parent_id"`
			Name     string  `json:"name"`
			Kind     string  `json:"kind"`
		} `json:"data"`
	}
	if _, err := t.Client.JSON(ctx, "tree", http.MethodGet, "/api/v0/notebooks/"+url.PathEscape(t.Notebook)+"/nodes", nil, &tree,
		http.StatusOK); err != nil {
		return "", err
	}
	for _, n := range tree.Data {
		if n.ParentID == nil && n.Name == t.Root && n.Kind == "page" {
			return n.ID, nil
		}
	}
	if !create {
		return "", fmt.Errorf("no page %q at the notebook's root", t.Root)
	}
	p, err := createPage(ctx, t.Client, "setup-page", t.Notebook, "", t.Root, "")
	return p.ID, err
}

// Import uploads zip under the root page and waits for the job, then the
// background's quiet.
func (t Transfer) Import(ctx context.Context, zip string) error {
	root, err := t.root(ctx, true)
	if err != nil {
		return err
	}
	before, err := t.footprint(ctx)
	if err != nil {
		return err
	}
	f, err := os.Open(zip)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		err := mw.WriteField("parent_id", root)
		if err == nil {
			var part io.Writer
			if part, err = mw.CreateFormFile("file", filepath.Base(zip)); err == nil {
				if _, err = io.Copy(part, f); err == nil {
					err = mw.Close()
				}
			}
		}
		pw.CloseWithError(err)
	}()
	rec, answer, err := t.Client.Send(ctx, client.Request{Op: "import", Method: http.MethodPost,
		Path: "/api/v0/notebooks/" + url.PathEscape(t.Notebook) + "/imports", Body: pr, ContentType: mw.FormDataContentType()}, nil)
	if err != nil {
		return err
	}
	var j job
	if rec.Status != http.StatusAccepted || json.Unmarshal(answer, &j) != nil {
		return fmt.Errorf("the import answered %d %.300s", rec.Status, answer)
	}
	accepted := time.Unix(0, rec.Header)
	if j, err = awaitJob(ctx, t.Client, "import-job", j.ID, t.Poll); err != nil {
		return err
	}
	done := time.Now()
	for {
		q, err := t.PSQL.Quiet(ctx)
		if err != nil {
			return err
		}
		if q {
			break
		}
		if err := sleep(ctx, t.Poll); err != nil {
			return err
		}
	}
	times := j.serverTimes()
	times["upload"], times["job"], times["quiet"] = rec.ToHeader(), done.Sub(accepted), time.Since(done)
	return t.report(ctx, "import", before, times)
}

// Export exports the root page's subtree, waits for the job and downloads
// its zip to download.
func (t Transfer) Export(ctx context.Context, download string) error {
	root, err := t.root(ctx, false)
	if err != nil {
		return err
	}
	before, err := t.footprint(ctx)
	if err != nil {
		return err
	}
	var j job
	rec, err := t.Client.JSON(ctx, "export", http.MethodPost, "/api/v0/notebooks/"+url.PathEscape(t.Notebook)+"/exports",
		map[string]string{"root_id": root}, &j, http.StatusAccepted)
	if err != nil {
		return err
	}
	accepted := time.Unix(0, rec.Header)
	j, err = awaitJob(ctx, t.Client, "export-job", j.ID, t.Poll)
	if err != nil {
		return err
	}
	done := time.Now()
	if j.Download == nil || !strings.HasPrefix(j.Download.URL, "/") {
		return fmt.Errorf("the export succeeded without its address on this server: %+v", j.Download)
	}
	out, err := os.Create(download)
	if err != nil {
		return err
	}
	// The signed address, as a browser downloads it: without a token.
	anonymous := *t.Client
	anonymous.Token = ""
	got, _, err := anonymous.Send(ctx, client.Request{Op: "download", Method: http.MethodGet, Path: j.Download.URL}, out)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if got.Status != http.StatusOK {
		return fmt.Errorf("the download answered %d", got.Status)
	}
	if _, err := fmt.Fprintf(t.Out, "download bytes=%d\n", got.Bytes); err != nil {
		return err
	}
	times := j.serverTimes()
	times["job"], times["download"] = done.Sub(accepted), got.Took()
	return t.report(ctx, "export", before, times)
}
