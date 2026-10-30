package bootstrap

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

// These requests reach the actual module wired by Serve, its PostgreSQL
// readers and access rules. No HTTP use case, projection or authorizer is
// replaced. The decoded JSON below is an independent wire assertion.
func TestTheLintReadsBothModesThroughServe(t *testing.T) {
	for _, mode := range []string{"standard", "llm_wiki"} {
		t.Run(mode, func(t *testing.T) {
			f := newLintRuntime(t, nil)
			f.tm.join(t, "dana", "admin")
			nb := f.tm.createWith(t, f.tm.tokens["alice"], `{"name":"REST lint","mode":"`+mode+`","workspace_access":"none"}`)
			f.tm.addNotebookMember(t, "alice", nb.ID, "bob", "reader")
			f.tm.addNotebookMember(t, "alice", nb.ID, "carol", "editor")
			emitter := f.tm.createPageWith(t, "alice", nb.ID, "", "Emitter", "---\ntype: note\nsummary: ready\n---\n[[lint-before-a]] [[lint-before-b]]\n")
			path := "/api/v0/notebooks/" + nb.ID + "/lint"
			credentials := []struct{ name, token string }{
				{"session-admin", f.tm.tokens["alice"]},
				{"session-reader", f.tm.tokens["bob"]},
				{"session-editor", f.tm.tokens["carol"]},
				{"PAT-reader", createToken(t, f.tm.contract, f.tm.base, f.tm.tokens["bob"])},
			}
			var before time.Time
			for _, credential := range credentials {
				t.Run(credential.name, func(t *testing.T) {
					summary := lintServeGet(t, f.tm, credential.token, path, http.StatusOK, "")
					before = lintServeEnvelope(t, summary, mode)
					wanted := lintServeSummary(t, summary, mode)
					lintServeWanted(t, wanted, "lint-before-a", "lint-before-b")
					preview := lintServeObjects(t, wanted["items"])
					first := lintServeGet(t, f.tm, credential.token, path+"/wanted?limit=1", http.StatusOK, "")
					lintServeEnvelope(t, first, mode)
					items := lintServeObjects(t, first["data"])
					if len(items) != 1 || items[0]["item_key"] != preview[0]["item_key"] || items[0]["cursor"] != preview[0]["cursor"] {
						t.Fatalf("first real REST page=%v, want first preview identity %v", items, preview[0])
					}
					cursor := lintServeString(t, items[0], "cursor")
					if first["next_cursor"] != cursor {
						t.Fatalf("first next_cursor=%v, want its actual last item cursor", first["next_cursor"])
					}
					second := lintServeGet(t, f.tm, credential.token, path+"/wanted?limit=1&cursor="+url.QueryEscape(cursor), http.StatusOK, "")
					lintServeEnvelope(t, second, mode)
					items = lintServeObjects(t, second["data"])
					if len(items) != 1 || items[0]["item_key"] != preview[1]["item_key"] || second["next_cursor"] != nil {
						t.Fatalf("second real REST page=%v, want second preview and null continuation", second)
					}
					cursor = lintServeString(t, items[0], "cursor")
					end := lintServeGet(t, f.tm, credential.token, path+"/wanted?limit=1&cursor="+url.QueryEscape(cursor), http.StatusOK, "")
					lintServeEnvelope(t, end, mode)
					if items := lintServeObjects(t, end["data"]); len(items) != 0 || end["next_cursor"] != nil {
						t.Fatalf("real REST item cursor did not terminate: %v", end)
					}
				})
			}
			for _, credential := range []struct{ name, token string }{
				{"session-private-workspace-admin", f.tm.tokens["dana"]},
				{"PAT-private-workspace-admin", createToken(t, f.tm.contract, f.tm.base, f.tm.tokens["dana"])},
			} {
				t.Run(credential.name, func(t *testing.T) {
					for _, suffix := range []string{"", "/wanted"} {
						lintServeGet(t, f.tm, credential.token, path+suffix, http.StatusNotFound, "notebook.not_found")
					}
				})
			}
			t.Run("new-committed-data", func(t *testing.T) {
				f.tm.send(t, contentWrite("alice", emitter, "---\ntype: note\nsummary: ready\n---\n[[lint-after]]\n", 1, ""), http.StatusOK)
				summary := lintServeGet(t, f.tm, f.tm.tokens["bob"], path, http.StatusOK, "")
				if after := lintServeEnvelope(t, summary, mode); !after.After(before) {
					t.Fatalf("new committed read computed_at=%s, want after %s", after, before)
				}
				lintServeWanted(t, lintServeSummary(t, summary, mode), "lint-after")
				page := lintServeGet(t, f.tm, f.tm.tokens["bob"], path+"/wanted", http.StatusOK, "")
				lintServeEnvelope(t, page, mode)
				items := lintServeObjects(t, page["data"])
				if len(items) != 1 || items[0]["target"] != "lint-after" || page["next_cursor"] != nil {
					t.Fatalf("list retained a prior snapshot after the real content write: %v", page)
				}
			})
			f.invariants(t)
		})
	}
	t.Run("error-order", lintServeErrorOrder)
	t.Run("same-owner-and-snapshot", lintServeSharedSnapshot)
}

