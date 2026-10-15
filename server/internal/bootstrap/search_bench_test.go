//go:build !race

package bootstrap

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// What a search costs on a corpus (M8/P5 design 3.9, step 2): the step 1
// measurement repeated on the product's code, run by hand,
//
//	NERVE_SEARCH_CORPUS=<dir> go test -v -run '^$' -bench 'BenchmarkSearch' -benchtime 1x -timeout 4h ./internal/bootstrap/
//
// where dir holds step 1's COPY files: notebooks.tsv, nodes.tsv and
// contents.tsv (the spike's, whose first 12 and 8 columns are the nodes'
// and the contents'), of the workspace 00000000-0000-4000-8000-000000000002
// and the user 00000000-0000-4000-8000-000000000001. Without it, it is
// skipped. The corpus is written through SQL, every live node queued in
// bulk as the migration's backfill does, and built by the job's runs
// (page.SearchIndex.Drain), timed: the job's throughput. Then each query
// of searchBenchQueries is sent through the handler in process, as serve
// does, by alice, the workspace's admin, who sees every notebook, then as
// a guest of ten of them, then limited to one: the first page, which
// counts, and the third; without statistics on the corpus's tables (the
// job analyzes its own), analyzed, and planned for any arguments. Each
// cell runs once to warm, then NERVE_SEARCH_REPS times (20 when unset),
// and logs its median and 99th percentile, nearest rank, in milliseconds
// (-v keeps every line: a benchmark that passes has its log cut to ten).
func BenchmarkSearch(b *testing.B) {
	dir := os.Getenv("NERVE_SEARCH_CORPUS")
	if dir == "" {
		b.Skip("NERVE_SEARCH_CORPUS names no corpus")
	}
	reps := 20
	if n, err := strconv.Atoi(os.Getenv("NERVE_SEARCH_REPS")); err == nil && n > 0 {
		reps = n
	}
	for range b.N {
		bench := newTrashBench(b, 0, 0)
		corpusWorkspace := bench.loadCorpus(b, dir)
		bench.buildSearch(b)
		bench.searchSizes(b)
		notebooks := queryStrings(b, bench.pool, `SELECT notebook_id::text FROM nodes WHERE deleted_at IS NULL
			GROUP BY notebook_id ORDER BY count(*) DESC, notebook_id`)
		one := notebooks[len(notebooks)/2]
		for _, stats := range []string{"without statistics", "analyzed", "planned for any arguments"} {
			switch stats {
			case "analyzed":
				bench.exec(b, "ANALYZE")
			case "planned for any arguments":
				bench.generic(b)
			}
			for _, vis := range []string{"200 of 200", "10 of 200", "one notebook"} {
				switch vis {
				case "10 of 200":
					bench.asGuestOf(b, corpusWorkspace, notebooks[:10])
				case "one notebook":
					bench.exec(b, "UPDATE workspace_members SET role = 'admin' WHERE user_id = $1", bench.alice)
				}
				for _, q := range searchBenchQueries() {
					for _, shape := range []string{"first", "third"} {
						more := url.Values{}
						if vis == "one notebook" {
							more.Set("notebook_id", one)
						}
						took, last := bench.searchTimes(b, q, more, shape == "third", reps)
						b.Logf("%-26s %-12s %-26s %-5s p50 %8.1f p99 %8.1f  %s", stats, vis, q, shape, nearestRank(took, 0.5),
							nearestRank(took, 0.99), last)
					}
				}
			}
		}
	}
}

// searchBenchQueries are step 1's queries (M8/P5 design 3.9): CJK terms of
// one to four characters, of each frequency; mixed; English, common to
// rare, of one or two letters and a substring; two terms and eight; the
// most common keys; rare words of common trigrams, which exhaust the walk;
// a word of many names and few contents; a count at its bound.
func searchBenchQueries() []string {
	return []string{
		"的", "锅", "囧", "犇", "中国", "索引", "绩效", "鉴权", "共和国", "数据库", "元数据", "微服务", "参考文献", "轨道交通", "空中加油",
		"全文检索", "C语言", "Windows系统", "Go语言", "API接口", "the", "linux", "latency", "kubernetes", "ai", "中国 历史",
		"数据库 索引", "Go 并发", "go", "c", "api", "锅 中国", "A股", "锅 #", "中国 历史 数据库 索引 latency Go 并发 系统", "ing", "a",
		"of", "veress", "ranters", "latere", "kernel", "geass", "pope", "history",
	}
}

