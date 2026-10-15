//go:build !race

package bootstrap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

// What a save costs (M8/P5 design 3.9, step 2), run by hand on a branch
// and on its base, the file copied there, so that the difference is what
// the branch adds to a save (P5A: the search's queueing in its unit):
//
//	go test -run '^$' -bench 'BenchmarkSave' -benchtime 300x ./internal/bootstrap/
//
// Each iteration saves a page of the size through the handler, as serve
// does, its content changed by a word; it reports the median and the 99th
// percentile, nearest rank, in milliseconds.
func BenchmarkSave(b *testing.B) {
	for _, size := range []int{10 << 10, 200 << 10} {
		b.Run(fmt.Sprintf("%d KiB", size>>10), func(b *testing.B) {
			bench := newTrashBench(b, 0, 0)
			status, answer := bench.serve(http.MethodPost, "/api/v0/notebooks/"+bench.nb.String()+"/pages",
				jsonBody(map[string]any{"parent_id": nil, "title": "Saved", "content": savedContent(size, 0)}))
			if status != http.StatusCreated {
				b.Fatalf("create the page = %d %s", status, answer)
			}
			var page struct {
				ID       string `json:"id"`
				Revision int    `json:"revision"`
			}
			if err := json.Unmarshal([]byte(answer), &page); err != nil {
				b.Fatal(err)
			}
			rev := page.Revision
			took := make([]time.Duration, 0, b.N)
			b.ResetTimer()
			for i := range b.N {
				body := jsonBody(map[string]any{"content": savedContent(size, i+1), "base_revision": rev})
				at := time.Now()
				status, answer := bench.serve(http.MethodPut, "/api/v0/pages/"+page.ID+"/content", body)
				took = append(took, time.Since(at))
				if status != http.StatusOK {
					b.Fatalf("save %d = %d %s", i, status, answer)
				}
				rev++
			}
			b.StopTimer()
			slices.Sort(took)
			at := func(p float64) float64 {
				return float64(took[min(len(took)-1, int(p*float64(len(took))+0.999999)-1)]) / float64(time.Millisecond)
			}
			b.ReportMetric(at(0.5), "p50-ms")
			b.ReportMetric(at(0.99), "p99-ms")
		})
	}
}

// savedContent is a page of size bytes or so, of paragraphs of words and
// CJK characters, its first word the version n: each save changes it.
func savedContent(size, n int) string {
	words := []string{"latency", "数据库", "the", "kernel", "索引", "history", "中国", "Go", "并发", "linux", "系统", "api"}
	var s strings.Builder
	fmt.Fprintf(&s, "version %d\n\n", n)
	for i := 0; s.Len() < size; i++ {
		s.WriteString(words[(i*7+i/len(words))%len(words)])
		if i%12 == 11 {
			s.WriteString("\n\n")
		} else {
			s.WriteString(" ")
		}
	}
	return s.String()
}