func lintServeErrorOrder(t *testing.T) {
	f := newLintRuntime(t, nil)
	visible := f.tm.createWith(t, f.tm.tokens["alice"], `{"name":"Visible","workspace_access":"viewer"}`)
	hidden := f.tm.createWith(t, f.tm.tokens["alice"], `{"name":"Private","workspace_access":"none"}`)
	f.tm.createPageWith(t, "alice", visible.ID, "", "Emitter", "[[foreign-cursor-item]]\n")
	path := "/api/v0/notebooks/" + visible.ID + "/lint"
	private := "/api/v0/notebooks/" + hidden.ID + "/lint"
	page := lintServeGet(t, f.tm, f.tm.tokens["bob"], path+"/wanted", http.StatusOK, "")
	items := lintServeObjects(t, page["data"])
	if len(items) != 1 {
		t.Fatalf("foreign binding fixture needs one actual item, got %v", page)
	}
	foreign := url.QueryEscape(lintServeString(t, items[0], "cursor"))
	for _, tc := range []struct {
		name, actor, path, code string
		status                  int
	}{
		{"known-foreign-before-hidden", "bob", private + "/wanted?limit=0&cursor=" + foreign, "bad_request", http.StatusBadRequest},
		{"unknown-valid-foreign-hidden", "bob", private + "/unknown?limit=0&cursor=" + foreign, "notebook.not_found", http.StatusNotFound},
		{"unknown-valid-foreign-visible", "alice", private + "/unknown?limit=0&cursor=" + foreign, "wiki.lint_rule_not_found", http.StatusNotFound},
		{"malformed-before-hidden-unknown", "bob", private + "/unknown?limit=0&cursor=bad", "bad_request", http.StatusBadRequest},
		{"malformed-before-visible-unknown", "bob", path + "/unknown?limit=0&cursor=bad", "bad_request", http.StatusBadRequest},
		{"unknown-before-limit", "bob", path + "/unknown?limit=0", "wiki.lint_rule_not_found", http.StatusNotFound},
		{"exclusive-mode-before-limit", "bob", path + "/missing_type?limit=0", "notebook.not_llm_wiki", http.StatusConflict},
		{"common-rule-then-limit", "bob", path + "/wanted?limit=0", "validation_failed", http.StatusUnprocessableEntity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lintServeGet(t, f.tm, f.tm.tokens[tc.actor], tc.path, tc.status, tc.code)
		})
	}
}

