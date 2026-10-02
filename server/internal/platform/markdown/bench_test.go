package markdown_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

// The benchmarks of an ordinary document (M4/P3 design 3.10), run by hand;
// their results are in the P3 document's:
//
//	cd server && go test -run '^$' -bench . -benchmem ./internal/platform/markdown

func sizes() []int { return []int{100 << 10, 1 << 20, 5 << 20} }

func BenchmarkParse(b *testing.B) {
	m := newMarkdown(b)
	for _, size := range sizes() {
		content := []byte(markdowntest.Normal(size))
		b.Run(fmt.Sprintf("%dKB", size>>10), func(b *testing.B) {
			b.SetBytes(int64(len(content)))
			for b.Loop() {
				m.Parse(content)
			}
		})
	}
}

func BenchmarkParseAndRender(b *testing.B) {
	m := newMarkdown(b)
	for _, size := range sizes() {
		content := []byte(markdowntest.Normal(size))
		b.Run(fmt.Sprintf("%dKB", size>>10), func(b *testing.B) {
			b.SetBytes(int64(len(content)))
			for b.Loop() {
				if _, err := m.Render(context.Background(), m.Parse(content), markdown.Page{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
