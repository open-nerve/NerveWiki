package domain_test

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/yuin/goldmark"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func filePath(s string) domain.FilePath { return domain.FilePath{Label: s, Destination: &s} }

func fileIndex(items ...domain.IndexFileItem) domain.RenderIndexInput {
	return domain.RenderIndexInput{Items: items, Total: len(items), SourcePath: filePath("raw.md"), Sources: 3}
}

func rendered(t *testing.T, in domain.RenderIndexInput, budget int) string {
	t.Helper()
	b, err := domain.RenderIndex(in, budget)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestVirtualFiles(t *testing.T) {
	t.Run("ingest", testVirtualIngestLog)
	t.Run("display_limits", func(t *testing.T) {
		summary, typ := strings.Repeat("é", 130), strings.Repeat("α", 130)
		tags := make([]string, 21)
		for i := range tags {
			tags[i] = fmt.Sprintf("tag%02d", i)
		}
		destination := strings.Repeat("L", 600) + ".md"
		got := rendered(t, fileIndex(domain.IndexFileItem{Path: filePath(destination), Title: strings.Repeat("T", 300), Type: &typ, Summary: &summary, Tags: tags}), 40960)
		for _, want := range []string{"## Type: " + strings.Repeat("α", 126) + "…", "[" + strings.Repeat("T", 253) + "…](<" + destination + ">)", " — " + strings.Repeat("é", 126) + "…", "tag19, …", "Use index with type, tag or parent"} {
			if !strings.Contains(got, want) {
				t.Errorf("missing marked display or destination %q", want)
			}
		}
		if strings.Contains(got, "tag20") || !utf8.ValidString(got) {
			t.Fatal("twenty-first tag displayed or UTF-8 split")
		}
		message := strings.Repeat("é", 300) + "TAIL"
		b, err := domain.RenderLog(domain.RenderLogInput{Items: []domain.RenderLogChangeset{{Message: &message}}, Total: 1}, 40960)
		if err != nil || !strings.Contains(string(b), "message: "+strings.Repeat("é", 248)+"…") || strings.Contains(string(b), "TAIL") || !strings.Contains(string(b), "Use log to filter and page through more.") {
			t.Errorf("log display boundary: %v %q", err, b)
		}
		for _, tt := range []struct {
			name, input, want string
			limit             int
		}{
			{"title_ascii_exact", strings.Repeat("A", 256), strings.Repeat("A", 256), 256},
			{"title_ascii_cut", strings.Repeat("A", 257), strings.Repeat("A", 253) + "…", 256},
			{"title_utf8_exact", strings.Repeat("界", 85) + "a", strings.Repeat("界", 85) + "a", 256},
			{"title_utf8_cut", strings.Repeat("界", 86), strings.Repeat("界", 84) + "…", 256},
			{"title_bracket_exact", strings.Repeat("[", 128), strings.Repeat(`\[`, 128), 256},
			{"title_bracket_cut", strings.Repeat("[", 129), strings.Repeat(`\[`, 126) + "…", 256},
			{"title_slash_exact", strings.Repeat(`\`, 128), strings.Repeat(`\\`, 128), 256},
			{"title_slash_cut", strings.Repeat(`\`, 129), strings.Repeat(`\\`, 126) + "…", 256},
			{"title_backtick_exact", strings.Repeat("`", 128), strings.Repeat("\\`", 128), 256},
			{"title_backtick_cut", strings.Repeat("`", 129), strings.Repeat("\\`", 126) + "…", 256},
			{"message_ascii_exact", strings.Repeat("A", 500), strings.Repeat("A", 500), 500},
			{"message_ascii_cut", strings.Repeat("A", 501), strings.Repeat("A", 497) + "…", 500},
			{"message_utf8_exact", strings.Repeat("界", 166) + "ab", strings.Repeat("界", 166) + "ab", 500},
			{"message_utf8_cut", strings.Repeat("界", 167), strings.Repeat("界", 165) + "…", 500},
			{"message_bracket_exact", strings.Repeat("]", 250), strings.Repeat(`\]`, 250), 500},
			{"message_bracket_cut", strings.Repeat("]", 251), strings.Repeat(`\]`, 248) + "…", 500},
			{"message_slash_exact", strings.Repeat(`\`, 250), strings.Repeat(`\\`, 250), 500},
			{"message_slash_cut", strings.Repeat(`\`, 251), strings.Repeat(`\\`, 248) + "…", 500},
			{"message_backtick_exact", strings.Repeat("`", 250), strings.Repeat("\\`", 250), 500},
			{"message_backtick_cut", strings.Repeat("`", 251), strings.Repeat("\\`", 248) + "…", 500},
		} {
			var label string
			if tt.limit == 256 {
				text := rendered(t, fileIndex(domain.IndexFileItem{Path: filePath(destination), Title: tt.input}), 40960)
				_, rest, found := strings.Cut(text, "\n- [")
				var complete bool
				label, _, complete = strings.Cut(rest, "](<"+destination+">)")
				if !found || !complete {
					t.Fatalf("%s: missing complete destination identity in %q", tt.name, text)
				}
			} else {
				body, err := domain.RenderLog(domain.RenderLogInput{Items: []domain.RenderLogChangeset{{Message: &tt.input}}, Total: 1}, 40960)
				if err != nil {
					t.Fatal(err)
				}
				_, rest, found := strings.Cut(string(body), "; message: ")
				label, _, _ = strings.Cut(rest, "\n")
				if !found {
					t.Fatalf("%s: missing message", tt.name)
				}
			}
			if len(label) > tt.limit || !utf8.ValidString(label) || label != tt.want {
				t.Errorf("%s: encoded display has %d bytes (limit %d): got %q, want %q", tt.name, len(label), tt.limit, label, tt.want)
			}
		}
	})
	t.Run("escape_bytes", func(t *testing.T) {
		summary := "a\n# forged\x01"
		got := rendered(t, fileIndex(domain.IndexFileItem{Path: filePath("a[one]<two>.md"), Title: `T[\]`, Summary: &summary}), 40960)
		for _, want := range []string{`[T\[\\\]](<a[one]%3Ctwo%3E.md>)`, " — a # forged "} {
			if !strings.Contains(got, want) {
				t.Errorf("missing safe fragment %q in %q", want, got)
			}
		}
	})
	t.Run("destination", func(t *testing.T) {
		got := rendered(t, fileIndex(domain.IndexFileItem{Path: filePath("A)<evil>.md"), Title: "A"}), 40960)
		if !strings.Contains(got, `](<A)%3Cevil%3E.md>)`) || strings.Contains(got, `](<A)<evil>.md>)`) {
			t.Fatalf("unsafe destination: %q", got)
		}
		for _, bad := range []string{"secret\x00name", "secret\nname", "secret\xffname"} {
			_, err := domain.RenderIndex(fileIndex(domain.IndexFileItem{Path: filePath(bad)}), 40960)
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("invalid destination was repaired or exposed: %v", err)
			}
		}
	})
	t.Run("newline", func(t *testing.T) {
		message := "message\n# forged\r\n- injected\x01\u2028end"
		got, err := domain.RenderLog(domain.RenderLogInput{Items: []domain.RenderLogChangeset{{Kind: "edit", Client: "mcp:\nclient", ByName: "user\rname", Message: &message}}, Total: 1}, 40960)
		if err != nil {
			t.Fatal(err)
		}
		s := string(got)
		if !strings.Contains(s, "message: message # forged  - injected  end") || strings.Contains(s, "\n# forged") || strings.Contains(s, "\n- injected") || strings.ContainsAny(s, "\r\x01\u2028") {
			t.Fatalf("multiline display injection: %q", s)
		}
	})
	t.Run("limit_after_escape", func(t *testing.T) {
		items := make([]domain.IndexFileItem, 300)
		for i := range items {
			items[i] = domain.IndexFileItem{Path: filePath("é.md"), Title: strings.Repeat(`[界]`, 40)}
		}
		got := rendered(t, fileIndex(items...), 40<<10)
		if len(got) > 40<<10 || !utf8.ValidString(got) {
			t.Fatalf("escaped file exceeds 40KiB or splits UTF-8: %d", len(got))
		}
		if n := strings.Count(got, "\n- "); n == 0 || n >= len(items) || !strings.Contains(got, fmt.Sprintf("Showing %d of 300 pages.", n)) {
			t.Fatalf("wrong whole-item count: %d", n)
		}
	})
	t.Run("actual_footer", func(t *testing.T) {
		// Literal bytes are independent of the renderer: exactly two of three
		// equal candidates fit, including the required source line and footer.
		header := "# Index\n\nSource area: [raw.md](<raw.md>) — 3 sources. Use pending_sources or lint for status.\n\n"
		group := "## Parent: /\n\n"
		line := "- [A](<A.md>); changed: 0001-01-01T00:00:00Z\n"
		footer := "\nShowing 2 of 3 pages. Use index with type, tag or parent to filter and page through more.\n"
		it := domain.IndexFileItem{Path: filePath("A.md"), Title: "A"}
		got := rendered(t, fileIndex(it, it, it), len(header)+len(group)+2*len(line)+len(footer))
		if got != header+group+line+line+footer {
			t.Fatalf("actual footer or whole item differs: %q", got)
		}
		if _, err := domain.RenderIndex(fileIndex(it), 10); err == nil {
			t.Fatal("mandatory sections silently truncated")
		}
	})
	t.Run("percent_identity", func(t *testing.T) {
		for _, tt := range []struct{ name, literal, other string }{{"percent41", "x%41.md", "xA.md"}, {"percent20", "x%20y.md", "x y.md"}} {
			t.Run(tt.name, func(t *testing.T) {
				in := fileIndex(domain.IndexFileItem{Path: filePath(tt.literal)}, domain.IndexFileItem{Path: filePath(tt.other)})
				got := rendered(t, in, 40960)
				matches := regexp.MustCompile(`\]\(<([^>]*)>\)`).FindAllStringSubmatch(got, -1)
				if len(matches) != 3 {
					t.Fatalf("links = %v", matches)
				}
				// Resolve the emitted URL exactly once against a hand-built map.
				ids := map[string]string{tt.literal: "literal-percent-node", tt.other: "other-node"}
				for i, want := range []string{"literal-percent-node", "other-node"} {
					decoded, err := url.PathUnescape(matches[i+1][1])
					if err != nil || ids[decoded] != want {
						t.Fatalf("link %q resolves to %q (%v), want %s", matches[i+1][1], ids[decoded], err, want)
					}
				}
			})
		}
	})
	t.Run("entity_identity", func(t *testing.T) {
		for _, tt := range []struct{ name, literal, other string }{
			{"amp", "x&amp;Y.md", "x&Y.md"},
			{"slash", "folder&sol;leaf.md", "folder/leaf.md"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				in := fileIndex(domain.IndexFileItem{Path: filePath(tt.literal), Title: "Literal"}, domain.IndexFileItem{Path: filePath(tt.other), Title: "Other"})
				md := rendered(t, in, 40960)
				var output bytes.Buffer
				if err := goldmark.Convert([]byte(md), &output); err != nil {
					t.Fatal(err)
				}
				links := regexp.MustCompile(`href="([^"]*)"`).FindAllStringSubmatch(output.String(), -1)
				if len(links) != 3 {
					t.Fatalf("rendered links = %v in %q", links, output.String())
				}
				// The renderer and browser both interpret entities. Resolve the
				// actual HTML href, then the URL, against independent node IDs.
				ids := map[string]string{tt.literal: "literal-entity-node", tt.other: "other-node"}
				for i, want := range []string{"literal-entity-node", "other-node"} {
					decoded, err := url.PathUnescape(html.UnescapeString(links[i+1][1]))
					if err != nil || ids[decoded] != want {
						t.Errorf("rendered entity link %q resolves to %q (%v), want %s", links[i+1][1], ids[decoded], err, want)
					}
				}
			})
		}
	})
	t.Run("backtick_identity", func(t *testing.T) {
		// A code span can consume the label's closing bracket and destination.
		// Render the actual output, then resolve its href, not Markdown syntax.
		for _, tt := range []struct{ title, path string }{
			{"tick`name", "tick`name.md"},
			{"tick``name", "tick``name.md"},
			{"`matched`", "`matched`.md"},
		} {
			t.Run(tt.title, func(t *testing.T) {
				index := rendered(t, fileIndex(domain.IndexFileItem{Title: tt.title, Path: filePath(tt.path)}), 40960)
				log, err := domain.RenderLog(domain.RenderLogInput{Items: []domain.RenderLogChangeset{{Changes: []domain.RenderLogChange{{Path: domain.FilePath{Label: tt.title, Destination: &tt.path}}}}}, Total: 1}, 40960)
				if err != nil {
					t.Fatal(err)
				}
				for _, body := range []string{index, string(log)} {
					var output bytes.Buffer
					if err := goldmark.Convert([]byte(body), &output); err != nil {
						t.Fatal(err)
					}
					found := false
					for _, link := range regexp.MustCompile(`href="([^"]*)"`).FindAllStringSubmatch(output.String(), -1) {
						path, err := url.PathUnescape(html.UnescapeString(link[1]))
						if err != nil {
							t.Fatal(err)
						}
						found = found || path == tt.path
					}
					if !found || !strings.Contains(output.String(), ">"+tt.title+"</a>") {
						t.Errorf("literal title or complete link %q lost in rendered Markdown: %q", tt.path, output.String())
					}
				}
			})
		}
	})
	t.Run("percent_budget", func(t *testing.T) {
		for _, tt := range []struct {
			name  string
			bytes int
		}{{"read", 40 << 10}, {"export", 2 << 20}} {
			t.Run(tt.name, func(t *testing.T) {
				items := make([]domain.IndexFileItem, 1000)
				for i := range items {
					items[i] = domain.IndexFileItem{Path: filePath(strings.Repeat("%", 1300) + ".md"), Title: "percent"}
				}
				got := rendered(t, fileIndex(items...), tt.bytes)
				if len(got) > tt.bytes || !utf8.ValidString(got) {
					t.Fatalf("encoded output exceeded budget: %d > %d", len(got), tt.bytes)
				}
				links := regexp.MustCompile(`\]\(<([^>]*)>\)`).FindAllStringSubmatch(got, -1)
				n := len(links) - 1
				if n < 1 || n >= len(items) || !strings.HasSuffix(got, "\nShowing "+strconv.Itoa(n)+" of 1000 pages. Use index with type, tag or parent to filter and page through more.\n") {
					t.Fatalf("partial item or dishonest count: %d", n)
				}
				for _, link := range links[1:] {
					if link[1] != strings.Repeat("%25", 1300)+".md" {
						t.Fatal("destination was shortened or not percent-encoded")
					}
				}
			})
		}
	})
}