func lintServeSharedSnapshot(t *testing.T) {
	f := newLintRuntime(t, nil)
	nb, orphan, asset := f.seedPending(t)
	emitter := f.tm.createPageWith(t, "alice", nb.id, "", "Emitter", "---\ntype: note\nsummary: ready\n---\n[[shared-before]]\n")
	release := f.holdAssets(t)
	internal := f.start(f.actor(t, "alice"), wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}, wiki.LintQuery{})
	created := f.event(t, wiki.RunCreated, 0)
	pid := f.waitAssetLock(t)
	path := "/api/v0/notebooks/" + nb.id + "/lint"
	send := f.tm.sender(t, request("bob", http.MethodGet, path, ""))
	httpDone := make(chan answer, 1)
	go func() { httpDone <- send() }()
	joined := f.event(t, wiki.RunJoined, created.FlightID)
	if joined.FlightID != created.FlightID {
		t.Fatal("REST and the actual internal module did not join the same flight")
	}
	f.tm.send(t, contentWrite("alice", emitter, "---\ntype: note\nsummary: ready\n---\n[[shared-after]]\n", 1, ""), http.StatusOK)
	f.tm.send(t, nodeRename("alice", orphan, "Renamed while reading"), http.StatusOK)
	release()
	got := lintReceive(t, internal)
	if got.err != nil {
		t.Fatal(got.err)
	}
	var response answer
	select {
	case response = <-httpDone:
	case <-time.After(interleavingWait):
		t.Fatal("real REST follower did not return after its shared snapshot completed")
	}
	if response.err != nil || response.status != http.StatusOK {
		t.Fatalf("real REST follower = %d %s, error %v", response.status, response.body, response.err)
	}
	f.tm.contract.CheckResponse(t, response.req, response.res)
	var summary map[string]any
	decodeAnswer(t, response.body, &summary)
	if computed := lintServeEnvelope(t, summary, "llm_wiki"); !computed.Equal(got.result.ComputedAt) {
		t.Fatalf("same-flight REST computed_at=%s, internal=%s", computed, got.result.ComputedAt)
	}
	lintServeWanted(t, lintServeSummary(t, summary, "llm_wiki"), "shared-before")
	internalWanted := lintResultRule(t, got.result, wiki.RuleWanted)
	if internalWanted.Count.Value != 1 || len(internalWanted.Items) != 1 || internalWanted.Items[0].Target != "shared-before" {
		t.Fatalf("internal joined result lost the old snapshot: %+v", internalWanted)
	}
	oldPath := false
	for _, item := range lintResultRule(t, got.result, wiki.RuleOrphans).Items {
		if item.PageID != nil && *item.PageID == uuid.MustParse(orphan) {
			oldPath = item.Path != nil && *item.Path == "Outside.md"
		}
	}
	if !oldPath {
		t.Fatal("same-flight internal Paths did not retain the snapshot's old name")
	}
	pending := lintServeRule(t, summary, "undigested_sources")
	pendingItems := lintServeObjects(t, pending["items"])
	if pending["count"] != float64(1) || len(pendingItems) != 1 || pendingItems[0]["node_id"] != asset ||
		pendingItems[0]["kind"] != "asset" || pendingItems[0]["bytes"] != float64(len("source bytes")) {
		t.Fatalf("real REST pending asset=%v, want the saved attachment and its bytes", pending)
	}
	f.assertSnapshot(t, pid)
	latest := lintServeGet(t, f.tm, f.tm.tokens["bob"], path, http.StatusOK, "")
	if computed := lintServeEnvelope(t, latest, "llm_wiki"); !computed.After(got.result.ComputedAt) {
		t.Fatalf("later REST request reused computed_at %s", computed)
	}
	lintServeWanted(t, lintServeSummary(t, latest, "llm_wiki"), "shared-after")
	f.invariants(t)
	t.Logf("actual REST/internal join flight=%d, PostgreSQL PID=%d; later request read the committed replacement", created.FlightID, pid)
}

func lintServeGet(t *testing.T, tm acmeTeam, token, path string, status int, code string) map[string]any {
	t.Helper()
	req := newRequest(t, http.MethodGet, tm.base+path, token, nil)
	res, body := sendRequest(t, req)
	tm.contract.CheckResponse(t, req, res)
	if res.StatusCode != status {
		t.Fatalf("real Serve GET %s = %d %s, want %d %s", path, res.StatusCode, body, status, code)
	}
	var decoded map[string]any
	decodeAnswer(t, string(body), &decoded)
	if code != "" && decoded["code"] != code {
		t.Fatalf("real Serve GET %s code=%v, want %s", path, decoded["code"], code)
	}
	return decoded
}

func lintServeEnvelope(t *testing.T, response map[string]any, mode string) time.Time {
	t.Helper()
	if response["mode"] != mode || response["stale_pages"] != float64(0) {
		t.Fatalf("real REST envelope=%v, want mode=%s and no stale production writes", response, mode)
	}
	computed, err := time.Parse(time.RFC3339Nano, lintServeString(t, response, "computed_at"))
	if err != nil || computed.IsZero() {
		t.Fatalf("real REST computed_at=%v, error=%v", response["computed_at"], err)
	}
	lintServeNoInternals(t, response)
	return computed
}

