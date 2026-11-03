package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// Missing required zero values must fail even though a typed decoder supplies
// the same Go zero values for absent members.
func TestIngestHistory(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Ingest history")
	if _, err := tm.pool.Exec(context.Background(), "UPDATE notebooks SET workspace_access = 'viewer' WHERE id = $1", nb); err != nil {
		t.Fatal(err)
	}
	node := tm.createPage(t, "alice", nb, "", "Ordinary")
	var cs string
	if err := tm.pool.QueryRow(context.Background(),
		"SELECT changeset_id::text FROM page_revisions WHERE node_id = $1", node).Scan(&cs); err != nil {
		t.Fatal(err)
	}
	read := func(t *testing.T, path string) map[string]json.RawMessage {
		t.Helper()
		status, body := ask(t, tm.contract, http.MethodGet, tm.base+path, tm.tokens["bob"], "")
		if status != http.StatusOK {
			t.Fatalf("history read = %d %s", status, body)
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	t.Run("zero_fields", func(t *testing.T) {
		list := read(t, "/api/v0/notebooks/"+nb+"/changesets")
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(list["data"], &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("ordinary history has %d rows, want one", len(rows))
		}
		for _, row := range []map[string]json.RawMessage{rows[0], read(t, "/api/v0/changesets/"+cs)} {
			var id string
			if err := json.Unmarshal(row["id"], &id); err != nil || id != cs {
				t.Fatalf("history id = %q, %v; want %s", id, err, cs)
			}
			for key, want := range map[string]string{"ingest_count": "0", "ingest_sources": "[]"} {
				raw, present := row[key]
				if !present || string(raw) != want {
					t.Errorf("required %s = %s (present=%v), want %s", key, raw, present, want)
				}
			}
		}
	})
	t.Run("empty_filter", func(t *testing.T) {
		for _, query := range []string{"", "?ingest=false", "?ingest=true"} {
			got := read(t, "/api/v0/notebooks/"+nb+"/changesets"+query)
			var rows []map[string]json.RawMessage
			if err := json.Unmarshal(got["data"], &rows); err != nil {
				t.Fatal(err)
			}
			want := 1
			if query == "?ingest=true" {
				want = 0
			}
			if len(rows) != want {
				t.Errorf("%s returns %d changesets, want %d", query, len(rows), want)
			}
			if want == 1 && len(rows) == 1 && string(rows[0]["id"]) != "\""+cs+"\"" {
				t.Errorf("%s skipped the ordinary edit: %s", query, rows[0]["id"])
			}
		}
		status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+nb+"/changesets?ingest=not-a-bool", tm.tokens["bob"], "")
		if status != http.StatusBadRequest || problemCode(t, body) != "bad_request" {
			t.Errorf("invalid ingest = %d %s, want bad_request", status, body)
		}
	})
}
