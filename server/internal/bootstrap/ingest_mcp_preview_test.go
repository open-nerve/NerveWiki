package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// previewWire decodes the actual SDK HTTP envelope. Size is measured directly
// from its original text tokens, never by re-marshalling decoded strings.
func previewWire(t *testing.T, a mcpAnswer) map[string]any {
	t.Helper()
	var wire struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Type string          `json:"type"`
				Text json.RawMessage `json:"text"`
			} `json:"content"`
			Structured json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(a.body, &wire); err != nil || a.status != http.StatusOK || wire.Result.IsError {
		t.Fatalf("SDK %s failed: status%d error%v %.500s", a.tool, a.status, err, a.body)
	}
	if len(wire.Result.Content) != 1 || wire.Result.Content[0].Type != "text" {
		t.Fatalf("SDK content=%+v, want one structured text block", wire.Result.Content)
	}
	size := 0
	for _, block := range wire.Result.Content {
		size += len(block.Text) - 2
	}
	if size > 49152 {
		t.Errorf("original SDK text tokens=%d, exceeds49152", size)
	}
	if a.took > 4*time.Second {
		t.Errorf("history call took%s, exceeds4s", a.took)
	}
	var text string
	if err := json.Unmarshal(wire.Result.Content[0].Text, &text); err != nil {
		t.Fatal(err)
	}
	var decoded, structured map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wire.Result.Structured, &structured); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, structured) {
		t.Fatal("SDK structuredContent differs from decoded text")
	}
	return structured
}

// previewBytes independently constructs the specified inner JSON and asks the
// pinned SDK TextContent encoder for its outer text token. It shares no builder,
// Fit, TextBytes or MemberBytes with production.
func previewBytes(t *testing.T, candidate any) int {
	t.Helper()
	var inner bytes.Buffer
	e := json.NewEncoder(&inner)
	e.SetEscapeHTML(false)
	if err := e.Encode(candidate); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(&mcp.TextContent{Text: strings.TrimSuffix(inner.String(), "\n")})
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(b, &object); err != nil {
		t.Fatal(err)
	}
	return len(object["text"]) - 2
}

func previewMax(t *testing.T, n int, candidate func(int) any) int {
	t.Helper()
	best := -1
	sizes := make([]int, n+1)
	for s := 0; s <= n; s++ {
		sizes[s] = previewBytes(t, candidate(s))
		if sizes[s] <= 49152 {
			best = s
		}
	}
	if best < 0 {
		t.Fatalf("no legal source prefix: B=%v", sizes)
	}
	t.Logf("independent SDK B=%v; s*=%d", sizes, best)
	return best
}

type previewSource struct {
	id         uuid.UUID
	path, name string
}

type previewCase struct {
	id, other, notebook, actor uuid.UUID
	client                     string
	at                         time.Time
	message                    *string
	sources                    []previewSource
	changes                    []any
	detailChanges              []any
}

func (p previewCase) header(s int, cut bool) map[string]any {
	sources := make([]any, s)
	for i := range s {
		if p.sources[i].path != "" {
			sources[i] = p.sources[i].path
		}
	}
	var message any
	if p.message != nil {
		message = *p.message
	}
	return map[string]any{
		"id": p.id.String(), "notebook_id": p.notebook.String(), "kind": "edit", "client": p.client, "message": message,
		"created_by": map[string]any{"user_id": p.actor.String(), "display_name": "Alice"},
		"created_at": p.at.Format(time.RFC3339Nano), "updated_at": p.at.Format(time.RFC3339Nano),
		"reverts": nil, "reverted_by": []any{}, "trimmed": false, "node_count": len(p.changes), "content_count": 0,
		"ingest_count": len(p.sources), "ingest_sources": sources, "ingest_sources_cut": cut,
	}
}

