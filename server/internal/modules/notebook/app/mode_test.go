package app_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The downgrade's tests (M10/P1 design 3.10), on the fakes.

// newWikiFixture is newFixture with its notebook an LLM Wiki.
func newWikiFixture() fixture {
	f := newFixture()
	f.notebook.Mode, f.notebook.SourceRootID = domain.ModeLLMWiki, uuid.NewV7()
	f.store.notebooks[f.notebook.ID] = f.notebook
	return f
}

// downgrade is the use case over f's fakes, jobs writing in the notebooks
// writing names, and clock its clock.
func (f fixture) downgrade(clock app.Clock, writing map[uuid.UUID]bool, subscribers ...app.ModeSubscriber) *app.DowngradeNotebook {
	return app.NewDowngradeNotebook(app.DowngradeNotebookDeps{
		Workspaces: f.workspaces, Finder: f.store, Notebooks: f.store, Jobs: fakeJobs{recorder: f.rec, writing: writing},
		Subscribers: subscribers, Auth: f.auth, Tx: f.tx, Clock: clock, Logger: f.logger(),
	})
}

// The downgrade locks, decides, reads the mode under the lock, asks for
// writing jobs, then writes the row standard at the clock's one read and
// tells the subscribers, in the transaction; the answer is the row
// written.
func TestDowngradeNotebookMakesItStandard(t *testing.T) {
	f := newWikiFixture()
	alice := uuid.NewV7()
	f.grant(domain.ActionChangeMode, shared.WorkspaceMember, shared.NotebookAdmin)
	clock := &tickingClock{}
	sub := &fakeModeSubscriber{recorder: f.rec, name: "a", store: f.store}

	got, err := f.downgrade(clock, nil, sub).Execute(as(alice), f.notebook.ID)

	standard := f.notebook
	standard.Mode, standard.SourceRootID, standard.UpdatedAt = domain.ModeStandard, uuid.UUID{}, now()
	if want := (app.View{Notebook: standard, Role: shared.NotebookAdmin, MemberCount: 3}); err != nil || got != want || clock.reads != 1 {
		t.Errorf("Execute() = %+v, %v after %d clock reads; want %+v after one", got, err, clock.reads, want)
	}
	if !slices.Equal(f.store.moded, []domain.Notebook{standard}) {
		t.Errorf("set %+v, want %+v", f.store.moded, standard)
	}
	calls := []string{"FindNotebook", "ShareByID in tx", "LockNotebook in tx", "Authorize notebook.change_mode in tx", "Writing in tx",
		"SetMode standard by " + alice.String() + " in tx", "ModeChanged a llm_wiki standard in tx", "CountMembers in tx"}
	if !slices.Equal(f.rec.calls, calls) {
		t.Errorf("calls = %q, want %q", f.rec.calls, calls)
	}
	want := app.ModeChange{WorkspaceID: f.acme, NotebookID: f.notebook.ID, From: domain.ModeLLMWiki, To: domain.ModeStandard, By: alice,
		At: now()}
	if !slices.Equal(sub.got, []app.ModeChange{want}) || !slices.Equal(sub.saw, []domain.Mode{domain.ModeStandard}) {
		t.Errorf("told %+v, seeing %q; want %+v, seeing standard", sub.got, sub.saw, want)
	}
	if want := (shared.Target{WorkspaceID: f.acme, NotebookID: f.notebook.ID}); f.auth.targets[0] != want {
		t.Errorf("target = %+v, want %+v", f.auth.targets[0], want)
	}
}

// The codes come 404, 403, then the notebook's own conflicts: not an LLM
// Wiki before a writing job; the mode is the one read under the lock. A
// refusal writes nothing and tells no subscriber.
func TestDowngradeNotebookRefuses(t *testing.T) {
	forbidden := shared.Forbidden()
	locked := []string{"FindNotebook", "ShareByID in tx", "LockNotebook in tx"}
	decided := append(slices.Clone(locked), "Authorize notebook.change_mode in tx")
	for _, tt := range []struct {
		name  string
		setup func(f fixture) uuid.UUID
		want  error
		calls []string
	}{
		{"the zero id", func(fixture) uuid.UUID { return uuid.UUID{} }, domain.ErrNotFound, []string{"FindNotebook"}},
		{"no notebook", func(fixture) uuid.UUID { return uuid.NewV7() }, domain.ErrNotFound, []string{"FindNotebook"}},
		{"deleted while it waited", func(f fixture) uuid.UUID { f.store.deleted[f.notebook.ID] = true; return f.notebook.ID },
			domain.ErrNotFound, locked},
		{"no role in it", func(f fixture) uuid.UUID { return f.notebook.ID }, domain.ErrNotFound, decided},
		{"an editor", func(f fixture) uuid.UUID { f.auth.err = forbidden; return f.notebook.ID }, forbidden, decided},
		{"an admin, a standard notebook", func(f fixture) uuid.UUID { f.admin(); f.standard(); return f.notebook.ID },
			domain.ErrNotLLMWiki, decided},
		{"an admin, a job writing", func(f fixture) uuid.UUID { f.admin(); f.writing(); return f.notebook.ID },
			domain.ErrJobsWriting, append(slices.Clone(decided), "Writing in tx")},
		{"an editor, a standard notebook", func(f fixture) uuid.UUID { f.auth.err = forbidden; f.standard(); return f.notebook.ID },
			forbidden, decided},
		{"an admin, a standard notebook with a job writing", func(f fixture) uuid.UUID {
			f.admin()
			f.standard()
			f.writing()
			return f.notebook.ID
		}, domain.ErrNotLLMWiki, decided},
		// Another admin's downgrade committed between the read and the lock.
		{"an LLM Wiki when found, standard under the lock", func(f fixture) uuid.UUID {
			f.admin()
			standard := f.notebook
			standard.Mode, standard.SourceRootID = domain.ModeStandard, uuid.UUID{}
			f.store.locked = map[uuid.UUID]domain.Notebook{f.notebook.ID: standard}
			return f.notebook.ID
		}, domain.ErrNotLLMWiki, decided},
	} {
		f := newWikiFixture()
		f.jobs = map[uuid.UUID]bool{}
		sub := &fakeModeSubscriber{recorder: f.rec, name: "a", store: f.store}
		id := tt.setup(f)
		got, err := f.downgrade(&tickingClock{}, f.jobs, sub).Execute(as(uuid.NewV7()), id)
		if !errors.Is(err, tt.want) || got != (app.View{}) || !slices.Equal(f.rec.calls, tt.calls) || len(f.store.moded) != 0 ||
			len(sub.got) != 0 || f.logs.Len() != 0 {
			t.Errorf("%s: Execute() = %+v, %v, calls %q, set %+v, told %+v, logs %q; want %v after %q, nothing written or told",
				tt.name, got, err, f.rec.calls, f.store.moded, sub.got, f.logs, tt.want, tt.calls)
		}
	}
}

