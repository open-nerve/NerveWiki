package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// A request's record: its times in order, status, problem code and bytes,
// its line written; the bearer token sent (design 3.4.4).
func TestARequestIsRecorded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("authorization %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path == "/busy" {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"status":503,"code":"server_busy","detail":"later"}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()
	var lines bytes.Buffer
	log := NewLog(&lines)
	c := New(srv.URL, "tok", 3, log)
	var out struct{ OK bool }
	rec, err := c.JSON(context.Background(), "fine", http.MethodPost, "/fine", map[string]int{"a": 1}, &out, http.StatusOK)
	if err != nil || !out.OK || rec.Status != 200 || rec.Bytes != 11 || rec.Credential != 3 || rec.Op != "fine" ||
		rec.Sent == 0 || rec.Header < rec.Sent || rec.Done < rec.Header {
		t.Errorf("fine: %+v, %v", rec, err)
	}
	rec, err = c.JSON(context.Background(), "busy", http.MethodGet, "/busy", nil, nil, http.StatusOK)
	var se *StatusError
	if !errors.As(err, &se) || se.Status != 503 || se.Code != "server_busy" || rec.Code != "server_busy" {
		t.Errorf("busy: %+v, %v", rec, err)
	}
	got := strings.Split(strings.TrimSpace(lines.String()), "\n")
	var first Record
	if len(got) != 2 || json.Unmarshal([]byte(got[0]), &first) != nil || first.Op != "fine" || len(log.Records()) != 2 {
		t.Errorf("lines %q", got)
	}
}

