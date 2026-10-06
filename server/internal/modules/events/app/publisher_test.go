package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
)

// fakeNotifier records the payloads it sends, or fails with err.
type fakeNotifier struct {
	sent []string
	err  error
}

func (n *fakeNotifier) Notify(_ context.Context, payload string) error {
	if n.err != nil {
		return n.err
	}
	n.sent = append(n.sent, payload)
	return nil
}

// only decodes the one payload n sent, failing t unless there is exactly
// one, and its data into data.
func (n *fakeNotifier) only(t *testing.T, data any) domain.Event {
	t.Helper()
	if len(n.sent) != 1 {
		t.Fatalf("sent %d payloads, want one: %q", len(n.sent), n.sent)
	}
	e, err := domain.Decode(n.sent[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(e.Data, data); err != nil {
		t.Fatal(err)
	}
	return e
}

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.NewV7()
	}
	return out
}

// A pages event says whether the unit changed the tree and lists the
// pages whose content it wrote with their revisions: an empty list when it
// wrote none, null past MaxPages.
func TestAPagesEventSaysWhatTheUnitWrote(t *testing.T) {
	p1, p2, p3 := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	// MaxPages contents are listed, one more are not (M4–M5 Codex review R7).
	contents := func(n int) ([]app.PageChange, []domain.PageRevision) {
		changes, listed := make([]app.PageChange, n), make([]domain.PageRevision, n)
		for i := range changes {
			changes[i] = app.PageChange{PageID: uuid.NewV7(), Revision: 2}
			listed[i] = domain.PageRevision{ID: changes[i].PageID, Revision: 2}
		}
		return changes, listed
	}
	most, mostListed := contents(domain.MaxPages)
	fewer, fewerListed := contents(domain.MaxPages - 1)
	many, _ := contents(domain.MaxPages + 1)
	for _, tt := range []struct {
		name    string
		changes []app.PageChange
		tree    bool
		pages   []domain.PageRevision
	}{
		{"a tree's change and contents", []app.PageChange{{PageID: p1, Tree: true}, {PageID: p2, Revision: 3}, {PageID: p3, Tree: true, Revision: 4}},
			true, []domain.PageRevision{{ID: p2, Revision: 3}, {ID: p3, Revision: 4}}},
		{"a content alone", []app.PageChange{{PageID: p2, Revision: 3}}, false, []domain.PageRevision{{ID: p2, Revision: 3}}},
		{"the tree alone", []app.PageChange{{PageID: p1, Tree: true}}, true, []domain.PageRevision{}},
		{"as many contents as it lists", most, false, mostListed},
		{"one content fewer", fewer, false, fewerListed},
		{"more contents than it lists", many, false, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			n := &fakeNotifier{}
			err := app.NewPublisher(n).PagesWritten(context.Background(), app.PagesWritten{WorkspaceID: acme(), NotebookID: seen(), Changes: tt.changes})
			var data domain.Pages
			e := n.only(t, &data)
			if err != nil || e.Type != domain.TypePages || e.WorkspaceID != acme() || e.NotebookID != seen() || data.Tree != tt.tree ||
				(data.Pages == nil) != (tt.pages == nil) || len(data.Pages) != len(tt.pages) {
				t.Fatalf("published %+v with %+v, %v; want tree %v, pages %v", e, data, err, tt.tree, tt.pages)
			}
			for i := range tt.pages {
				if data.Pages[i] != tt.pages[i] {
					t.Errorf("page %d = %+v, want %+v", i, data.Pages[i], tt.pages[i])
				}
			}
		})
	}
}

// A lock event names the page and the session, so that a take-over's end
// and opening, in one transaction, stay two notifications.
func TestALockEventNamesThePageAndTheSession(t *testing.T) {
	n := &fakeNotifier{}
	page, ended, opened := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	pub := app.NewPublisher(n)

	err1 := pub.LockChanged(context.Background(), app.LockChanged{WorkspaceID: acme(), NotebookID: seen(), PageID: page, SessionID: ended})
	err2 := pub.LockChanged(context.Background(), app.LockChanged{WorkspaceID: acme(), NotebookID: seen(), PageID: page, SessionID: opened})

	var data domain.Lock
	if err1 != nil || err2 != nil || len(n.sent) != 2 || n.sent[0] == n.sent[1] {
		t.Fatalf("sent %q, %v, %v; want two payloads that differ", n.sent, err1, err2)
	}
	n.sent = n.sent[1:]
	if e := n.only(t, &data); e.Type != domain.TypeLock || e.NotebookID != seen() || data != (domain.Lock{PageID: page, SessionID: opened}) {
		t.Errorf("published %+v with %+v; want the page and the session", e, data)
	}
}