func lintServeSummary(t *testing.T, response map[string]any, mode string) map[string]any {
	t.Helper()
	want := []string{"wanted", "orphans", "ambiguous", "invalid_frontmatter"}
	if mode == "llm_wiki" {
		want = append(want, "missing_type", "missing_summary", "undigested_sources", "invalid_sources")
	}
	var rules []string
	for _, rule := range lintServeObjects(t, response["rules"]) {
		rules = append(rules, lintServeString(t, rule, "rule"))
		lintServeObjects(t, rule["items"])
	}
	if !slices.Equal(rules, want) {
		t.Fatalf("real REST summary rule order=%v, want %v", rules, want)
	}
	return lintServeRule(t, response, "wanted")
}

func lintServeRule(t *testing.T, response map[string]any, name string) map[string]any {
	t.Helper()
	for _, rule := range lintServeObjects(t, response["rules"]) {
		if rule["rule"] == name {
			return rule
		}
	}
	t.Fatalf("real REST summary has no %s rule: %v", name, response)
	return nil
}

func lintServeWanted(t *testing.T, rule map[string]any, targets ...string) {
	t.Helper()
	items := lintServeObjects(t, rule["items"])
	if rule["count"] != float64(len(targets)) || rule["count_capped"] != false || len(items) != len(targets) || rule["next_cursor"] != nil {
		t.Fatalf("real REST wanted=%v, want the %d independently seeded targets", rule, len(targets))
	}
	// These independently saved targets are literal ASCII with no spelling
	// transformation. The public key is their complete SHA-256, not a key
	// copied from another production projection or generated by its helper.
	expectedKeys := map[string]string{}
	for _, target := range targets {
		hash := sha256.Sum256([]byte(target))
		expectedKeys[target] = "w:" + hex.EncodeToString(hash[:])
	}
	seenKeys, seenCursors := map[string]bool{}, map[string]bool{}
	var got []string
	for _, item := range items {
		if item["rule"] != "wanted" || item["pages"] != float64(1) || item["target_cut"] != false {
			t.Fatalf("real REST wanted item=%v, want one page and the complete target", item)
		}
		target := lintServeString(t, item, "target")
		key, cursor := lintServeString(t, item, "item_key"), lintServeString(t, item, "cursor")
		if key != expectedKeys[target] || seenKeys[key] || seenCursors[cursor] {
			t.Fatalf("real REST wanted identity target=%q key=%q cursor=%q, want independent key %q and unique identities", target, key, cursor, expectedKeys[target])
		}
		seenKeys[key], seenCursors[cursor] = true, true
		got = append(got, target)
	}
	slices.Sort(got)
	want := slices.Sorted(slices.Values(targets))
	if !slices.Equal(got, want) {
		t.Fatalf("real REST wanted targets=%v, want independently saved %v", got, want)
	}
}

func lintServeObjects(t *testing.T, value any) []map[string]any {
	t.Helper()
	array, ok := value.([]any)
	if !ok || array == nil {
		t.Fatalf("real REST array=%v, want a JSON array, including [] at the end", value)
	}
	objects := make([]map[string]any, len(array))
	for i, raw := range array {
		objects[i], ok = raw.(map[string]any)
		if !ok || objects[i] == nil {
			t.Fatalf("real REST array item=%v, want an object", raw)
		}
	}
	return objects
}

func lintServeString(t *testing.T, object map[string]any, field string) string {
	t.Helper()
	value, ok := object[field].(string)
	if !ok || value == "" {
		t.Fatalf("real REST %s=%v, want a nonempty string", field, object[field])
	}
	return value
}

func lintServeNoInternals(t *testing.T, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for field, child := range value {
			if slices.Contains([]string{"path", "start_cursor", "timings", "scope", "stats"}, field) {
				t.Fatalf("real REST leaked internal field %s", field)
			}
			lintServeNoInternals(t, child)
		}
	case []any:
		for _, child := range value {
			lintServeNoInternals(t, child)
		}
	}
}
