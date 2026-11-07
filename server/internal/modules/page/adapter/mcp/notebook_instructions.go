package mcpadapter

import (
	"errors"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// maxSystemBytes bounds an LLM Wiki's system paragraph, as it is written,
// not as a text block escapes it (M10 design 4.15).
const maxSystemBytes = 512

// errNoRoomForInstructions is a fits too small for the system paragraph
// and the note: no notebook's answer leaves so little.
var errNoRoomForInstructions = errors.New("page: the instructions do not fit their answer")

// NotebookText is a notebook's instructions for entry, both entries'
// one writer (M10 design 4.15, decision 161): an LLM Wiki's system
// paragraph first, whole, then the AGENTS page's text, cut as fits and
// MaxInstructionsBytes allow what is measured with the paragraph, or the
// built-in instructions when it has none. A standard notebook has no
// paragraph. AgentsText falls back to its note without telling, so the
// whole is measured once more here.
func NotebookText(i app.NotebookInstructions, entry mcpserver.Entry, fits func(string) bool) (string, error) {
	system := systemParagraph(i, entry)
	whole := func(text string) string {
		if system == "" {
			return text
		}
		return system + "\n\n" + text
	}
	within := func(text string) bool {
		return mcpserver.TextBytes(text) <= domain.MaxInstructionsBytes && fits(text)
	}
	text := whole(NotebookInstructions)
	if i.HasAgents {
		text = whole(domain.AgentsText(i.Agents, func(s string) bool { return within(whole(s)) }))
	}
	if !within(text) {
		return "", errNoRoomForInstructions
	}
	return text, nil
}

// systemParagraph is an LLM Wiki's paragraph for entry, "" for a standard
// notebook: within maxSystemBytes with the source root's path, else its
// title, else as much of the title as fits with "…". Only that part ever
// gives way.
func systemParagraph(i app.NotebookInstructions, entry mcpserver.Entry) string {
	if i.Mode != domain.ModeLLMWiki {
		return ""
	}
	const lead = "This notebook is an LLM Wiki: pages under `"
	const generated = "/` are sources you add once and never change; index.md and log.md are generated; "
	tail := generated + "run the prompts ingest, query and lint."
	if entry == mcpserver.AccountEntry {
		tail = generated + "its prompts ingest, query and lint are served at `" + mcpserver.Pattern + "?notebook=" + i.NotebookID.String() + "`."
	}
	room := maxSystemBytes - len(lead) - len(tail)
	switch {
	case len(i.SourceRootPath) <= room:
		return lead + i.SourceRootPath + tail
	case len(i.SourceRootTitle) <= room:
		return lead + i.SourceRootTitle + tail
	}
	return lead + cutTitle(i.SourceRootTitle, room) + tail
}

// cutTitle is title, longer than room, cut at a character's boundary with
// "…" after it, within room.
func cutTitle(title string, room int) string {
	const mark = "…"
	n := room - len(mark)
	for n > 0 && !utf8.RuneStart(title[n]) {
		n--
	}
	return title[:n] + mark
}