// An access event lists its accounts and whether it may reach everyone;
// accounts too many for a payload are shed, and it reaches everyone. One
// that concerns no one is not published.
func TestAnAccessEventListsItsAccounts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		a       app.AccessChanged
		sent    bool
		users   int
		reached bool
	}{
		{"two accounts", app.AccessChanged{WorkspaceID: acme(), UserIDs: []uuid.UUID{alice(), bob()}}, true, 2, false},
		{"everyone", app.AccessChanged{WorkspaceID: acme(), Reached: true}, true, 0, true},
		{"too many accounts", app.AccessChanged{WorkspaceID: acme(), UserIDs: ids(300)}, true, -1, true},
		{"no one", app.AccessChanged{WorkspaceID: acme()}, false, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			n := &fakeNotifier{}
			if err := app.NewPublisher(n).AccessChanged(context.Background(), tt.a); err != nil {
				t.Fatal(err)
			}
			if !tt.sent {
				if len(n.sent) != 0 {
					t.Errorf("sent %q, want nothing", n.sent)
				}
				return
			}
			var data domain.Access
			e := n.only(t, &data)
			users := len(data.UserIDs)
			if data.UserIDs == nil {
				users = -1
			}
			if e.Type != domain.TypeAccess || e.WorkspaceID != acme() || e.NotebookID != uuid.Nil() || users != tt.users || data.Reached != tt.reached {
				t.Errorf("published %+v with %d accounts, reached %v; want %d, %v", e, users, data.Reached, tt.users, tt.reached)
			}
		})
	}
}

// A notebooks_deleted event lists the notebooks, none when they are too
// many for a payload; no notebook is not published.
func TestANotebooksDeletedEventListsTheNotebooks(t *testing.T) {
	for _, tt := range []struct {
		name      string
		notebooks []uuid.UUID
		listed    int
	}{
		{"two", []uuid.UUID{seen(), unseen()}, 2},
		{"too many", ids(300), -1},
		{"none", nil, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			n := &fakeNotifier{}
			if err := app.NewPublisher(n).NotebooksDeleted(context.Background(), app.NotebooksDeleted{WorkspaceID: acme(), NotebookIDs: tt.notebooks}); err != nil {
				t.Fatal(err)
			}
			if tt.notebooks == nil {
				if len(n.sent) != 0 {
					t.Errorf("sent %q, want nothing", n.sent)
				}
				return
			}
			var data domain.NotebooksDeleted
			e := n.only(t, &data)
			listed := len(data.NotebookIDs)
			if data.NotebookIDs == nil {
				listed = -1
			}
			if e.Type != domain.TypeNotebooksDeleted || e.WorkspaceID != acme() || listed != tt.listed {
				t.Errorf("published %+v listing %d; want %d", e, listed, tt.listed)
			}
		})
	}
}

// Another M's event is published as it is: too long, it is an error and
// nothing is sent. The notifier's error is the publisher's.
func TestPublishSendsAnEventAsItIs(t *testing.T) {
	n := &fakeNotifier{}
	e, err := domain.NewEvent("later", acme(), seen(), map[string]string{"x": "y"})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.NewPublisher(n).Publish(context.Background(), e); err != nil || len(n.sent) != 1 || !strings.Contains(n.sent[0], `"later"`) {
		t.Errorf("Publish() = %v, sent %q", err, n.sent)
	}
	long, _ := domain.NewEvent("later", acme(), seen(), map[string]string{"x": strings.Repeat("y", domain.MaxPayload)})
	if err := app.NewPublisher(n).Publish(context.Background(), long); !errors.Is(err, domain.ErrTooLong) || len(n.sent) != 1 {
		t.Errorf("Publish() of a long event = %v, sent %d; want ErrTooLong and nothing more", err, len(n.sent))
	}
	down := errors.New("no transaction")
	if err := app.NewPublisher(&fakeNotifier{err: down}).LockChanged(context.Background(), app.LockChanged{WorkspaceID: acme()}); !errors.Is(err, down) {
		t.Errorf("LockChanged() = %v, want the notifier's error", err)
	}
}
