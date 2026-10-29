package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestSourceNoOpsKeepTheirDifferentContracts(t *testing.T) {
	for _, tt := range []struct {
		name                                  string
		mode                                  domain.Mode
		in, session, foreign, merged, restore bool
		body, want                            string
	}{
		{"one_shot_source", domain.ModeLLMWiki, true, false, false, false, false, "A", ""},
		{"version_source", domain.ModeLLMWiki, true, false, false, false, true, "A", "page.source_immutable"},
		{"valid_source_session", domain.ModeLLMWiki, true, true, false, false, false, "A", "page.source_immutable"},
		{"empty_source_session", domain.ModeLLMWiki, true, true, false, false, false, "", "page.source_immutable"},
		{"foreign_session", domain.ModeLLMWiki, true, true, true, false, false, "A", "page.edit_session_ended"},
		{"merged_source", domain.ModeLLMWiki, true, false, false, true, false, "A", "page.source_immutable"},
		{"outside_version", domain.ModeLLMWiki, false, false, false, false, true, "A", ""},
		{"outside_session", domain.ModeLLMWiki, false, true, false, false, false, "A", ""},
		{"standard_version", domain.ModeStandard, true, false, false, false, true, "A", ""},
		{"standard_session", domain.ModeStandard, true, true, false, false, false, "A", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionWrite)
			raw := f.page("raw", nil, 0)
			var parent *uuid.UUID
			if tt.in {
				parent = &raw.ID
			}
			p := f.page("P", parent, 0)
			f.content(p.ID, tt.body, 3)
			f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: tt.mode, SourceRootID: raw.ID}}
			g := &guard{recorder: f.rec, err: errors.New("no-op reached a write guard")}
			f.guards = []app.WriteGuard{g, app.NewEditLock(f.store, f.names)}
			w := app.ContentWrite{NodeID: p.ID, Content: tt.body, Base: 1, Rewrite: tt.restore || tt.session}
			if tt.session {
				user := f.alice
				if tt.foreign {
					user = uuid.NewV7()
				}
				w.EditSession = f.session(p.ID, user, now().Add(time.Hour)).ID
			}
			if tt.merged {
				w.Merged = new(2)
			}
			kind := domain.ChangesetEdit
			if tt.restore {
				kind = domain.ChangesetVersionRestore
			}
			var revision int
			out, err := f.writer().Run(f.asAlice(), app.UnitSpec{NotebookID: f.eng, Action: domain.ActionWrite,
				Client: domain.ClientWeb, Kind: kind, NotFound: domain.ErrNotFound}, func(ctx context.Context, u *app.Unit) error {
				var err error
				revision, err = u.WriteContent(ctx, w)
				return err
			})
			if codeOf(err) != tt.want {
				t.Fatalf("no-op=%v, want %q; calls=%v", err, tt.want, f.rec.calls)
			}
			if tt.want == "" && revision != 3 {
				t.Fatalf("revision=%d, want unchanged 3", revision)
			}
			if out.ChangesetID != uuid.Nil() || len(g.steps) != 0 || f.called("AliveSessionsOf in tx") || f.called("TouchChangeset in tx") || f.called("WriteContent in tx") || len(f.store.changesets) != 0 {
				t.Fatalf("no-op did writes/guards/locks: %+v / %v", out, f.rec.calls)
			}
			if tt.mode == domain.ModeStandard || !tt.session && !tt.restore && !tt.merged {
				if f.count("Ancestors in tx") != 0 {
					t.Fatalf("ordinary no-op added ancestors: %v", f.rec.calls)
				}
			}
		})
	}
}

func TestSourceMCPChecksBeforeLocksAndBudgets(t *testing.T) {
	for _, locked := range []bool{false, true} {
		t.Run(map[bool]string{false: "budget", true: "lock"}[locked], func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRead, domain.ActionWrite)
			raw := f.page("raw", nil, 0)
			p := f.page("P", &raw.ID, 0)
			f.content(p.ID, "one two", 2)
			f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
			f.budget.err = errors.New("parse budget exhausted")
			if locked {
				f.session(p.ID, f.alice, now().Add(time.Hour))
			}
			for _, tt := range []struct {
				old  string
				base *string
				want string
			}{
				{"missing", tokenOf(p.ID, 1), "page.revision_mismatch"},
				{"missing", tokenOf(p.ID, 2), "validation_failed old_string"},
				{"two", tokenOf(p.ID, 2), "page.source_immutable"},
			} {
				f.rec.calls = nil
				_, err := f.editPage(f.eng, app.PageEdit{Path: "raw/P.md", Old: tt.old, New: "changed", Base: tt.base})
				if codeOf(err) != tt.want || f.called("AliveSessionsOf") || f.took() || f.called("ShareWorkspace in tx") {
					t.Errorf("MCP precheck=%v, want %s before lock/parse/unit; calls=%v", err, tt.want, f.rec.calls)
				}
				for _, call := range f.rec.calls {
					if strings.HasPrefix(call, "Ancestors") || strings.Contains(call, "page.manage_source") {
						t.Errorf("precheck reread ancestors/authorization: %s", call)
					}
				}
			}
		})
	}
}

