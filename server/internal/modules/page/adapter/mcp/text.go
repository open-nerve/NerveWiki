package mcpadapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// toolErr is err as a tool answers it, in the module's words (M9/P1
// design 3.3.6, 3.10.5; M9/P2 design 3.10.4, 3.11.5), each path quoted as
// it is (domain.Quoted): a path that names nothing, with its hint while
// the answer holds it; a version token that is not the page's version,
// with the version there now; a page deleted without recursive, counted,
// at the field the call names it (a batch's op's own); a refusal of edit
// locks, with each page's path and whether c's caller holds the lock.
// path is the path the call names, which the last two quote: write's,
// edit's and delete's path, move's from. Any other error is as it is.
func toolErr(err error, c mcpserver.Call, path string) error {
	var (
		missing  *domain.PathNotFound
		mismatch *domain.VersionMismatch
		children *domain.Recursive
		locks    *domain.LockPaths
	)
	switch {
	case errors.As(err, &missing):
		return notFound(err, missing)
	case errors.As(err, &mismatch):
		return &mcpserver.ToolError{Err: err, Text: mismatchText(mismatch),
			Extra: map[string]any{"current_version": mismatch.Current.String()}}
	case errors.As(err, &children):
		return &mcpserver.ToolError{Err: err, Text: "validation_failed: " + domain.Quoted(path) + " has " +
			domain.PagesAndAttachments(children.Pages, children.Assets) +
			" under it: set " + children.Field + " to true to put them in the trash with it."}
	case errors.As(err, &locks):
		return lockErr(err, locks, c, path)
	}
	return err
}

// notFound is page.not_found at missing's path, as it was written, or at
// the parent of the field ParentOf names, and the hint while the answer
// holds it (a fit of one): a hint quotes another path, and two of the
// longest paths are over the bound (M9/P2 design 3.10.3).
func notFound(err error, missing *domain.PathNotFound) error {
	at := domain.Quoted(missing.Path)
	if missing.ParentOf != "" {
		at += ", " + missing.ParentOf + "'s parent"
	}
	return &mcpserver.ToolError{Err: err, Fit: &mcpserver.ErrorFit{N: 1, Build: func(k int) (string, map[string]any) {
		text := "page.not_found: Nothing is at " + at + "."
		if k == 1 && missing.Hint != "" {
			text += " " + missing.Hint
		}
		return text, nil
	}}}
}

// mismatchText is page.revision_mismatch's text for m: what to do, and
// the page's version now.
func mismatchText(m *domain.VersionMismatch) string {
	at, now := domain.Quoted(m.Path), " Its version is "+domain.Quoted(m.Current.String())+"."
	switch m.Why {
	case domain.OtherNode:
		return "page.revision_mismatch: " + at +
			" is another page now than the one base_version was read from: read it, and write from what you read." + now
	case domain.KeptChanging:
		return "page.revision_mismatch: " + at + " changed twice while the edit was made: read it again, and edit what you read." + now
	}
	return "page.revision_mismatch: " + at + " changed since base_version: read it again, and write from what you read." + now
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
