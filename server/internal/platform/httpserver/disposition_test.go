package httpserver_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// A file's name is written in ASCII, the rest an underscore, and in UTF-8,
// each byte outside RFC 8187's attr-char escaped.
func TestDisposition(t *testing.T) {
	for _, tt := range []struct {
		inline bool
		name   string
		want   string
	}{
		{true, "a.png", `inline; filename="a.png"; filename*=UTF-8''a.png`},
		{false, "会议 纪要.zip", `attachment; filename="__ __.zip"; filename*=UTF-8''%E4%BC%9A%E8%AE%AE%20%E7%BA%AA%E8%A6%81.zip`},
		{false, `a"b\c;d.zip`, `attachment; filename="a_b_c;d.zip"; filename*=UTF-8''a%22b%5Cc%3Bd.zip`},
	} {
		if got := httpserver.Disposition(tt.inline, tt.name); got != tt.want {
			t.Errorf("Disposition(%v, %q) = %s, want %s", tt.inline, tt.name, got, tt.want)
		}
	}
}
