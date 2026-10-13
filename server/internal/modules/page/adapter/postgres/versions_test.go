package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// versions gives the page id the versions 1 to n, each on the one before
// in a changeset of its own, an edit by alice from the web, of the content
// "v" and its revision: one statement a table.
func (f fixture) versions(t *testing.T, id uuid.UUID, n int) {
	t.Helper()
	f.exec(t, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
		SELECT md5($1::text || i)::uuid, $2, 'edit', 'web', $3, $4::timestamptz + i * interval '1 second',
			$4::timestamptz + i * interval '1 second'
		FROM generate_series(1, $5::integer) i`, id, f.eng, f.alice, now(), n)
	f.exec(t, `INSERT INTO page_revisions (id, changeset_id, node_id, base_revision, revision, content, content_hash, byte_size,
			created_at, updated_at)
		SELECT md5($1::text || 'r' || i)::uuid, md5($1::text || i)::uuid, $1, NULLIF(i - 1, 0), i, 'v' || i,
			sha256(convert_to('v' || i, 'UTF8')), octet_length('v' || i), $2::timestamptz + i * interval '1 second',
			$2::timestamptz + i * interval '1 second'
		FROM generate_series(1, $3::integer) i`, id, now(), n)
}

// A page's versions list newer-first below a revision, at most a limit,
// each with its changeset: through a thousand and more, a page at a time,
// none repeated or skipped.
func TestListVersionsNewerFirst(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	n := f.page(t, f.eng, nil, "A", 0)
	f.versions(t, n.ID, 1201)
	var got []int
	for below := 0; ; {
		page, err := f.s.ListVersions(ctx, n.ID, below, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range page {
			got = append(got, v.Revision)
		}
		if len(page) < 100 {
			break
		}
		below = page[len(page)-1].Revision
	}
	want := make([]int, 1201)
	for i := range want {
		want[i] = 1201 - i
	}
	if !slices.Equal(got, want) {
		t.Errorf("listed %d versions from %v, want 1201 to 1 once each", len(got), got[:min(len(got), 5)])
	}
	page, err := f.s.ListVersions(ctx, n.ID, 3, 5)
	if err != nil {
		t.Fatal(err)
	}
	at := now().Add(2 * time.Second)
	if len(page) != 2 || page[0].Revision != 2 || *page[0].Base != 1 || page[0].ByteSize != 2 || !page[0].UpdatedAt.Equal(at) ||
		page[1].Base != nil || page[0].Changeset.Kind != domain.ChangesetEdit || page[0].Changeset.Client != domain.ClientWeb ||
		page[0].Changeset.By != f.alice || !page[0].Changeset.CreatedAt.Equal(at) {
		t.Errorf("versions below 3 = %+v, want 2 on 1, then 1, alice's edits from the web", page)
	}
}

// The version that may hold a revision is the first at or above it: an
// edit session's, from its base on.
func TestFindVersionIsTheFirstAtOrAbove(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	n := f.page(t, f.eng, nil, "A", 0)
	other := f.page(t, f.eng, nil, "B", 1)
	f.versions(t, other.ID, 9)
	cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetEdit, Client: domain.ClientWeb, By: f.alice, At: now()}
	session := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetEdit, Client: domain.ClientWeb, By: f.alice,
		At: now()}
	one := 1
	for _, c := range []app.Changeset{cs, session} {
		if err := f.s.CreateChangeset(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []app.Revision{{ChangesetID: cs.ID, Revision: 1, Content: "one"}, {ChangesetID: session.ID, Base: &one, Revision: 4,
		Content: "four"}} {
		sum := sha256.Sum256([]byte(r.Content))
		r.ID, r.NodeID, r.Hash, r.ByteSize, r.At = uuid.NewV7(), n.ID, sum[:], len(r.Content), now()
		if err := f.s.RecordRevision(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	for revision, want := range map[int]int{1: 1, 2: 4, 4: 4} {
		v, err := f.s.FindVersion(ctx, n.ID, revision)
		if err != nil || v.Revision != want {
			t.Errorf("the version at or above %d = %+v, %v; want %d", revision, v, err, want)
		}
	}
	if v, err := f.s.FindVersion(ctx, n.ID, 5); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("the version at or above 5 = %+v, %v; want ErrNotFound", v, err)
	}
	v, err := f.s.FindVersion(ctx, n.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	content, err := f.s.VersionContent(ctx, v.ID)
	if err != nil || content != "four" {
		t.Errorf("its content = %q, %v; want four", content, err)
	}
	if _, err := f.s.VersionContent(ctx, uuid.NewV7()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("no version's content = %v, want ErrNotFound", err)
	}
}

// A page's history is read in the trash too: its node, with when it was
// deleted, and its versions.
func TestAPageInTheTrashHasItsHistory(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	n := f.page(t, f.eng, nil, "A", 0)
	f.versions(t, n.ID, 2)
	alive, err := f.s.FindAnyNode(ctx, n.ID)
	if err != nil || alive.ID != n.ID || alive.Name != "A" || alive.DeletedAt != nil {
		t.Errorf("alive = %+v, %v; want A, not deleted", alive, err)
	}
	at := now().Add(time.Hour)
	if err := f.s.DeleteNodes(ctx, []uuid.UUID{n.ID}, f.alice, at); err != nil {
		t.Fatal(err)
	}
	trashed, err := f.s.FindAnyNode(ctx, n.ID)
	if err != nil || trashed.ID != n.ID || trashed.Kind != domain.KindPage || trashed.DeletedAt == nil || !trashed.DeletedAt.Equal(at) {
		t.Errorf("trashed = %+v, %v; want A, deleted at %v", trashed, err, at)
	}
	if versions, err := f.s.ListVersions(ctx, n.ID, 0, 10); err != nil || len(versions) != 2 {
		t.Errorf("its versions = %+v, %v; want both", versions, err)
	}
	if _, err := f.s.FindAnyNode(ctx, uuid.NewV7()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("no node = %v, want ErrNotFound", err)
	}
}

// The changeset a page's alive edit session writes in: none for a session
// not written, expired or ended, or of another page.
func TestAliveChangesetOf(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	n := f.page(t, f.eng, nil, "A", 0)
	other := f.page(t, f.eng, nil, "B", 1)
	cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetEdit, Client: domain.ClientWeb, By: f.alice, At: now()}
	if err := f.s.CreateChangeset(ctx, cs); err != nil {
		t.Fatal(err)
	}
	none := func(what string) {
		t.Helper()
		if id, err := f.s.AliveChangesetOf(ctx, n.ID, now()); err != nil || id != (uuid.UUID{}) {
			t.Errorf("%s: %s, %v; want none", what, id, err)
		}
	}
	none("no session")
	s := f.open(t, n.ID, f.eng, f.alice, now().Add(time.Minute))
	f.open(t, other.ID, f.eng, f.alice, now().Add(time.Minute))
	none("a session not written")
	if err := f.s.SetSessionWrite(ctx, s.ID, cs.ID, 2); err != nil {
		t.Fatal(err)
	}
	if id, err := f.s.AliveChangesetOf(ctx, n.ID, now()); err != nil || id != cs.ID {
		t.Errorf("an alive session's changeset = %s, %v; want %s", id, err, cs.ID)
	}
	if id, err := f.s.AliveChangesetOf(ctx, n.ID, now().Add(time.Minute)); err != nil || id != (uuid.UUID{}) {
		t.Errorf("at its expiry = %s, %v; want none", id, err)
	}
	f.end(t, n.ID, nil, domain.EndedUnlocked, f.alice, now())
	none("a session ended")
}

// A revert's changeset names what it reverts, and a unit merging into it
// reads it back; no other changeset names one.
func TestARevertsChangesetNamesWhatItReverts(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	reverted := uuid.NewV7()
	revert := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetRevert, RevertsID: reverted,
		Client: domain.ClientWeb, By: f.alice, At: now()}
	edit := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetVersionRestore, Client: domain.ClientWeb,
		By: f.alice, At: now()}
	for _, c := range []app.Changeset{revert, edit} {
		if err := f.s.CreateChangeset(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []app.Changeset{revert, edit} {
		got, err := f.s.LockChangeset(ctx, want.ID)
		if err != nil || got.Kind != want.Kind || got.RevertsID != want.RevertsID {
			t.Errorf("changeset = %+v, %v; want %s reverting %s", got, err, want.Kind, want.RevertsID)
		}
	}
}

// Two versions of a page never hold one revision: the unique index refuses
// the second.
func TestAPageHasOneVersionARevision(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	n := f.page(t, f.eng, nil, "A", 0)
	for i := range 2 {
		cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetEdit, Client: domain.ClientWeb, By: f.alice,
			At: now()}
		if err := f.s.CreateChangeset(ctx, cs); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte("x"))
		err := f.s.RecordRevision(ctx, app.Revision{ID: uuid.NewV7(), ChangesetID: cs.ID, NodeID: n.ID, Revision: 2, Content: "x",
			Hash: sum[:], ByteSize: 1, At: now()})
		var pgErr *pgconn.PgError
		switch {
		case i == 0 && err != nil:
			t.Fatal(err)
		case i == 1 && (!errors.As(err, &pgErr) || pgErr.ConstraintName != "page_revisions_node_id_revision_key"):
			t.Errorf("a second version of revision 2 = %v, want the unique index's violation", err)
		}
	}
}
