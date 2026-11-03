package mcpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// This adapter oracle owns a literal text envelope and its JSON string token.
// It does not use adapter builders or production measuring/fitting functions.
// The real SDK outer encoding is independently exercised in bootstrap.
func ingestEnvelopeBytes(t *testing.T, v any) int {
	t.Helper()
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		t.Fatal(err)
	}
	inner := strings.TrimSuffix(b.String(), "\n")
	outer, err := json.Marshal(map[string]any{"type": "text", "text": inner})
	if err != nil {
		t.Fatal(err)
	}
	var token map[string]json.RawMessage
	if err := json.Unmarshal(outer, &token); err != nil {
		t.Fatal(err)
	}
	return len(token["text"]) - 2
}

func ingestLiteralHeader(l app.LoggedChangeset, sources []any, cut bool) map[string]any {
	c := l.Changeset
	user := func(id uuid.UUID, name string) any {
		return map[string]any{"user_id": id.String(), "display_name": name}
	}
	var reverts any
	if r := c.Reverts; r != nil {
		reverts = map[string]any{"id": r.ID.String(), "kind": string(r.Kind), "created_by": user(r.By, r.ByName),
			"created_at": r.CreatedAt.UTC().Format(time.RFC3339Nano)}
	}
	reverted := make([]any, len(l.Facts.RevertedBy))
	for i, id := range l.Facts.RevertedBy {
		reverted[i] = id.String()
	}
	var message any
	if c.Message != nil {
		message = *c.Message
	}
	return map[string]any{"id": c.ID.String(), "notebook_id": c.NotebookID.String(), "kind": string(c.Kind),
		"client": string(c.Client), "message": message, "created_by": user(c.By, c.ByName),
		"created_at": c.CreatedAt.UTC().Format(time.RFC3339Nano), "updated_at": c.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"reverts": reverts, "reverted_by": reverted, "trimmed": c.Trimmed, "node_count": l.Facts.Nodes, "content_count": l.Facts.Contents,
		"ingest_count": c.IngestCount, "ingest_sources": sources, "ingest_sources_cut": cut}
}

func ingestLiteralChanges(changes []domain.ChangeRow, paths map[uuid.UUID]string, withPath bool) []any {
	out := make([]any, len(changes))
	place := func(p *domain.ChangePlace) any {
		if p == nil {
			return nil
		}
		var parent, name any
		if p.ParentID != nil {
			parent = p.ParentID.String()
		}
		if p.ParentName != nil {
			name = *p.ParentName
		}
		return map[string]any{"parent_id": parent, "parent_name": name, "name": p.Name}
	}
	for i, c := range changes {
		kind := string(c.NodeKind)
		if kind == "asset" {
			kind = "attachment"
		}
		node := map[string]any{"id": c.NodeID.String(), "kind": kind, "name": c.Name, "alive": c.Alive}
		if withPath {
			node["path"] = nil
			if path, ok := paths[c.NodeID]; ok {
				node["path"] = path
			}
		}
		var content any
		if c.Content != nil {
			var base any
			if c.Content.Base != nil {
				base = *c.Content.Base
			}
			content = map[string]any{"base_revision": base, "revision": c.Content.Revision}
		}
		// Fixtures below use either a creation or a move with content.
		changeKind := "created"
		if c.Before != nil {
			changeKind = "moved"
		}
		out[i] = map[string]any{"node": node, "kind": changeKind, "before": place(c.Before), "after": place(c.After), "content": content}
	}
	return out
}

func ingestMaxPrefix(t *testing.T, n int, build func(int) any) int {
	t.Helper()
	best := -1
	sizes := make([]int, n+1)
	for i := 0; i <= n; i++ {
		sizes[i] = ingestEnvelopeBytes(t, build(i))
		if sizes[i] <= 49152 {
			best = i
		}
	}
	if best < 0 {
		t.Fatalf("fixture has no fitting prefix: %v", sizes)
	}
	if best < n && sizes[best+1] <= 49152 {
		t.Fatalf("oracle's adjacent prefix fits: %v", sizes)
	}
	t.Logf("independent literal envelope candidates=%v, maximal prefix=%d", sizes, best)
	return best
}

func ingestSameWire(t *testing.T, got mcpserver.Result, want any) {
	t.Helper()
	raw := rawOf(t, got)
	var actual, expected any
	if err := json.Unmarshal([]byte(raw), &actual); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("wire differs: got %.1600s; want %.1600s", raw, b)
	}
	if size := ingestEnvelopeBytes(t, actual); size > 49152 {
		t.Errorf("literal text token=%d, exceeds49152", size)
	}
}

