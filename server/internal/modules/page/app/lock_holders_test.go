package app_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The lock holders of pages are their sessions alive at now, one a page,
// the first opened, named, by page id: none of a page with no session, an
// ended or expired one.
func TestTheLockHoldersOfPages(t *testing.T) {
	f := newFixture()
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	ann, bob := uuid.NewV7(), uuid.NewV7()
	f.names = fakeNames{ann: "Ann", bob: "Bob"}
	pages := []uuid.UUID{uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()}
	session := func(page, user uuid.UUID, opened time.Duration, expires time.Duration) {
		id := uuid.NewV7()
		f.store.sessions[id] = app.EditSession{ID: id, NodeID: page, UserID: user, CreatedAt: now.Add(opened), ExpiresAt: now.Add(expires)}
	}
	session(pages[3], bob, -time.Minute, time.Minute)
	session(pages[3], ann, -2*time.Minute, time.Minute) // first opened: the page's lock
	session(pages[1], ann, -time.Minute, time.Minute)
	session(pages[2], bob, -time.Hour, -time.Second) // expired
	got, err := app.NewLockHolders(f.store, f.names).Of(context.Background(), pages, now)
	if err != nil {
		t.Fatal(err)
	}
	want := []shared.LockHolder{{PageID: pages[1], UserID: ann, DisplayName: "Ann"}, {PageID: pages[3], UserID: ann, DisplayName: "Ann"}}
	if !slices.Equal(got, want) {
		t.Errorf("lock holders %+v, want %+v", got, want)
	}
	if none, err := app.NewLockHolders(f.store, f.names).Of(context.Background(), pages[:1], now); err != nil || none != nil {
		t.Errorf("lock holders of a page without a session: %+v, %v", none, err)
	}
}
