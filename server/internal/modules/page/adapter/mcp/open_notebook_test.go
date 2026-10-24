package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// opened is an OpenNotebook of an answer.
type opened struct {
	answer app.Opened
	err    error
}

func (o opened) Execute(context.Context, uuid.UUID) (app.Opened, error) { return o.answer, o.err }

// nb is the notebook the tests open.
var nb = uuid.MustParse("01920000-0000-7000-8000-0000000000b0") //nolint:gochecknoglobals // read only

// openOf is open_notebook's answer to o, its JSON as Fit encoded it.
func openOf(t *testing.T, o opened) (openAnswer, string, error) {
	t.Helper()
	res, err := tools{uc: UseCases{Open: o}}.openNotebook(context.Background(), mcpserver.Call{Notebook: nb}, struct{}{})
	if err != nil {
		return openAnswer{}, "", err
	}
	raw := rawOf(t, res)
	var a openAnswer
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatalf("the answer %s: %v", raw, err)
	}
	return a, raw, nil
}

// The answer is the notebook, its mode, its instructions and where they
// come from, null for the built-in ones, and its overview: the counts, the
// last change in UTC, null when nothing was written, and the root's nodes
// by their paths and kinds (M9/P1 design 3.17.3; M10/P1 design 3.12).
func TestTheOpenNotebookAnswer(t *testing.T) {
	last := time.Date(2026, 10, 17, 16, 30, 0, 0, time.FixedZone("CST", 8*3600))
	_, raw, err := openOf(t, opened{answer: app.Opened{Mode: domain.ModeLLMWiki, Agents: domain.Agents{Body: "# Rules <&>", Title: "agents"},
		HasAgents: true, Pages: 120, Attachments: 8, LastChange: &last,
		Root: []app.RootNode{{Path: "Inbox.md", Kind: domain.KindPage}, {Path: "logo.png", Kind: domain.KindAsset}}}})
	want := `{"notebook":"01920000-0000-7000-8000-0000000000b0","mode":"llm_wiki","instructions":"# Rules <&>","instructions_from":"agents.md",` +
		`"overview":{"pages":120,"attachments":8,"last_changed_at":"2026-10-17T08:30:00Z","root":[{"path":"Inbox.md","kind":"page"},` +
		`{"path":"logo.png","kind":"attachment"}]},"truncated":false}`
	if err != nil || raw != want {
		t.Errorf("answer = %s, %v;\nwant %s", raw, err, want)
	}
	a, raw, err := openOf(t, opened{})
	if err != nil || a.Instructions != NotebookInstructions || !strings.Contains(raw, `"instructions_from":null`) ||
		!strings.Contains(raw, `"last_changed_at":null,"root":[]`) || a.Truncated {
		t.Errorf("an empty notebook = %s, %v", raw, err)
	}
	if _, _, err := openOf(t, opened{err: errors.New("the database is gone")}); err == nil {
		t.Error("the use case's error was lost")
	}
}

// longRoot is n nodes at the root, pages of names of 255 bytes.
func longRoot(n int) []app.RootNode {
	out := make([]app.RootNode, n)
	for i := range out {
		out[i] = app.RootNode{Path: fmt.Sprintf("%03d", i) + strings.Repeat("n", 252) + ".md", Kind: domain.KindPage}
	}
	return out
}

// The answer is filled against what is left of the bound, the
// instructions first: an AGENTS page of 32 KiB as the handshake counts it,
// all quotes and line breaks, which a member writes escaped twice, is cut
// further to leave the root its reserve of 8,192 bytes, no more, at its
// last line break that fits, ends with its note, and the answer is within
// the bound, never internal_error; the root's nodes fill the rest,
// truncated when they do not all fit (M9/P1 design 3.17.3, review B-I1,
// S3 review C-M1).
func TestOpenNotebookFitsItsAnswer(t *testing.T) {
	body, cut := domain.AgentsBody(strings.Repeat("\"\n", 20000), 0, mcpserver.TextBytes)
	if !cut || mcpserver.TextBytes(body) > domain.MaxInstructionsBytes || mcpserver.TextBytes(body) < domain.MaxInstructionsBytes-4 {
		t.Fatalf("the AGENTS page is %d bytes, cut %v: not the handshake's 32 KiB", mcpserver.TextBytes(body), cut)
	}
	for _, title := range []string{"AGENTS", strings.Repeat("&", 255)} {
		agents := domain.Agents{Body: body, Cut: true, Title: title}
		a, raw, err := openOf(t, opened{answer: app.Opened{Agents: agents, HasAgents: true, Pages: 100, Root: longRoot(domain.MaxOverviewRoot)}})
		if err != nil {
			t.Fatalf("%.10s: %v", title, err)
		}
		skeleton := a
		skeleton.Instructions, skeleton.Overview.Root, skeleton.Truncated = "", []rootItem{}, false
		s, _ := line(skeleton)
		budget := mcpserver.MaxAnswerBytes - mcpserver.TextBytes(s) - 8192
		note := "\n\n(The rest of " + title + `.md is cut here: read it whole with read, path "` + title + `.md".)`
		kept := strings.TrimSuffix(a.Instructions, note)
		next := strings.IndexByte(body[min(len(kept)+1, len(body)):], '\n')
		if !strings.HasPrefix(body, kept) || next < 0 {
			t.Fatalf("%.10s: the instructions keep %d bytes, not a prefix of the body ended by a line break", title, len(kept))
		}
		longer := strings.TrimSpace(body[:len(kept)+1+next]) + note
		switch {
		case mcpserver.TextBytes(raw) > mcpserver.MaxAnswerBytes:
			t.Errorf("%.10s: the answer is %d bytes, over the bound", title, mcpserver.TextBytes(raw))
		case !strings.HasSuffix(a.Instructions, note):
			t.Errorf("%.10s: the instructions end %q, want the note", title, a.Instructions[max(0, len(a.Instructions)-80):])
		case mcpserver.MemberBytes(a.Instructions) > budget:
			t.Errorf("%.10s: the instructions are %d bytes as a member, over their %d", title, mcpserver.MemberBytes(a.Instructions), budget)
		case mcpserver.MemberBytes(longer) <= budget && mcpserver.TextBytes(longer) <= domain.MaxInstructionsBytes:
			t.Errorf("%.10s: the instructions are %d bytes as a member; a line more, %d, fits their %d", title,
				mcpserver.MemberBytes(a.Instructions), mcpserver.MemberBytes(longer), budget)
		case len(a.Overview.Root) < 28 || !a.Truncated:
			t.Errorf("%.10s: %d root nodes, truncated %v; want the reserve's 28 at least, truncated", title, len(a.Overview.Root), a.Truncated)
		}
	}
	var short []app.RootNode
	for i := range domain.MaxOverviewRoot {
		short = append(short, app.RootNode{Path: fmt.Sprintf("P%d.md", i), Kind: domain.KindPage})
	}
	for _, more := range []bool{false, true} {
		a, _, err := openOf(t, opened{answer: app.Opened{Root: short, MoreRoot: more}})
		if err != nil || len(a.Overview.Root) != domain.MaxOverviewRoot || a.Truncated != more || a.Instructions != NotebookInstructions {
			t.Errorf("100 short names, more %v: %d listed, truncated %v, %v", more, len(a.Overview.Root), a.Truncated, err)
		}
	}
}
