package domain_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Flip changes the one byte between the brackets, whatever it held, and
// leaves the rest, line breaks and all, as it was.
func TestFlipChangesTheOneByte(t *testing.T) {
	for _, tt := range []struct {
		content string
		offset  int
		checked bool
		want    string
	}{
		{"- [ ] a\r\n", 3, true, "- [x] a\r\n"},
		{"- [x] a\r\n", 3, false, "- [ ] a\r\n"},
		{"\ufeff- [X] a", 6, false, "\ufeff- [ ] a"},
		{"- [\t] Café\n", 3, true, "- [x] Café\n"},
		{"- [x] a", 3, true, "- [x] a"},
	} {
		if got := domain.Flip(tt.content, tt.offset, tt.checked); got != tt.want {
			t.Errorf("Flip(%q, %d, %t) = %q, want %q", tt.content, tt.offset, tt.checked, got, tt.want)
		}
	}
}
