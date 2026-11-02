package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The port observes the actual entry into RR, before its first statement. The
// real TxManager still owns every begin/commit/rollback and rejects nesting.
type virtualSnapshots struct {
	shared.Snapshots
	tx     page.Tx
	calls  atomic.Int32
	nest   atomic.Bool
	nested atomic.Bool
	mu     sync.Mutex
	gate   *sourceWriteGate
}

func (p *virtualSnapshots) WithinSnapshot(ctx context.Context, fn func(context.Context) error) error {
	p.calls.Add(1)
	p.mu.Lock()
	g := p.gate
	p.gate = nil
	p.mu.Unlock()
	if g != nil {
		close(g.reached)
		select {
		case <-g.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if p.nest.Load() {
		return p.tx.WithinTx(ctx, func(inner context.Context) error {
			err := p.Snapshots.WithinSnapshot(inner, fn)
			p.nested.Store(errors.Is(err, postgres.ErrNestedSnapshot))
			return err
		})
	}
	return p.Snapshots.WithinSnapshot(ctx, fn)
}

type virtualNotebooks struct {
	page.Notebooks
	mu         sync.Mutex
	workspaces []bool
}

func (n *virtualNotebooks) WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	n.mu.Lock()
	n.workspaces = append(n.workspaces, postgres.InTx(ctx))
	n.mu.Unlock()
	return n.Notebooks.WorkspaceOf(ctx, id)
}

func virtualReadTeam(t *testing.T) (acmeTeam, *virtualSnapshots, *virtualNotebooks) {
	t.Helper()
	p, n := &virtualSnapshots{}, &virtualNotebooks{}
	tm := newAcmeTeamWiring(t, "member", "", wiring{page: func(d *page.Deps) {
		p.Snapshots, p.tx = d.Snapshots, d.Tx
		d.Snapshots = p
		n.Notebooks = d.Notebooks
		d.Notebooks = n
	}})
	return tm, p, n
}

func virtualAwait(t *testing.T, done <-chan mcpAnswer) mcpAnswer {
	t.Helper()
	select {
	case a := <-done:
		return a
	case <-time.After(interleavingWait):
		t.Fatal("virtual read did not finish")
		return mcpAnswer{}
	}
}

func virtualGateRead(t *testing.T, p *virtualSnapshots, send func() mcpAnswer) (*sourceWriteGate, <-chan mcpAnswer) {
	t.Helper()
	g := &sourceWriteGate{reached: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(g.open)
	p.mu.Lock()
	p.gate = g
	p.mu.Unlock()
	done := make(chan mcpAnswer, 1)
	go func() { done <- send() }()
	select {
	case <-g.reached:
	case got := <-done:
		t.Fatalf("read answered before the RR seam: %.400s", got.body)
	case <-time.After(interleavingWait):
		t.Fatal("read never reached RR after its mode hint")
	}
	return g, done
}

func TestVirtualRead(t *testing.T) {
	t.Run("normal_unlocked", func(t *testing.T) {
		tm, p, _ := virtualReadTeam(t)
		wiki := tm.llmWiki(t, "Snapshot ownership")
		tm.createPageWith(t, "alice", wiki.id, "", "Real", "ordinary")
		m := tm.agent(t, "alice", wiki.id)
		before := p.calls.Load()
		res := sourceMCPCall(t, m, "notebook", "read", map[string]any{"path": "Real.md"}).result(t)
		if blocksOf(t, res)[1]["text"] != "ordinary" || p.calls.Load() != before {
			t.Fatal("ordinary read gained a snapshot or changed content")
		}
		res = sourceMCPCall(t, m, "notebook", "read", map[string]any{"path": "index.md"}).result(t)
		if metaOf(t, res)["virtual"] != true || p.calls.Load() != before+1 {
			t.Fatal("virtual read did not own exactly one snapshot")
		}
	})
	t.Run("rename_snapshot", func(t *testing.T) {
		tm, _, _ := virtualReadTeam(t)
		wiki := tm.llmWiki(t, "Rename during virtual read")
		tm.createPageWith(t, "alice", wiki.id, "", "BeforeAlpha", "alpha")
		tm.createPageWith(t, "alice", wiki.id, "", "BeforeBeta", "beta")
		m := tm.agent(t, "alice", wiki.id)
		ctx := context.Background()
		holder, err := tm.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = holder.Rollback(ctx) })
		// History's facts/first changes need page_revisions after its first
		// changeset read. A name-only move writes no content revisions.
		if _, err := holder.Exec(ctx, "LOCK TABLE page_revisions IN ACCESS EXCLUSIVE MODE"); err != nil {
			t.Fatal(err)
		}
		done := make(chan mcpAnswer, 1)
		go func() { done <- sourceMCPCall(t, m, "notebook", "read", map[string]any{"path": "log.md"}) }()
		pgtest.WaitForTableLockWaits(t, tm.pool, "page_revisions", 1, interleavingWait)
		// The observed blocked backend is a real late query, not a timing
		// sleep. Rename commits before that query and its later PathsOf run.
		structured(t, sourceMCPCall(t, m, "account", "move", map[string]any{"from": "BeforeAlpha.md", "to": "AfterAlpha.md", "update_links": false}))
		if err := holder.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		res := virtualAwait(t, done).result(t)
		text := blocksOf(t, res)[1]["text"].(string)
		if !strings.Contains(text, "](<BeforeAlpha.md>)") || !strings.Contains(text, "](<BeforeBeta.md>)") || strings.Contains(text, "AfterAlpha") {
			t.Fatalf("one read mixed paths across the rename: %q", text)
		}
		next := blocksOf(t, sourceMCPCall(t, m, "notebook", "read", map[string]any{"path": "log.md"}).result(t))[1]["text"].(string)
		if !strings.Contains(next, "](<AfterAlpha.md>)") || strings.Contains(next, "](<BeforeAlpha.md>)") {
			t.Fatalf("next read did not see the committed rename: %q", next)
		}
		checkPages(t, tm.pool)
	})
	t.Run("recheck_mode", func(t *testing.T) {
		tm, p, n := virtualReadTeam(t)
		wiki := tm.llmWiki(t, "Downgrade before snapshot")
		m := tm.agent(t, "alice", wiki.id)
		n.mu.Lock()
		n.workspaces = nil
		n.mu.Unlock()
		g, done := virtualGateRead(t, p, func() mcpAnswer { return sourceMCPCall(t, m, "notebook", "read", map[string]any{"path": "index.md"}) })
		if status, body := tm.downgrade(t, "alice", wiki.id); status != http.StatusOK {
			t.Fatalf("downgrade: %d %s", status, body)
		}
		tm.createPageWith(t, "alice", wiki.id, "", "index", "real after downgrade")
		g.open()
		res := virtualAwait(t, done).result(t)
		if res["isError"] == true || metaOf(t, res)["virtual"] != nil || blocksOf(t, res)[1]["text"] != "real after downgrade" {
			t.Fatalf("mode recheck did not fall back: %v", res)
		}
		n.mu.Lock()
		calls := slices.Clone(n.workspaces)
		n.mu.Unlock()
		if len(calls) < 3 || !slices.Equal(calls[len(calls)-3:], []bool{false, true, false}) {
			t.Fatalf("fallback did not reauthorize on outer context: %v", calls)
		}
	})
	t.Run("recheck_visibility", func(t *testing.T) {
		tm, p, _ := virtualReadTeam(t)
		wiki := tm.llmWiki(t, "Visibility before snapshot")
		tm.createPageWith(t, "alice", wiki.id, "", "Secret", "content")
		m := tm.agent(t, "bob", wiki.id)
		g, done := virtualGateRead(t, p, func() mcpAnswer { return sourceMCPCall(t, m, "notebook", "read", map[string]any{"path": "index.md"}) })
		tm.send(t, tm.removal("alice", "bob"), http.StatusNoContent)
		g.open()
		a := virtualAwait(t, done)
		e := a.toolError(t)
		if e["code"] != "notebook.not_found" || strings.Contains(string(a.body), "Secret") {
			t.Fatalf("snapshot visibility loss exposed content: %.500s", a.body)
		}
	})
	t.Run("no_nested", func(t *testing.T) {
		tm, p, _ := virtualReadTeam(t)
		wiki := tm.llmWiki(t, "No nested virtual snapshot")
		m := tm.agent(t, "alice", wiki.id)
		res := sourceMCPCall(t, m, "notebook", "read", map[string]any{"path": "index.md"}).result(t)
		if res["isError"] == true || p.calls.Load() != 1 {
			t.Fatalf("generator nested its normal snapshot: %v", res)
		}
		p.nest.Store(true)
		res = sourceMCPCall(t, m, "notebook", "read", map[string]any{"path": "index.md"}).result(t)
		if res["isError"] != true || !p.nested.Load() {
			t.Fatalf("writer RC nesting was swallowed: %v, nested=%v", res, p.nested.Load())
		}
	})
}
