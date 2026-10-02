package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestCheckTitle(t *testing.T) {
	got, err := domain.CheckTitle("title", "  Straße ")
	if err != nil || got.Name != "Straße" || got.Key != shared.TitleKey("STRASSE") {
		t.Errorf("CheckTitle(Straße) = %+v, %v; want the trimmed name and the key of STRASSE", got, err)
	}
	_, err = domain.CheckTitle("title", "a/b")
	var e *shared.Error
	if !errors.As(err, &e) || e.Kind != shared.KindInvalid || len(e.Fields) != 1 || e.Fields[0].Field != "title" {
		t.Errorf("CheckTitle(a/b) = %v; want 422 on title", err)
	}
}

func TestClientValid(t *testing.T) {
	for _, tt := range []struct {
		c    domain.Client
		want bool
	}{
		{domain.ClientWeb, true}, {domain.ClientAPI, true}, {domain.ClientCLI, true},
		{"mcp:claude-code", true}, {"mcp:" + domain.Client(strings.Repeat("名", 128)), true},
		{"mcp:" + domain.Client(strings.Repeat("名", 129)), false},
		{"mcp:", false}, {"mcp:a\nb", false}, {"MCP:x", false}, {"web ", false}, {"", false}, {"mcp:\xff", false},
	} {
		if got := tt.c.Valid(); got != tt.want {
			t.Errorf("Client(%q).Valid() = %v, want %v", tt.c, got, tt.want)
		}
	}
}