// admin grants f's caller the downgrade as the notebook's admin.
func (f fixture) admin() {
	f.grant(domain.ActionChangeMode, shared.WorkspaceMember, shared.NotebookAdmin)
}

// standard makes f's notebook standard, as found and as locked.
func (f fixture) standard() {
	n := f.notebook
	n.Mode, n.SourceRootID = domain.ModeStandard, uuid.UUID{}
	f.store.notebooks[n.ID] = n
}

// writing gives f's notebook a job that writes.
func (f fixture) writing() { f.jobs[f.notebook.ID] = true }

// A subscriber's error is the downgrade's, after the row was written in
// the transaction, which rolls back; nothing is logged.
func TestADowngradesSubscriberFailureRollsItBack(t *testing.T) {
	f := newWikiFixture()
	f.admin()
	failure := errors.New("the stream is gone")
	sub := &fakeModeSubscriber{recorder: f.rec, name: "a", store: f.store, err: failure}

	_, err := f.downgrade(&tickingClock{}, nil, sub).Execute(as(uuid.NewV7()), f.notebook.ID)

	if !errors.Is(err, failure) || !f.tx.rolledBack || !slices.ContainsFunc(f.rec.calls, func(c string) bool {
		return strings.HasPrefix(c, "SetMode standard") && strings.HasSuffix(c, " in tx")
	}) || f.logs.Len() != 0 {
		t.Errorf("Execute() = %v, rolled back %v, calls %q, logs %q; want the failure after SetMode in the transaction, rolled back, "+
			"no log", err, f.tx.rolledBack, f.rec.calls, f.logs)
	}
}

// Two mode subscribers (v0.1 design 13.1, item 21): both follow a
// downgrade, in the order registered; the first error stops the dispatch
// and is the use case's, whose transaction rolls back.
func TestTheModeSubscribersRunInTheirOrder(t *testing.T) {
	failure := errors.New("the subscriber failed")
	for _, tt := range []struct {
		name    string
		failing string
		told    []string
	}{
		{"both follow", "", []string{"ModeChanged a llm_wiki standard in tx", "ModeChanged b llm_wiki standard in tx"}},
		{"the first fails", "a", []string{"ModeChanged a llm_wiki standard in tx"}},
		{"the second fails", "b", []string{"ModeChanged a llm_wiki standard in tx", "ModeChanged b llm_wiki standard in tx"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newWikiFixture()
			f.admin()
			a, b := &fakeModeSubscriber{recorder: f.rec, name: "a", store: f.store}, &fakeModeSubscriber{recorder: f.rec, name: "b", store: f.store}
			for _, s := range []*fakeModeSubscriber{a, b} {
				if s.name == tt.failing {
					s.err = failure
				}
			}

			_, err := f.downgrade(&tickingClock{}, nil, a, b).Execute(as(uuid.NewV7()), f.notebook.ID)

			got := slices.DeleteFunc(slices.Clone(f.rec.calls), func(c string) bool { return !strings.HasPrefix(c, "ModeChanged ") })
			if wantErr := map[bool]error{true: failure, false: nil}[tt.failing != ""]; !errors.Is(err, wantErr) ||
				!slices.Equal(got, tt.told) || f.tx.rolledBack != (tt.failing != "") {
				t.Errorf("Execute() = %v, told %q, rolled back %v; want %v, %q", err, got, f.tx.rolledBack, wantErr, tt.told)
			}
		})
	}
}

// A downgrade logs one line, by its ids, without the notebook's name (v0.1
// design 13.1 item 10).
func TestDowngradeNotebookLogs(t *testing.T) {
	f := newWikiFixture()
	f.admin()
	alice := uuid.NewV7()
	if _, err := f.downgrade(&tickingClock{}, nil).Execute(as(alice), f.notebook.ID); err != nil {
		t.Fatal(err)
	}
	logs := f.logs.String()
	if strings.Count(logs, "\n") != 1 || !strings.Contains(logs, `msg="notebook downgraded"`) ||
		!strings.Contains(logs, "workspace_id="+f.acme.String()) || !strings.Contains(logs, "notebook_id="+f.notebook.ID.String()) ||
		!strings.Contains(logs, "user_id="+alice.String()) || strings.Contains(logs, f.notebook.Name) {
		t.Errorf("logs = %q, want one line of the downgrade by its three ids, not its name", logs)
	}
}