func previewCursor(t *testing.T, payload any) string {
	t.Helper()
	value, err := shared.EncodeCursor(payload)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// Every controlled leg goes through a real PostgreSQL Add and both SDK scopes.
// These are S2-1 store fixtures; production ingest is wired only in S2-2.
func TestIngestMCPPreview(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.createWith(t, tm.tokens["alice"], `{"name":"Preview","mode":"llm_wiki"}`)
	nb := uuid.MustParse(wiki.ID)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tm.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("UPDATE notebooks SET workspace_access='viewer' WHERE id=$1", nb)
	var actor uuid.UUID
	if err := tm.pool.QueryRow(ctx, "SELECT id FROM users WHERE email='alice@example.com'").Scan(&actor); err != nil {
		t.Fatal(err)
	}
	exec("UPDATE users SET display_name='Alice' WHERE id=$1", actor)
	raw := *wiki.SourceRootID
	var ampParents []string
	buildSources := func(char string, deep bool) []previewSource {
		parent, path := raw, "raw"
		if deep {
			for range 9 {
				name := strings.Repeat(char, 255)
				parent = tm.createPage(t, "alice", wiki.ID, parent, name)
				path += "/" + name
				if char == "&" {
					ampParents = append(ampParents, parent)
				}
			}
		}
		out := make([]previewSource, 25)
		for i := range out {
			name := fmt.Sprintf("Source-%02d.txt", i)
			if deep {
				name = strings.Repeat(char, 249) + fmt.Sprintf("%02d.txt", i)
			}
			asset := tm.upload(t, "alice", wiki.ID, parent, name, fmt.Sprintf("%s source content %d", char, i))
			out[i] = previewSource{id: uuid.MustParse(asset.ID), path: path + "/" + name, name: name}
			if deep && len(out[i].path) != 2563 {
				t.Fatalf("legal source path is%dB, want2563", len(out[i].path))
			}
		}
		return out
	}
	short, ascii, amp := buildSources("s", false), buildSources("a", true), buildSources("&", true)
	output := tm.createPage(t, "alice", wiki.ID, "", "Output")
	shortChange := map[string]any{"node": map[string]any{"id": output, "kind": "page", "name": "Output", "alive": true},
		"kind": "created", "before": nil, "after": map[string]any{"parent_id": nil, "parent_name": nil, "name": "Output"}, "content": nil}
	shortDetail := maps.Clone(shortChange)
	shortDetail["node"] = map[string]any{"id": output, "kind": "page", "name": "Output", "alive": true, "path": "Output.md"}
	cases := map[string]previewCase{}
	clock := time.Now().UTC().Truncate(time.Microsecond).Add(time.Hour)
	for _, tool := range []string{"log", "detail"} {
		for _, mode := range []string{"zero", "short", "source", "prefix", "changes", "both"} {
			p := previewCase{id: uuid.NewV7(), notebook: nb, actor: actor, at: clock, client: "mcp:ingest-" + tool + "-" + mode,
				changes: []any{shortChange}, detailChanges: []any{shortDetail}}
			clock = clock.Add(time.Second)
			if mode != "zero" {
				p.sources = slices.Clone(short)
			}
			if mode == "source" || mode == "prefix" || mode == "both" {
				p.sources = slices.Clone(ascii)
				if tool == "log" {
					p.sources = slices.Clone(amp)
					p.message = new(strings.Repeat("\x01", 4096))
				}
			}
			if mode == "prefix" {
				pos := 19
				if tool == "log" {
					pos = 2
				}
				p.sources[pos] = previewSource{id: uuid.NewV7()} // a purged source's persistent position
			}
			longChanges := mode == "changes" || mode == "both" || (tool == "detail" && (mode == "source" || mode == "prefix"))
			if longChanges {
				if mode == "changes" || mode == "both" {
					p.message = new(strings.Repeat("\x01", 4096))
				}
				p.changes, p.detailChanges = []any{}, []any{}
				for i := range 3 {
					src := amp[i]
					before := map[string]any{"parent_id": ampParents[7], "parent_name": strings.Repeat("&", 255), "name": src.name}
					after := map[string]any{"parent_id": ampParents[8], "parent_name": strings.Repeat("&", 255), "name": src.name}
					c := map[string]any{"node": map[string]any{"id": src.id.String(), "kind": "attachment", "name": src.name, "alive": true},
						"kind": "moved", "before": before, "after": after, "content": nil}
					d := maps.Clone(c)
					d["node"] = map[string]any{"id": src.id.String(), "kind": "attachment", "name": src.name, "alive": true, "path": src.path}
					p.changes, p.detailChanges = append(p.changes, c), append(p.detailChanges, d)
				}
			}
			seed := func(id uuid.UUID, at time.Time) {
				exec(`INSERT INTO changesets(id,notebook_id,kind,client,message,created_by_id,created_at,updated_at)
					VALUES($1,$2,'edit',$3,$4,$5,$6,$6)`, id, nb, p.client, p.message, actor, at)
				for _, c := range p.changes {
					row := c.(map[string]any)
					node := row["node"].(map[string]any)
					after := row["after"].(map[string]any)
					var beforeParent, beforeName any
					if before, ok := row["before"].(map[string]any); ok {
						beforeParent, beforeName = before["parent_id"], before["name"]
					}
					exec(`INSERT INTO changeset_items(id,changeset_id,node_id,before_parent_id,before_name,before_sort_order,
						after_parent_id,after_name,after_sort_order,created_at,updated_at)
						VALUES(gen_random_uuid(),$1,$2,$3,$4,CASE WHEN $4::text IS NULL THEN NULL ELSE 0 END,$5,$6,0,$7,$7)`,
						id, node["id"], beforeParent, beforeName, after["parent_id"], after["name"], at)
				}
				marks := page.NewIngestMarks(tm.pool)
				for i, source := range p.sources {
					if added, err := marks.Add(ctx, nb, id, []uuid.UUID{source.id}); err != nil || added.Added != 1 || added.Count != i+1 {
						t.Fatalf("real Add %s seq%d=%+v,%v", id, i+1, added, err)
					}
				}
				if n := count(t, tm.pool, "SELECT ingest_count FROM changesets WHERE id=$1", id); n != len(p.sources) {
					t.Fatalf("persisted count=%d, want%d", n, len(p.sources))
				}
				want := make([]string, len(p.sources))
				for i, source := range p.sources {
					want[i] = fmt.Sprintf("%d:%s", i+1, source.id)
				}
				got := queryStrings(t, tm.pool, "SELECT seq::text||':'||node_id::text FROM changeset_sources WHERE changeset_id=$1 ORDER BY seq", id)
				if !slices.Equal(got, want) {
					t.Fatalf("persistent seq differs: got%v want%v", got, want)
				}
			}
			seed(p.id, p.at)
			if tool == "log" {
				p.other = uuid.NewV7()
				seed(p.other, p.at.Add(-time.Microsecond))
			}
			cases[tool+"_"+mode] = p
		}
	}
	reader := tm.agent(t, "bob", wiki.ID)
	for _, scope := range []string{"notebook", "account"} {
		t.Run(scope, func(t *testing.T) {
			call := func(t *testing.T, tool string, args map[string]any) map[string]any {
				t.Helper()
				if scope == "account" {
					args = maps.Clone(args)
					args["notebook"] = wiki.ID
					return previewWire(t, reader.call(t, mcpserver.Pattern, tool, args))
				}
				return previewWire(t, reader.use(t, tool, args))
			}
			log := func(t *testing.T, mode string, limit int, second bool) map[string]any {
				t.Helper()
				p := cases["log_"+mode]
				firstCursor := previewCursor(t, domain.ChangesetCursor{At: p.at, ID: p.id})
				args := map[string]any{"client": p.client, "limit": limit}
				next := any(firstCursor)
				if second {
					args["cursor"] = firstCursor
					p.id, p.at, next = p.other, p.at.Add(-time.Microsecond), nil
				}
				m := min(3, len(p.sources))
				build := func(s, j int) any {
					row := p.header(s, s < m)
					row["changes"] = p.changes[:j]
					if j < len(p.changes) {
						row["changes_cut"] = true
					}
					return map[string]any{"changesets": []any{row}, "next_cursor": next}
				}
				s, j := m, len(p.changes)
				if previewBytes(t, build(s, j)) > 49152 {
					s = previewMax(t, m, func(s int) any { return build(s, 0) })
					j = previewMax(t, len(p.changes), func(j int) any { return build(s, j) })
				}
				if mode == "source" && (s < 1 || s >= m) {
					t.Fatalf("log maximum prefix%d, want 1<=s<%d", s, m)
				}
				got := call(t, "log", args)
				sameJSON(t, "literal log envelope", got, normalizePreview(t, build(s, j)))
				return got
			}
			detail := func(t *testing.T, mode string) map[string]any {
				t.Helper()
				p := cases["detail_"+mode]
				m := min(20, len(p.sources))
				start := previewCursor(t, domain.ChangeCursor{})
				build := func(s, j int) any {
					h := p.header(s, s < m)
					h["renames"], h["moves"] = false, false
					var next any
					switch {
					case j == 0 && len(p.detailChanges) > 0:
						next = start
					case j < len(p.detailChanges):
						node := p.detailChanges[j-1].(map[string]any)["node"].(map[string]any)["id"].(string)
						next = previewCursor(t, domain.ChangeCursor{After: uuid.MustParse(node)})
					}
					return map[string]any{"changeset": h, "changes": p.detailChanges[:j], "next_cursor": next}
				}
				s := previewMax(t, m, func(s int) any { return build(s, 0) })
				j := previewMax(t, len(p.detailChanges), func(j int) any { return build(s, j) })
				if mode == "source" && (s < 2 || s >= m) {
					t.Fatalf("detail maximum prefix%d, want 2<=s<%d", s, m)
				}
				got := call(t, "changeset", map[string]any{"id": p.id.String()})
				sameJSON(t, "literal detail envelope", got, normalizePreview(t, build(s, j)))
				return got
			}
			continuation := func(t *testing.T) {
				t.Helper()
				p := cases["detail_source"]
				first := detail(t, "source")
				if len(first["changes"].([]any)) != 0 || first["next_cursor"] == nil {
					t.Fatal("detail fixture must answer header only and real Start")
				}
				var all []any
				next := first["next_cursor"]
				for turn := 0; next != nil && turn < 10; turn++ {
					got := call(t, "changeset", map[string]any{"id": p.id.String(), "cursor": next, "limit": 1})
					if len(got) != 3 || got["changeset"] != nil {
						t.Errorf("continuation header/outer fields=%v, want three members and null header", got)
					}
					changes := got["changes"].([]any)
					if len(changes) != 1 {
						t.Fatalf("continuation has%d changes, want one", len(changes))
					}
					all = append(all, changes...)
					wantNext := any(nil)
					if len(all) < len(p.detailChanges) {
						node := changes[0].(map[string]any)["node"].(map[string]any)["id"].(string)
						wantNext = previewCursor(t, domain.ChangeCursor{After: uuid.MustParse(node)})
					}
					sameJSON(t, "continuation cursor", got["next_cursor"], wantNext)
					next = got["next_cursor"]
				}
				sameJSON(t, "all changes, no gaps or duplicates", all, normalizePreview(t, p.detailChanges))
				if next != nil {
					t.Fatal("continuation never ended")
				}
			}
			for _, leg := range []struct{ name, tool, mode string }{
				{"log_required_false", "log", "short"}, {"detail_required_false", "detail", "short"},
				{"log_count_not_cut", "log", "short"}, {"detail_count_not_cut", "detail", "short"},
				{"log_source_fit", "log", "source"}, {"detail_source_fit", "detail", "source"},
				{"log_overcut", "log", "source"}, {"detail_overcut", "detail", "source"},
				{"log_independent_cuts_01", "log", "changes"}, {"log_independent_cuts_10", "log", "source"},
				{"log_prefix_no_skip", "log", "prefix"}, {"detail_prefix_no_skip", "detail", "prefix"},
				{"log_sources_before_changes", "log", "changes"},
			} {
				t.Run(leg.name, func(t *testing.T) {
					if leg.tool == "log" {
						log(t, leg.mode, 1, false)
						if strings.HasSuffix(leg.name, "required_false") {
							log(t, "zero", 1, false)
						}
					} else {
						detail(t, leg.mode)
						if strings.HasSuffix(leg.name, "required_false") {
							detail(t, "zero")
						}
					}
				})
			}
			t.Run("detail_header_only_cursor", continuation)
			t.Run("detail_continuation_null", continuation)
			t.Run("log_many_cursor", func(t *testing.T) {
				log(t, "source", 2, false)
				log(t, "source", 2, true)
			})
			t.Run("log_one_more_cursor", func(t *testing.T) {
				log(t, "source", 1, false)
				log(t, "source", 1, true)
			})
			t.Run("positive_both_cut", func(t *testing.T) {
				log(t, "both", 1, false)
				detail(t, "both")
			})
		})
	}
}

func normalizePreview(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
