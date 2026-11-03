package bootstrap

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// Store fixtures deliberately exercise the already public marks port. S2-2
// alone will claim automatic producer detection.
func TestIngestHistoryProtocols(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.createWith(t, tm.tokens["alice"], `{"name":"Ingest protocol","mode":"llm_wiki"}`)
	nb := uuid.MustParse(wiki.ID)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tm.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("UPDATE notebooks SET workspace_access='viewer' WHERE id=$1", nb)
	var alice, bob uuid.UUID
	if err := tm.pool.QueryRow(ctx, "SELECT id FROM users WHERE email='alice@example.com'").Scan(&alice); err != nil {
		t.Fatal(err)
	}
	if err := tm.pool.QueryRow(ctx, "SELECT id FROM users WHERE email='bob@example.com'").Scan(&bob); err != nil {
		t.Fatal(err)
	}
	output := tm.createPage(t, "alice", wiki.ID, "", "Output")
	sources := make([]uuid.UUID, 25)
	wantIDs, wantPaths := make([]any, 25), make([]any, 25)
	for i := range sources {
		sources[i] = uuid.MustParse(fmt.Sprintf("00000000-0000-7000-8000-%012x", 100-i))
		name, kind := fmt.Sprintf("Source-%02d.txt", i+1), "asset"
		if i == 1 {
			name, kind = "Source-02", "page"
		}
		exec(`INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at)
   VALUES($1,$2,$3,$4,$5,lower($5),$6,$7,$7,now(),now())`, sources[i], nb, *wiki.SourceRootID, kind, name, i, alice)
		wantIDs[i], wantPaths[i] = sources[i].String(), "raw/"+name
		if kind == "page" {
			wantPaths[i] = "raw/" + name + ".md"
		}
	}
	marked, other, ordinary := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	at := time.Now().UTC().Truncate(time.Microsecond).Add(time.Hour)
	for i, cs := range []uuid.UUID{marked, other, ordinary} {
		actor, kind := alice, "edit"
		if cs == other {
			actor = bob
		}
		if cs == ordinary {
			kind = "import"
		}
		exec(`INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at)
   VALUES($1,$2,$3,'mcp:history-fixture',$4,$5,$5)`, cs, nb, kind, actor, at.Add(time.Duration(i)*time.Second))
		exec(`INSERT INTO changeset_items(id,changeset_id,node_id,after_name,after_sort_order,created_at,updated_at)
   VALUES(uuidv7(),$1,$2,'Output',0,now(),now())`, cs, output)
	}
	marks := page.NewIngestMarks(tm.pool)
	for i, id := range sources {
		if got, err := marks.Add(ctx, nb, marked, []uuid.UUID{id}); err != nil || got.Count != i+1 {
			t.Fatalf("Add seq%d=%+v,%v", i+1, got, err)
		}
	}
	if got, err := marks.Add(ctx, nb, other, sources[:1]); err != nil || got.Count != 1 {
		t.Fatalf("other Add=%+v,%v", got, err)
	}
	// One soft deleted source and one physically deleted source: neither mark
	// disappears and neither preview may substitute the fourth source.
	exec("UPDATE nodes SET deleted_at=now() WHERE id=$1", sources[0])
	exec("DELETE FROM nodes WHERE id=$1", sources[2])
	wantIDs[0], wantIDs[2], wantPaths[0], wantPaths[2] = nil, nil, nil, nil
	check := func(t *testing.T, row map[string]any, n int, paths bool) {
		t.Helper()
		want := wantIDs[:n]
		if paths {
			want = wantPaths[:n]
		}
		if row["id"] != marked.String() || row["kind"] != "edit" || row["ingest_count"] != float64(25) || !reflect.DeepEqual(row["ingest_sources"], want) {
			t.Errorf("count/seq preview=%v; want edit %s count25 sources%v", row, marked, want)
		}
		if paths {
			if cut, ok := row["ingest_sources_cut"]; !ok || cut != false {
				t.Errorf("required complete-preview cut=%v present%v", cut, ok)
			}
		}
	}
	idsOf := func(v any) []string {
		rows, ok := v.([]any)
		if !ok {
			t.Fatalf("missing array: %T %v", v, v)
		}
		ids := make([]string, len(rows))
		for i, row := range rows {
			ids[i] = row.(map[string]any)["id"].(string)
		}
		return ids
	}
	t.Run("REST_reader", func(t *testing.T) {
		check(t, tm.restOf(t, "bob", "/api/v0/changesets/"+marked.String()), 20, false)
		for _, query := range []string{"", "&ingest=false", "&ingest=true"} {
			rows := tm.restOf(t, "bob", "/api/v0/notebooks/"+wiki.ID+"/changesets?client=mcp:history-fixture"+query)["data"]
			want := []string{ordinary.String(), other.String(), marked.String()}
			if query == "&ingest=true" {
				want = want[1:]
			}
			if got := idsOf(rows); !slices.Equal(got, want) {
				t.Errorf("%s IDs%v want%v", query, got, want)
			}
			list := rows.([]any)
			check(t, list[len(list)-1].(map[string]any), 3, false)
		}
		for _, query := range []string{"&actor=" + alice.String(), "&page_id=" + output + "&actor=" + alice.String()} {
			got := tm.restOf(t, "bob", "/api/v0/notebooks/"+wiki.ID+"/changesets?client=mcp:history-fixture&ingest=true"+query)["data"]
			if ids := idsOf(got); !slices.Equal(ids, []string{marked.String()}) {
				t.Errorf("intersect%s=%v", query, ids)
			}
		}
		got := tm.restOf(t, "bob", "/api/v0/notebooks/"+wiki.ID+"/changesets?ingest=true&page_id="+sources[1].String())["data"]
		if ids := idsOf(got); len(ids) != 0 {
			t.Errorf("source membership is not a changed-page relation: %v", ids)
		}
	})
	reader := tm.agent(t, "bob", wiki.ID)
	for _, scope := range []string{"notebook", "account"} {
		t.Run(scope, func(t *testing.T) {
			call := func(tool string, args map[string]any) mcpAnswer {
				if scope == "account" {
					args = maps.Clone(args)
					args["notebook"] = wiki.ID
					return reader.call(t, mcpserver.Pattern, tool, args)
				}
				return reader.use(t, tool, args)
			}
			check(t, previewWire(t, call("changeset", map[string]any{"id": marked.String()}))["changeset"].(map[string]any), 20, true)
			for _, ingest := range []any{nil, false, true} {
				args := map[string]any{"client": "mcp:history-fixture"}
				if ingest != nil {
					args["ingest"] = ingest
				}
				rows := previewWire(t, call("log", args))["changesets"]
				want := []string{ordinary.String(), other.String(), marked.String()}
				if ingest == true {
					want = want[1:]
				}
				if ids := idsOf(rows); !slices.Equal(ids, want) {
					t.Errorf("ingest%v IDs%v want%v", ingest, ids, want)
				}
				list := rows.([]any)
				check(t, list[len(list)-1].(map[string]any), 3, true)
			}
			for _, args := range []map[string]any{
				{"ingest": true, "actor": alice.String(), "client": "mcp:history-fixture"},
				{"ingest": true, "actor": alice.String(), "client": "mcp:history-fixture", "path": "Output.md"},
			} {
				if ids := idsOf(previewWire(t, call("log", args))["changesets"]); !slices.Equal(ids, []string{marked.String()}) {
					t.Errorf("intersection %v=%v", args, ids)
				}
			}
			if ids := idsOf(previewWire(t, call("log", map[string]any{"ingest": true, "path": "raw/Source-02.md"}))["changesets"]); len(ids) != 0 {
				t.Errorf("source became changed page: %v", ids)
			}
			if r := refusalOf(t, call("log", map[string]any{"ingest": "true"})); r.Code != "bad_request" {
				t.Errorf("nonboolean ingest=%+v", r)
			}
		})
	}
	t.Run("hidden_and_cross_notebook", func(t *testing.T) {
		hidden := tm.openNotebook(t, "alice", "Hidden")
		exec("UPDATE notebooks SET workspace_access='none' WHERE id=$1", hidden)
		hiddenCS := uuid.NewV7()
		exec(`INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at)
   VALUES($1,$2,'edit','api',$3,now(),now())`, hiddenCS, hidden, alice)
		if got, err := marks.Add(ctx, uuid.MustParse(hidden), hiddenCS, sources[:1]); err != nil || got.Count != 1 {
			t.Fatalf("hidden Add=%+v,%v", got, err)
		}
		for _, path := range []string{"/api/v0/notebooks/" + hidden + "/changesets?ingest=true", "/api/v0/changesets/" + hiddenCS.String()} {
			status, body := ask(t, tm.contract, http.MethodGet, tm.base+path, tm.tokens["bob"], "")
			want := "page.changeset_not_found"
			if path[:len("/api/v0/notebooks/")] == "/api/v0/notebooks/" {
				want = "notebook.not_found"
			}
			if status != 404 || problemCode(t, body) != want {
				t.Errorf("hidden REST %s=%d %s", path, status, body)
			}
		}
		for _, scope := range []string{"notebook", "account"} {
			for _, tool := range []string{"log", "changeset"} {
				args := map[string]any{"ingest": true}
				if tool == "changeset" {
					args = map[string]any{"id": hiddenCS.String()}
				}
				endpoint := atNotebook(hidden)
				if scope == "account" {
					endpoint = mcpserver.Pattern
					args["notebook"] = hidden
				}
				a := reader.call(t, endpoint, tool, args)
				code := ""
				if scope == "notebook" {
					code = a.problem(t, http.StatusNotFound)
				} else {
					code = refusalOf(t, a).Code
				}
				if code != "notebook.not_found" {
					t.Errorf("hidden %s/%s=%s", scope, tool, code)
				}
			}
			args := map[string]any{"id": hiddenCS.String()}
			endpoint := atNotebook(wiki.ID)
			if scope == "account" {
				endpoint = mcpserver.Pattern
				args["notebook"] = wiki.ID
			}
			if r := refusalOf(t, reader.call(t, endpoint, "changeset", args)); r.Code != "page.changeset_not_found" {
				t.Errorf("foreign changeset %s=%+v", scope, r)
			}
		}
	})
	if n := count(t, tm.pool, "SELECT ingest_count FROM changesets WHERE id=$1", marked); n != 25 {
		t.Errorf("read changed persistent count=%d", n)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1", marked); n != 25 {
		t.Errorf("read changed source rows=%d", n)
	}
}
