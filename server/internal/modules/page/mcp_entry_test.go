package page

import (
	"context"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// agentsOf is a notebook whose root page AGENTS holds content, at revision
// 1 and with no frontmatter, for the instructions' use case: the other
// reads are none of its.
type agentsOf struct {
	app.Nodes
	app.Markdown
	node    uuid.UUID
	content string
}

func (a agentsOf) AgentsPage(context.Context, uuid.UUID, string) (uuid.UUID, domain.Kind, string, int, bool, error) {
	return a.node, domain.KindPage, domain.AgentsTitle, 1, true, nil
}

func (a agentsOf) PageContent(context.Context, uuid.UUID) (app.PageContent, error) {
	return app.PageContent{Content: a.content, Revision: 1}, nil
}

func (agentsOf) FrontmatterEnd(string) int { return 0 }

// measured is a body that the handshake's text counts n bytes of: lines of
// "<", "漢" and "x", padded with "x".
func measured(n int) string {
	s := strings.Repeat("<漢 x\n", n/16)
	return s + strings.Repeat("x", n-mcpserver.TextBytes(s))
}

// The handshake's instructions hold an AGENTS page whole up to
// MaxInstructionsBytes as its text counts them, 32,768, and cut it one
// byte past, with the note, within the bound (M9/P1 design 3.15.2, 6.2,
// S3 review A-M1).
func TestTheHandshakesInstructionsAreWholeUpToTheirBound(t *testing.T) {
	const note = `(The rest of AGENTS.md is cut here: read it whole with read, path "AGENTS.md".)`
	for _, n := range []int{domain.MaxInstructionsBytes, domain.MaxInstructionsBytes + 1} {
		body := measured(n)
		if mcpserver.TextBytes(body) != n {
			t.Fatalf("a body of %d bytes measured %d", n, mcpserver.TextBytes(body))
		}
		page := agentsOf{node: uuid.NewV7(), content: body}
		entry := MCPEntry{instructions: app.NewInstructions(page, page, page, mcpserver.TextBytes)}
		got, err := entry.Instructions(context.Background(), uuid.NewV7())
		switch {
		case err != nil:
			t.Fatalf("%d bytes: %v", n, err)
		case mcpserver.TextBytes(got) > domain.MaxInstructionsBytes:
			t.Errorf("%d bytes: the instructions are %d", n, mcpserver.TextBytes(got))
		case n <= domain.MaxInstructionsBytes && got != body:
			t.Errorf("%d bytes: cut to %d bytes, want the body whole", n, mcpserver.TextBytes(got))
		case n > domain.MaxInstructionsBytes &&
			(!strings.HasSuffix(got, "\n\n"+note) || !strings.HasPrefix(body, strings.TrimSuffix(got, "\n\n"+note))):
			t.Errorf("%d bytes: %d bytes not a prefix of the body and the note", n, mcpserver.TextBytes(got))
		}
	}
}
