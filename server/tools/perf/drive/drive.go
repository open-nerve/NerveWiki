// Package drive holds perf's five drivers (M12/P4 design 3.4.5): bodies
// (B2), mcp (B3), editors (M4-performance 1 and 6, batch G), tabs (the
// 100,000 pages' tree reread) and ops (B4), and transfer (B8). Each sends
// through client, records each request, and waits as the design says;
// their times are given, so that a test against a fake server takes
// milliseconds.
package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/open-nerve/NerveWiki/server/tools/perf/client"
	"github.com/open-nerve/NerveWiki/server/tools/perf/stats"
)

// MaxContentBytes is a page's content's bound (page/domain/content.go).
const MaxContentBytes = 5 << 20

// NormalBody is size bytes of ordinary prose from seed: headings, lists,
// wiki links and links among paragraphs (design 3.6's B2).
func NormalBody(seed uint64, size int) string { return prose(seed, size, true) }

// prose is size bytes of prose from seed, with links or without.
func prose(seed uint64, size int, links bool) string {
	r := rand.New(rand.NewPCG(seed, 1))
	var b strings.Builder
	b.Grow(size + 256)
	words := func(n int) string {
		w := make([]string, n)
		for i := range w {
			w[i] = proseWord(r)
		}
		return strings.Join(w, " ")
	}
	for b.Len() < size {
		switch k := r.IntN(10); {
		case k == 0:
			b.WriteString("## " + words(2+r.IntN(4)) + "\n\n")
		case k <= 2:
			for range 2 + r.IntN(4) {
				b.WriteString("- " + words(3+r.IntN(8)) + "\n")
			}
			b.WriteString("\n")
		default:
			for s := range 3 + r.IntN(5) {
				if s > 0 {
					b.WriteString(" ")
				}
				b.WriteString(words(6 + r.IntN(12)))
				switch k := r.IntN(6); {
				case !links:
				case k == 0:
					fmt.Fprintf(&b, " [[Page %d]]", r.IntN(1000))
				case k == 1:
					fmt.Fprintf(&b, " [%s](https://example.com/%d)", proseWord(r), r.IntN(1000))
				}
				b.WriteString(".")
			}
			b.WriteString("\n\n")
		}
	}
	return b.String()[:size]
}

// proseWord is a word of two to ten letters.
func proseWord(r *rand.Rand) string {
	w := make([]byte, 2+r.IntN(9))
	for i := range w {
		w[i] = byte('a' + r.IntN(26))
	}
	return string(w)
}

// PathologicalBody is markdowntest's "mismatched emphasis a*_" at size
// bytes (platform/markdown/markdowntest, M4/P4 review P1), written out:
// perf does not import across modules.
func PathologicalBody(size int) string { return strings.Repeat("a*_", max(1, size/3)) }

// LinkedBody is size bytes, or its links' when more, whose first lines are
// links [[Page n]], links of them, then prose without links (the editors'
// pages).
func LinkedBody(seed uint64, size, links int) string {
	var b strings.Builder
	for i := range links {
		fmt.Fprintf(&b, "[[Page %d]]\n", i)
	}
	b.WriteString("\n")
	if b.Len() < size {
		b.WriteString(prose(seed, size-b.Len(), false))
	}
	return b.String()
}

// changed is content with one byte changed, the k-th change: the byte in
// its middle, a letter that is not the one before.
func changed(content string, k int) string {
	b := []byte(content)
	mid := len(b) / 2
	b[mid] = byte('p' + k%2)
	return string(b)
}

// page is a node's answer, as far as a driver reads it.
type page struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

// createPage makes a page titled title under parent (the root for "")
// in notebook, as c, with content unless empty: its id and revision.
func createPage(ctx context.Context, c *client.Client, op, notebook, parent, title, content string) (page, error) {
	in := map[string]any{"parent_id": nil, "title": title}
	if parent != "" {
		in["parent_id"] = parent
	}
	var p page
	if _, err := c.JSON(ctx, op, http.MethodPost, "/api/v0/notebooks/"+url.PathEscape(notebook)+"/pages", in, &p,
		http.StatusCreated); err != nil {
		return page{}, err
	}
	if content == "" {
		return p, nil
	}
	return putContent(ctx, c, op, p.ID, content, p.Revision, "")
}

// putContent writes a page's content over base, in session when given.
func putContent(ctx context.Context, c *client.Client, op, id, content string, base int64, session string) (page, error) {
	in := map[string]any{"content": content, "base_revision": base}
	if session != "" {
		in["edit_session_id"] = session
	}
	var p page
	_, err := c.JSON(ctx, op, http.MethodPut, "/api/v0/pages/"+url.PathEscape(id)+"/content", in, &p, http.StatusOK)
	return p, err
}

// openSession opens an edit session on a page: its id.
func openSession(ctx context.Context, c *client.Client, op, id string) (string, error) {
	var s struct {
		ID string `json:"id"`
	}
	_, err := c.JSON(ctx, op, http.MethodPost, "/api/v0/pages/"+url.PathEscape(id)+"/edit-sessions", map[string]any{}, &s,
		http.StatusCreated)
	return s.ID, err
}

// job is a background job's answer: its id and state, and an export's
// signed address once it succeeded.
type job struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Download *struct {
		URL string `json:"url"`
	} `json:"download"`
}

// awaitJob asks for a job every poll until it succeeds, and answers it:
// an error when it fails, is cancelled or expires.
func awaitJob(ctx context.Context, c *client.Client, op, id string, poll time.Duration) (job, error) {
	for {
		var j job
		if _, err := c.JSON(ctx, op, http.MethodGet, "/api/v0/transfer-jobs/"+url.PathEscape(id), nil, &j, http.StatusOK); err != nil {
			return job{}, err
		}
		switch j.State {
		case "succeeded":
			return j, nil
		case "queued", "running":
		default:
			return job{}, fmt.Errorf("job %s %s", id, j.State)
		}
		if err := sleep(ctx, poll); err != nil {
			return job{}, err
		}
	}
}

// sleep waits d, or until ctx ends.
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Summarize writes, for each operation of records sent at or after from,
// how many, their statuses and their times to the answer's end; a request
// without an answer (the run's end cut it, or the transport failed) is
// counted apart, as cut, not timed.
func Summarize(w io.Writer, records []client.Record, from time.Time) error {
	byOp := map[string][]time.Duration{}
	statuses := map[string]map[int]int{}
	cut := map[string]int{}
	for _, r := range records {
		if r.Sent < from.UnixNano() {
			continue
		}
		if statuses[r.Op] == nil {
			statuses[r.Op] = map[int]int{}
		}
		if r.Status == 0 {
			cut[r.Op]++
			continue
		}
		byOp[r.Op] = append(byOp[r.Op], r.Took())
		statuses[r.Op][r.Status]++
	}
	ops := make([]string, 0, len(statuses))
	for op := range statuses {
		ops = append(ops, op)
	}
	sort.Strings(ops)
	for _, op := range ops {
		st, err := json.Marshal(statuses[op])
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%s %s statuses=%s cut=%d\n", op, stats.Of(byOp[op]), st, cut[op]); err != nil {
			return err
		}
	}
	return nil
}