// corpusTables are the corpus's files and the columns of the table each
// fills, in the file's order: a line's further fields are left out.
func corpusTables() []struct{ file, table, columns string } {
	return []struct{ file, table, columns string }{
		{"notebooks.tsv", "notebooks", ""},
		{"nodes.tsv", "nodes", "id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, " +
			"updated_at, deleted_at"},
		{"contents.tsv", "page_contents", "node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at, deleted_at"},
	}
}

// loadCorpus writes the corpus of dir through COPY, its user and its
// workspace first, alice its admin, and answers the workspace's slug.
func (b trashBench) loadCorpus(tb testing.TB, dir string) string {
	tb.Helper()
	const user, workspace, slug = "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "corpus"
	b.exec(tb, `INSERT INTO users (id, email, password, display_name, created_at, updated_at)
		VALUES ($1, 'corpus@example.com', 'x', 'Corpus', now(), now())`, user)
	b.exec(tb, `INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at)
		VALUES ($1, $2, 'Corpus', $3, $3, now(), now())`, workspace, slug, user)
	b.exec(tb, `INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, 'admin', $2, $2, now(), now())`, workspace, b.alice)
	conn, err := b.pool.Acquire(context.Background())
	if err != nil {
		tb.Fatal(err)
	}
	defer conn.Release()
	for _, t := range corpusTables() {
		at := time.Now()
		f, err := os.Open(filepath.Join(dir, t.file))
		if err != nil {
			tb.Fatal(err)
		}
		fields := strings.Count(t.columns, ",") + 1
		r, w := io.Pipe()
		go func() { w.CloseWithError(cutFields(w, f, fields, t.columns == "")) }()
		columns := ""
		if t.columns != "" {
			columns = " (" + t.columns + ")"
		}
		tag, err := conn.Conn().PgConn().CopyFrom(context.Background(), r, "COPY "+t.table+columns+" FROM STDIN")
		_ = f.Close()
		if err != nil {
			tb.Fatalf("COPY %s: %v", t.table, err)
		}
		tb.Logf("COPY %s: %d rows in %s", t.table, tag.RowsAffected(), time.Since(at).Round(time.Second))
	}
	return slug
}

// cutFields copies the lines of r to w, each cut to its first n
// tab-separated fields, or whole when all.
func cutFields(w io.Writer, r io.Reader, n int, all bool) error {
	in := bufio.NewReaderSize(r, 1<<20)
	out := bufio.NewWriterSize(w, 1<<20)
	for {
		line, err := in.ReadBytes('\n')
		if len(line) > 0 && !all {
			at := 0
			for range n {
				j := bytes.IndexByte(line[at:], '\t')
				if j < 0 {
					at = -1
					break
				}
				at += j + 1
			}
			if at > 0 {
				line = append(line[:at-1], '\n')
			}
		}
		if _, werr := out.Write(line); werr != nil {
			return werr
		}
		if errors.Is(err, io.EOF) {
			return out.Flush()
		}
		if err != nil {
			return err
		}
	}
}

// buildSearch queues every live node in bulk, as the migration's backfill
// does, and runs the job until the queue is empty, timed.
func (b trashBench) buildSearch(tb testing.TB) {
	tb.Helper()
	b.exec(tb, `INSERT INTO page_search_queue (node_id, notebook_id, bulk) SELECT id, notebook_id, true FROM nodes
		WHERE deleted_at IS NULL ORDER BY id ON CONFLICT (node_id) DO NOTHING`)
	index := page.NewSearchIndex(b.pool, 2*time.Second)
	var written, runs int
	at := time.Now()
	for count(tb, b.pool, "SELECT count(*) FROM page_search_queue") > 0 {
		run, err := index.Drain(context.Background())
		if err != nil || run.Failed != 0 {
			tb.Fatalf("Drain() = %+v, %v", run, err)
		}
		written += run.Written
		runs++
	}
	took := time.Since(at)
	bytes := count(tb, b.pool, "SELECT coalesce(sum(byte_size), 0)::bigint FROM page_contents WHERE deleted_at IS NULL")
	tb.Logf("the job built %d rows in %d runs, %s: %.1f rows/s, %.2f MB/s of content", written, runs, took.Round(time.Second),
		float64(written)/took.Seconds(), float64(bytes)/took.Seconds()/1e6)
}

