package bootstrap

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// A session opened before the page enters source remains a real live lock.
// Removing EditPage's source precheck makes the valid replacement answer
// locked instead of immutable; moving it before base/Replace breaks the
// independent mismatch and old_string cases. Both MCP entries use production
// registrants, and the outside control proves the lock is being enforced.
func TestSourceMCPPrecheckBeforeEditLocks(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Locked sources")
	source := tm.createPageWith(t, "alice", wiki.id, "", "Source", "source original\n")
	session := tm.openSession(t, "alice", source)
	tm.send(t, nodeMove("alice", source, wiki.raw), http.StatusOK)
	outside := tm.createPageWith(t, "alice", wiki.id, "", "Locked", "outside original\n")
	tm.openSession(t, "alice", outside)
	writer := tm.agent(t, "bob", wiki.id)
	for _, scope := range []string{"notebook", "account"} {
		t.Run(scope, func(t *testing.T) {
			sourceMCPUnchangedRefusal(t, tm, wiki, source, writer, scope, "edit", map[string]any{
				"path": "Locked.md", "old_string": "original", "new_string": "changed", "base_version": outside + ":1",
			}, "page.locked", "", -1)
			sourceMCPPrecedenceCases(t, tm, wiki, source, writer, scope, "changed")
		})
	}
	if reason, there := tm.endOf(t, session); !there || reason != "" || tm.sessionsOf(t, source) != 1 {
		t.Fatalf("source session after refusals: there=%v reason=%q, want still live", there, reason)
	}
	checkPages(t, tm.pool)
}

// A real write holds all parse capacity while waiting for its content row.
// Its notebook is separate, so it cannot block source's notebook lock and
// turn a missed precheck into a coincidental refusal. The ordinary edit must
// reach server_busy; the source's valid replacement must still be immutable.
func TestSourceMCPPrecheckBeforeParseBudget(t *testing.T) {
	tm := newAcmeTeamWith(t, "member", "", func(c *config.Config) {
		c.Page.ParseBudgetBytes, c.Page.ParseMaxWait = 64<<10, 300*time.Millisecond
	})
	wiki := tm.llmWiki(t, "Source parse precheck")
	source := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Source", "source original\n")
	outside := tm.createPageWith(t, "alice", wiki.id, "", "Outside", "outside original\n")
	plain := tm.openNotebook(t, "alice", "Held budget")
	held := tm.createPage(t, "alice", plain, "", "Held")
	writer := tm.agent(t, "bob", wiki.id)
	large := strings.Repeat("changed ", 16<<10)
	whileAWriteWaitsForItsLock(t, tm, held, strings.Repeat("a", 640<<10), 1, func() {
		for _, scope := range []string{"notebook", "account"} {
			t.Run(scope, func(t *testing.T) {
				sourceMCPUnchangedRefusal(t, tm, wiki, source, writer, scope, "edit", map[string]any{
					"path": "Outside.md", "old_string": "original", "new_string": large, "base_version": outside + ":1",
				}, "server_busy", "", -1)
				sourceMCPPrecedenceCases(t, tm, wiki, source, writer, scope, large)
			})
		}
	})
	checkPages(t, tm.pool)
}

// The batch's authoritative per-item source check runs before edit-lock
// prechecks of any item. A valid edit of a locked outside page at op0 must
// not hide the source edit at op1, and neither item may leave durable writes.
func TestSourceMCPBatchPlanBeforeEarlierEditLock(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Source batch precheck")
	source := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Source", "source original\n")
	outside := tm.createPageWith(t, "alice", wiki.id, "", "Locked", "outside original\n")
	session := tm.openSession(t, "alice", outside)
	writer := tm.agent(t, "bob", wiki.id)
	for _, scope := range []string{"notebook", "account"} {
		t.Run(scope, func(t *testing.T) {
			sourceMCPUnchangedRefusal(t, tm, wiki, source, writer, scope, "edit", map[string]any{
				"path": "Locked.md", "old_string": "original", "new_string": "changed", "base_version": outside + ":1",
			}, "page.locked", "", -1)
			sourceMCPUnchangedRefusal(t, tm, wiki, source, writer, scope, "batch", batched(
				opWith(opEdit("Locked.md", "original", "changed"), "base_version", outside+":1"),
				opWith(opEdit("raw/Source.md", "original", "changed"), "base_version", source+":1"),
			), "page.source_immutable", "", 1)
		})
	}
	if reason, there := tm.endOf(t, session); !there || reason != "" {
		t.Fatalf("outside lock after refused batches: there=%v reason=%q", there, reason)
	}
	checkPages(t, tm.pool)
}

// Both precedence contexts exercise the same three independently expected
// results: stale base wins before a missing replacement, a missing replacement
// wins before source policy, then a matched replacement is immutable.
func sourceMCPPrecedenceCases(t *testing.T, tm acmeTeam, wiki guardedNotebook, source string, writer *mcpApp, scope, replacement string) {
	t.Helper()
	for _, tt := range []struct {
		name, old, base, code, field string
	}{
		{"stale base before missing replacement", "absent", source + ":99", "page.revision_mismatch", ""},
		{"missing replacement before immutable", "absent", source + ":1", "validation_failed", "old_string"},
		{"matched replacement is immutable", "original", source + ":1", "page.source_immutable", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sourceMCPUnchangedRefusal(t, tm, wiki, source, writer, scope, "edit", map[string]any{
				"path": "raw/Source.md", "old_string": tt.old, "new_string": replacement, "base_version": tt.base,
			}, tt.code, tt.field, -1)
		})
	}
}

func sourceMCPUnchangedRefusal(t *testing.T, tm acmeTeam, wiki guardedNotebook, source string, writer *mcpApp,
	scope, tool string, args map[string]any, code, field string, op int,
) {
	t.Helper()
	before, state := sourcePersistentRows(t, tm, wiki.id), tm.sourceStored(t, wiki.id, source)
	member := sourceMCPCall(t, writer, scope, tool, args).toolError(t)
	message, _ := member["message"].(string)
	if member["code"] != code || !strings.HasPrefix(message, code+": ") {
		t.Errorf("%s %s = %v, want %s", scope, tool, member, code)
	}
	if op >= 0 && (member["op"] != float64(op) || !strings.Contains(message, "ops[1] (edit)")) {
		t.Errorf("%s batch = %v, want source edit at op %d", scope, member, op)
	}
	if code == "page.revision_mismatch" && member["current_version"] != source+":1" {
		t.Errorf("%s mismatch current_version=%v, want %s:1", scope, member["current_version"], source)
	}
	if field != "" {
		fields, _ := member["fields"].([]any)
		found := false
		for _, raw := range fields {
			f, _ := raw.(map[string]any)
			found = found || f["field"] == field && f["code"] == "not_allowed"
		}
		if !found {
			t.Errorf("%s validation fields=%v, want %s not_allowed", scope, fields, field)
		}
	}
	if after := sourcePersistentRows(t, tm, wiki.id); !slices.Equal(after, before) || tm.sourceStored(t, wiki.id, source) != state {
		t.Errorf("%s %s refusal changed notebook nodes, bodies, history, blobs or sessions", scope, tool)
	}
}
