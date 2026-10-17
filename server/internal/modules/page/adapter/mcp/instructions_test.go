package mcpadapter

import (
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// The built-in instructions are short, a handshake's text of at most
// 4 KiB each; the notebook entry's tell of the AGENTS page that replaces
// them, and both share their paths and tools (M9/P1 design 3.15.4).
// bootstrap's TestTheBuiltInInstructionsNameEveryTool holds their tools
// to the contract.
func TestTheBuiltInInstructionsAreShort(t *testing.T) {
	for name, text := range map[string]string{"the notebook entry's": NotebookInstructions, "the account entry's": AccountInstructions} {
		if n := mcpserver.TextBytes(text); n > 4<<10 {
			t.Errorf("%s instructions are %d bytes, over 4 KiB", name, n)
		}
		if !strings.Contains(text, pathsAndTools) {
			t.Errorf("%s instructions lack the paths and the tools", name)
		}
	}
	if !strings.Contains(NotebookInstructions, "A root page titled AGENTS") || strings.Contains(AccountInstructions, "AGENTS") {
		t.Error("the notebook entry's instructions, and they alone, tell of the AGENTS page")
	}
}