// searchSizes logs page_search's sizes against the content's.
func (b trashBench) searchSizes(tb testing.TB) {
	tb.Helper()
	tb.Logf("sizes: %s", strings.Join(queryStrings(tb, b.pool, `SELECT format('heap %s MB, TOAST %s MB, title %s MB, body %s MB, '
			|| 'total %s MB, %s times the content''s %s MB',
		pg_relation_size('page_search') / 1000000, pg_relation_size(c.reltoastrelid) / 1000000,
		pg_relation_size('page_search_title_idx') / 1000000, pg_relation_size('page_search_body_idx') / 1000000,
		pg_total_relation_size('page_search') / 1000000,
		round(pg_total_relation_size('page_search')::numeric / (SELECT sum(byte_size) FROM page_contents WHERE deleted_at IS NULL), 2),
		(SELECT sum(byte_size) FROM page_contents WHERE deleted_at IS NULL) / 1000000)
		FROM pg_class c WHERE c.oid = 'page_search'::regclass`), ""))
}

// asGuestOf makes alice a guest of the workspace slug, a reader of the
// notebooks alone: at each stage of statistics, her memberships of the
// stage before kept.
func (b trashBench) asGuestOf(tb testing.TB, slug string, notebooks []string) {
	tb.Helper()
	b.exec(tb, `UPDATE workspace_members SET role = 'guest' WHERE user_id = $1
		AND workspace_id = (SELECT id FROM workspaces WHERE slug = $2)`, b.alice, slug)
	for _, nb := range notebooks {
		b.exec(tb, `INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2, 'reader', $2, $2, now(), now()) ON CONFLICT (notebook_id, user_id) WHERE deleted_at IS NULL DO NOTHING`,
			nb, b.alice)
	}
}

// searchTimes sends alice's search of the corpus for q with more, reps
// times after a warm one, and answers how long each took and what the last
// answered: its hits, its count, whether it says building. The third page
// follows the first's cursors, untimed; a page without a cursor ends there.
func (b trashBench) searchTimes(tb testing.TB, q string, more url.Values, third bool, reps int) ([]time.Duration, string) {
	tb.Helper()
	path := searchPath("corpus", q, more)
	if third {
		for range 2 {
			status, body := b.serve(http.MethodGet, path, "")
			var a searchAnswer
			if status != http.StatusOK {
				tb.Fatalf("GET %s = %d %s", path, status, body)
			}
			decodeAnswer(tb, body, &a)
			if a.NextCursor == nil {
				return nil, "no third page"
			}
			v := url.Values{}
			for k, vs := range more {
				v[k] = vs
			}
			v.Set("cursor", *a.NextCursor)
			path = searchPath("corpus", q, v)
		}
	}
	var took []time.Duration
	var last string
	for i := range reps + 1 {
		at := time.Now()
		status, body := b.serve(http.MethodGet, path, "")
		d := time.Since(at)
		if status != http.StatusOK {
			tb.Fatalf("GET %s = %d %s", path, status, body)
		}
		if i == 0 {
			continue
		}
		took = append(took, d)
		var a searchAnswer
		decodeAnswer(tb, body, &a)
		count := "null"
		if a.Count != nil {
			count = fmt.Sprintf("%s %d", a.Count.Kind, a.Count.Value)
		}
		last = fmt.Sprintf("hits %d, next %t, count %s, building %t", len(a.Data), a.NextCursor != nil, count, a.Building)
	}
	return took, last
}

// nearestRank is the p-th quantile of took by the nearest rank, in
// milliseconds; 0 for none.
func nearestRank(took []time.Duration, p float64) float64 {
	if len(took) == 0 {
		return 0
	}
	s := slices.Sorted(slices.Values(took))
	i := int(p*float64(len(s))+0.999999) - 1
	return float64(s[max(0, min(len(s)-1, i))]) / float64(time.Millisecond)
}