func TestSourceModeGuardVetoesSessionsBeforeEditLocks(t *testing.T) {
	f := newFixture()
	g := app.NewModeGuard(f.auth, f.store, f.store)
	v, ok := any(g).(app.EditSessionVetoer)
	if !ok {
		t.Fatal("mode guard does not veto source edit sessions")
	}
	f.grant(domain.ActionEdit)
	raw := f.page("raw", nil, 0)
	p := f.page("Empty source", &raw.ID, 0)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
	f.session(p.ID, f.alice, now().Add(time.Hour))
	f.vetoers = []app.EditSessionVetoer{v, app.NewEditLock(f.store, f.names)}
	_, err := f.open(p.ID)
	var refusal *domain.Refused
	if codeOf(err) != "page.source_immutable" || !errors.As(err, &refusal) || refusal.NodeID != p.ID || f.called("AliveSessionsOf in tx") || f.called("CreateSession in tx") {
		t.Fatalf("opening source=%v, refusal=%+v calls=%v", err, refusal, f.rec.calls)
	}
}

func TestSourceChangedWritesReachTheGuard(t *testing.T) {
	for _, tt := range []struct {
		name, before, want                     string
		admin, rewrite, session, merged, stale bool
	}{
		{"writer existing", "A", "page.source_immutable", false, false, false, false, false},
		{"admin existing", "A", "page.source_immutable", true, false, false, false, false},
		{"empty first body", "", "", false, false, false, false, false},
		{"system rewrite", "A", "", false, true, false, false, false},
		{"empty valid session", "", "page.source_immutable", false, false, true, false, false},
		{"merged body without planner", "", "page.source_immutable", false, false, false, true, false},
		{"mismatch before guard", "A", "page.revision_mismatch", false, false, false, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionWrite)
			if tt.admin {
				f.grant(domain.ActionManageSource)
			}
			raw := f.page("raw", nil, 0)
			p := f.page("P", &raw.ID, 0)
			f.content(p.ID, tt.before, 2)
			f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
			f.guards = []app.WriteGuard{app.NewModeGuard(f.auth, f.store, f.store), app.NewEditLock(f.store, f.names)}
			w := app.ContentWrite{NodeID: p.ID, Content: "B", Base: 2, Rewrite: tt.rewrite}
			if tt.session {
				w.EditSession = f.session(p.ID, f.alice, now().Add(time.Hour)).ID
			}
			if tt.merged {
				w.Merged = new(2)
			}
			if tt.stale {
				w.Base = 1
			}
			_, err := f.writer().Run(f.asAlice(), app.UnitSpec{NotebookID: f.eng, Action: domain.ActionWrite,
				Client: domain.ClientWeb, NotFound: domain.ErrNotFound}, func(ctx context.Context, u *app.Unit) error {
				_, err := u.WriteContent(ctx, w)
				return err
			})
			if codeOf(err) != tt.want {
				t.Fatalf("write=%v, want %q", err, tt.want)
			}
			if tt.want != "" {
				if f.called("WriteContent in tx") || f.store.contents[p.ID].Content != tt.before || len(f.store.changesets) != 0 {
					t.Fatalf("refused write changed state: %v", f.rec.calls)
				}
			} else if got := f.store.contents[p.ID]; got.Content != "B" || got.Revision != 3 {
				t.Fatalf("written=%+v", got)
			}
			if tt.stale && f.called("Ancestors in tx") {
				t.Fatalf("mismatch reached source guard: %v", f.rec.calls)
			}
		})
	}
}

// Every individual content item is allowed here: the page is empty at each
// source write and is cleared outside. The documented conservative final
// body guard still receives both source changes and owns the first item.
func TestSourceMergedBatchGuardKeepsItsActualOwner(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionManageSource)
	raw := f.page("raw", nil, 0)
	p := f.page("P", &raw.ID, 0)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
	f.guards = []app.WriteGuard{app.NewModeGuard(f.auth, f.store, f.store)}
	write := func(path, body string) domain.BatchArgs {
		op := writeOp(path, body)
		op.Base = tokenOf(p.ID, 1)
		return op
	}
	_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, write("raw/P.md", "A"), moveOp("raw/P.md", "P.md"),
		write("P.md", ""), moveOp("P.md", "raw/P.md"), write("raw/P.md", "B"))
	var refused *domain.Refused
	if opOf(err) != "0 page.source_immutable" || !errors.As(err, &refused) || refused.NodeID != p.ID || !f.tx.rolledBack ||
		f.called("WriteContent in tx") || f.called("CreateChangeset in tx") {
		t.Fatalf("merged fallback=%v (%+v), calls=%v", err, refused, f.rec.calls)
	}
	if f.count("ListNodes") > 1 {
		t.Fatalf("refusal replanned: %v", f.rec.calls)
	}
}

func TestSourceMergedCreationIsCheckedWithoutAPlanner(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	raw := f.page("raw", nil, 0)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
	f.guards = []app.WriteGuard{app.NewModeGuard(f.auth, f.store, f.store)}
	_, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
		_, err := u.CreatePage(ctx, app.PageDraft{ParentID: &raw.ID, Title: "Merged", Content: "B", Merged: new(2)})
		return err
	})
	if codeOf(err) != "page.source_immutable" || len(f.store.nodes) != 1 || len(f.store.changesets) != 0 {
		t.Fatalf("merged create=%v, nodes=%v calls=%v", err, f.store.nodes, f.rec.calls)
	}
}
