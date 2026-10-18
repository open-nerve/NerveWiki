package mcpadapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// toolErr is err as a tool answers it (M9/P1 design 3.3.6, 3.10.5): a
// path that names nothing in the module's words, with the path as it was
// written, quoted as it is (M9/P2 design 3.10.4), and its hint; any other
// error as it is.
func toolErr(err error) error {
	var missing *domain.PathNotFound
	if !errors.As(err, &missing) {
		return err
	}
	text := strings.TrimSpace("page.not_found: Nothing is at " + domain.Quoted(missing.Path) + ". " + missing.Hint)
	return &mcpserver.ToolError{Err: err, Text: text}
}

// line is v's JSON as one line, as mcpserver encodes a structured result:
// without the HTML escapes, so that the model reads <, > and & as they
// are, and without the encoder's newline.
func line(v any) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}