// MCP's call goes to the notebook's entry with the SDK's headers; its
// values are counted as bodyshape counts them (design 3.4.5).
func TestAnMCPCallAndItsValues(t *testing.T) {
	var got *http.Request
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, body = r, must(io.ReadAll(r.Body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":7,"result":{}}`)
	}))
	defer srv.Close()
	call := Call("write", map[string]string{"path": "a.md", "content": "x"}, `"m0":1,"m1":[[],[1]]`, 3)
	if _, _, err := New(srv.URL, "tok", 0, nil).MCP(context.Background(), "mcp", "nb-1", call); err != nil {
		t.Fatal(err)
	}
	if got.URL.Path != "/mcp" || got.URL.Query().Get("notebook") != "nb-1" || got.Header.Get("Content-Type") != "application/json" ||
		got.Header.Get("Accept") != "application/json, text/event-stream" || got.Header.Get("Mcp-Protocol-Version") != MCPVersion ||
		got.Header.Get("Authorization") != "Bearer tok" || !bytes.Equal(body, call) {
		t.Errorf("request %v %v", got.URL, got.Header)
	}
	if !json.Valid(call) || !bytes.HasSuffix(call, []byte("   }}}")) {
		t.Errorf("call %s", call)
	}
	for _, c := range []struct {
		body string
		want int
	}{
		{`1`, 1},
		{`{}`, 1},
		{`{"a":1,"b":"x"}`, 3},
		{`{"a":{"b":{"c":[]}}}`, 4},
		{`[[],[],[1,true,null,"s"]]`, 8},
		{`{"a":[{"b":1},{"c":{"d":"e"}}],"f":false}`, 8},
		{string(call), 15}, // message, jsonrpc, id, method, params, name, arguments, path, content, _meta; 1; [[],[1]] 4
	} {
		if n, err := CountValues([]byte(c.body)); err != nil || n != c.want {
			t.Errorf("%s: %d values, %v; want %d", c.body, n, err, c.want)
		}
	}
	if _, err := CountValues([]byte(`{"a":[1,2}`)); err == nil {
		t.Error("an invalid body counted")
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// psql runs in the container by docker exec, quiet and stopping at an
// error, and its unaligned rows are read (design 3.4).
func TestPSQLGoesThroughDockerExec(t *testing.T) {
	var args [][]string
	p := PSQL{Container: "c0ffee", User: "nervewiki", Database: "nervewiki", Run: func(_ context.Context, name string, a ...string) ([]byte, error) {
		args = append(args, append([]string{name}, a...))
		switch sql := a[len(a)-1]; {
		case strings.HasPrefix(sql, "SELECT (SELECT count(*) FROM page_search_queue)"):
			// The quiet is the search queue's rows and River's jobs running
			// or available, both (design 3.4.5's ops, review B's M6).
			if sql != "SELECT (SELECT count(*) FROM page_search_queue) + "+
				"(SELECT count(*) FROM river_job WHERE state IN ('running', 'available'))" {
				return nil, fmt.Errorf("the quiet read as %q", sql)
			}
			return []byte("0\n"), nil
		default:
			return []byte("a|1\nb|2\n"), nil
		}
	}}
	rows, err := p.Query(context.Background(), "SELECT x, y FROM t")
	if err != nil || !slices.EqualFunc(rows, [][]string{{"a", "1"}, {"b", "2"}}, slices.Equal) {
		t.Errorf("rows %v, %v", rows, err)
	}
	want := []string{"docker", "exec", "c0ffee", "psql", "-XAt", "-v", "ON_ERROR_STOP=1", "-U", "nervewiki", "-d", "nervewiki", "-c",
		"SELECT x, y FROM t"}
	if !slices.Equal(args[0], want) {
		t.Errorf("ran %q, want %q", args[0], want)
	}
	if quiet, err := p.Quiet(context.Background()); err != nil || !quiet {
		t.Errorf("quiet %v, %v", quiet, err)
	}
	// A notebook's quiet is its nodes' search queue and River's jobs but the
	// search's, which other notebooks' saves keep queueing (M12/P4/S7 design
	// 6.3 item 5).
	notebookQuiet := PSQL{Container: "c0ffee", User: "nervewiki", Database: "nervewiki", Run: func(_ context.Context, _ string, a ...string) ([]byte, error) {
		if sql := a[len(a)-1]; sql != "SELECT (SELECT count(*) FROM page_search_queue WHERE notebook_id = 'nb''1') + "+
			"(SELECT count(*) FROM river_job WHERE state IN ('running', 'available') AND kind <> 'page.index_search')" {
			return nil, fmt.Errorf("the notebook's quiet read as %q", sql)
		}
		return []byte("2\n"), nil
	}}
	if quiet, err := notebookQuiet.QuietIn(context.Background(), "nb'1"); err != nil || quiet {
		t.Errorf("the notebook's quiet %v, %v; want not quiet", quiet, err)
	}
	if _, err := p.Int(context.Background(), "SELECT two rows"); err == nil {
		t.Error("two rows read as one integer")
	}
	if Literal("a'b") != "'a''b'" {
		t.Error(Literal("a'b"))
	}
}

// Prepare signs in, makes the workspace and the notebooks it lacks, and
// the tokens, with the password, a token the rate limit refused asked for
// again.
func TestPrepareMakesWhatARoundLacks(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	refused := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v0/auth/login":
			if in["email"] != "perf@example.com" || in["password"] != "pw" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = io.WriteString(w, `{"access_token":"access"}`)
		case "GET /api/v0/workspaces":
			_, _ = io.WriteString(w, `{"data":[{"slug":"perf"}]}`)
		case "GET /api/v0/workspaces/perf/notebooks":
			_, _ = io.WriteString(w, `{"data":[{"id":"nb-a","name":"a"}]}`)
		case "POST /api/v0/workspaces/perf/notebooks":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":"nb-`+in["name"]+`"}`)
		case "POST /api/v0/me/api-tokens":
			if in["current_password"] != "pw" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			// The password's bucket past its burst: once refused.
			mu.Lock()
			refused++
			mu.Unlock()
			if refused == 2 {
				w.Header().Set("Content-Type", "application/problem+json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, `{"status":429,"code":"rate_limited","detail":"later"}`)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"token":"pat-`+in["name"]+`"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, err := Prepare(ctx, srv.URL, "perf@example.com", "pw", []string{"a", "b"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if p.Notebooks["a"] != "nb-a" || p.Notebooks["b"] != "nb-b" || !slices.Equal(p.PATs, []string{"pat-perf 0", "pat-perf 1"}) {
		t.Errorf("prepared %+v", p)
	}
	count := func(call string) int {
		n := 0
		for _, c := range calls {
			if c == call {
				n++
			}
		}
		return n
	}
	if count("POST /api/v0/workspaces") != 0 || count("POST /api/v0/workspaces/perf/notebooks") != 1 {
		t.Errorf("calls %v: the workspace and notebook a were there", calls)
	}
	path := t.TempDir() + "/pats.json"
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	back, err := LoadPrepared(path)
	if err != nil || back.Notebooks["b"] != "nb-b" || len(back.PATs) != 2 {
		t.Errorf("loaded %+v, %v", back, err)
	}
	if _, err := back.Clients(3, nil); err == nil {
		t.Error("three clients of two tokens")
	}
}