func TestIngestHistory(t *testing.T) {
	for _, kind := range []string{"log", "detail"} {
		for _, mode := range []string{"zero", "count_not_cut", "source_fit", "prefix_no_skip", "changes_only", "both_cut"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				l := app.LoggedChangeset{Changeset: domain.Changeset{ID: uuid.NewV7(), NotebookID: uuid.NewV7(),
					Kind: domain.ChangesetEdit, Client: domain.ClientAPI, By: uuid.NewV7(), ByName: "Alice",
					CreatedAt: worst().at, UpdatedAt: worst().at}}
				m := 3
				if kind == "detail" {
					m = 20
				}
				if mode == "zero" {
					m = 0
				} else {
					l.Changeset.IngestCount = 25
				}
				prefix := make([]any, m)
				paths := map[uuid.UUID]string{}
				for i := range m {
					id := uuid.NewV7()
					path := fmt.Sprintf("Source-%02d.md", i)
					if mode == "source_fit" || mode == "prefix_no_skip" || mode == "both_cut" {
						char := "a"
						if kind == "log" {
							char = "&"
						}
						path = strings.Repeat(strings.Repeat(char, 255)+"/", 9) + strings.Repeat(char, 250) + fmt.Sprintf("%02d.md", i)
					}
					prefix[i] = path
					l.IngestSources = append(l.IngestSources, &id)
					paths[id] = path
				}
				if mode == "prefix_no_skip" {
					// A short/null position after the first overflowing long one may
					// not replace that long position.
					prefix[m-1] = nil
					l.IngestSources[m-1] = nil
				}
				if kind == "log" && (mode == "source_fit" || mode == "prefix_no_skip" || mode == "both_cut") {
					l.Changeset.Message = new(strings.Repeat("\x01", 4096))
				}
				changes := []domain.ChangeRow{{NodeID: uuid.NewV7(), NodeKind: domain.KindPage, Name: "Output", Alive: true,
					After: &domain.ChangePlace{Name: "Output"}}}
				if mode == "changes_only" || mode == "both_cut" {
					l.Changeset.Message = new(worst().message)
					l.Changeset.ByName = worst().name
					l.Changeset.Client = worst().client
					l.Facts.RevertedBy = make([]uuid.UUID, 100)
					changes = []domain.ChangeRow{worstChange(), worstChange(), worstChange()}
				}
				l.Changes = changes
				for _, c := range changes {
					paths[c.NodeID] = worst().path
				}
				if kind == "log" {
					next := "after-first"
					literalChanges := ingestLiteralChanges(changes, paths, false)
					build := func(s, j int) any {
						row := ingestLiteralHeader(l, prefix[:s], s < m)
						row["changes"] = literalChanges[:j]
						if j < len(changes) {
							row["changes_cut"] = true
						}
						return map[string]any{"changesets": []any{row}, "next_cursor": next}
					}
					s, j := m, len(changes)
					if ingestEnvelopeBytes(t, build(s, j)) > 49152 {
						s = ingestMaxPrefix(t, m, func(s int) any { return build(s, 0) })
						j = ingestMaxPrefix(t, len(changes), func(j int) any { return build(s, j) })
					}
					if mode == "source_fit" && (s < 1 || s == m) {
						t.Fatalf("log source fixture s=%d, want nonempty strict prefix", s)
					}
					tool := tools{uc: UseCases{Log: logFunc(func(uuid.UUID, app.PathLogQuery) (app.PathLog, error) {
						return app.PathLog{Changesets: []app.LoggedChangeset{l}, Paths: paths, Cursors: []string{next}, NextCursor: next}, nil
					})}}
					res, err := tool.log(context.Background(), mcpserver.Call{}, logIn{})
					if err != nil {
						t.Fatalf("log failed instead of fitting: %v", err)
					}
					ingestSameWire(t, res, build(s, j))
				} else {
					cursors := []string{"after-one", "after-two", "after-end"}
					start := "real-start"
					literalChanges := ingestLiteralChanges(changes, paths, true)
					build := func(s, j int) any {
						header := ingestLiteralHeader(l, prefix[:s], s < m)
						header["renames"], header["moves"] = false, false
						var next any
						switch {
						case j == 0:
							next = start
						case j < len(changes):
							next = cursors[j-1]
						}
						return map[string]any{"changeset": header, "changes": literalChanges[:j], "next_cursor": next}
					}
					s := ingestMaxPrefix(t, m, func(s int) any { return build(s, 0) })
					j := ingestMaxPrefix(t, len(changes), func(j int) any { return build(s, j) })
					if mode == "source_fit" && (s < 2 || s == m) {
						t.Fatalf("detail source fixture s=%d, want >=2 strict prefix", s)
					}
					tool := tools{uc: UseCases{Changeset: changesetFunc(func(uuid.UUID, uuid.UUID, *int, *string) (app.ChangesetPage, error) {
						return app.ChangesetPage{Detail: &app.ChangesetDetail{LoggedChangeset: l}, Changes: changes,
							Paths: paths, Cursors: cursors, Start: start}, nil
					})}}
					res, err := tool.changeset(context.Background(), mcpserver.Call{}, changesetIn{ID: l.Changeset.ID.String()})
					if err != nil {
						t.Fatalf("detail failed instead of fitting: %v", err)
					}
					ingestSameWire(t, res, build(s, j))
				}
			})
		}
	}
}
