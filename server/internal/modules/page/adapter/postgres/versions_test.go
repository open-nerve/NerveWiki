package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// versions gives the page id the versions 1 to n, each on the one before
// in a changeset of its own, an edit by alice from the web, of the content
// "v" and its revision: one statement a table. Version i's changeset is
// made at now + i minutes, the version written first at now + i minutes
// and a second, last at now + i minutes and two.
func (f fixture) versions(t *testing.T, id uuid.UUID, n int) {
	t.Helper()
	f.exec(t, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
		SELECT md5($1::text || i)::uuid, $2, 'edit', 'web', $3, $4::timestamptz + i * interval '1 minute',
			$4::timestamptz + i * interval '1 minute'
		FROM generate_series(1, $5::integer) i`, id, f.eng, f.alice, now(), n)
	f.exec(t, `INSERT INTO page_revisions (id, changeset_id, node_id, base_revision, revision, content, content_hash, byte_size,
			created_at, updated_at)
		SELECT md5($1::text || 'r' || i)::uuid, md5($1::text || i)::uuid, $1::uuid, NULLIF(i - 1, 0), i, 'v' || i,
			sha256(convert_to('v' || i, 'UTF8')), octet_length('v' || i),
			$2::timestamptz + i * interval '1 minute' + interval '1 second',
			$2::timestamptz + i * interval '1 minute' + interval '2 seconds'
		FROM generate_series(1, $3::integer) i`, id, now(), n)
}

// versionID and changesetID are the ids versions gives the page id's i-th version and its changeset.
func versionID(t *testing.T, f fixture, id uuid.UUID, i int) (version, changeset uuid.UUID) {
	t.Helper()
	if err := f.pool.QueryRow(context.Background(), "SELECT md5($1::text || 'r' || $2::integer)::uuid, md5($1::text || $2::integer)::uuid",
		id, i).Scan(&version, &changeset); err != nil {
		t.Fatal(err)
	}
	return version, changeset
}

// newest is ListVersions's below for the newest: a revision no version has.
const newest = domain.MaxRevision + 1

// A page's versions list newer-first below a revision, at most a limit,
// each with its changeset: through a thousand and more, a page at a time,
// none repeated or skipped.
func TestListVersionsNewerFirst(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	n := f.page(t, f.eng, nil, "A", 0)
	f.versions(t, n.ID, 1201)
	other := f.page(t, f.eng, nil, "B", 1)
	f.versions(t, other.ID, 3)
	var got []int
	for below := newest; ; {
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
	id, cs := versionID(t, f, n.ID, 2)
	one := 1
	want2 := domain.Version{ID: id, Base: &one, Revision: 2, ByteSize: 2, CreatedAt: now().Add(2*time.Minute + time.Second),
		UpdatedAt: now().Add(2*time.Minute + 2*time.Second), Changeset: domain.VersionChangeset{ID: cs, Kind: domain.ChangesetEdit,
			Client: domain.ClientWeb, By: f.alice, CreatedAt: now().Add(2 * time.Minute)}}
	if len(page) != 2 || !reflect.DeepEqual(page[0], want2) || page[1].Revision != 1 || page[1].Base != nil {
		t.Errorf("versions below 3 = %+v, want %+v, then 1", page, want2)
	}
}

// The list reads no content: a version of 5 MiB a row, a hundred a page
// (M8 design review A-N8).
func TestListVersionsReadsNoContent(t *testing.T) {
	if _, has := reflect.TypeFor[gen.ListVersionsRow]().FieldByName("Content"); has {
		t.Error("ListVersions reads the versions' contents")
	}
}

// The version that may hold a revision is the first at or above it, read
// with its content: an edit session's, from its base on.
func TestFindVersionIsTheFirstAtOrAbove(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	n := f.page(t, f.eng, nil, "A", 0)
	other := f.page(t, f.eng, nil, "B", 1)
	f.versions(t, other.ID, 9)
	cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetEdit, Client: domain.ClientWeb, By: f.alice, At: now()}
	session := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetVersionRestore, Client: domain.ClientAPI,
		By: f.alice, At: now().Add(time.Minute)}
	one := 1
	for _, c := range []app.Changeset{cs, session} {
		if err := f.s.CreateChangeset(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	four := app.Revision{ID: uuid.NewV7(), ChangesetID: session.ID, NodeID: n.ID, Base: &one, Revision: 4, Content: "four",
		ByteSize: 4, At: now().Add(2 * time.Minute)}
	for _, r := range []app.Revision{{ID: uuid.NewV7(), ChangesetID: cs.ID, NodeID: n.ID, Revision: 1, Content: "one", ByteSize: 3,
		At: now()}, four} {
		sum := sha256.Sum256([]byte(r.Content))
		r.Hash = sum[:]
		if err := f.s.RecordRevision(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	for revision, want := range map[int]string{1: "one", 2: "four", 4: "four"} {
		_, content, err := f.s.FindVersion(ctx, n.ID, revision)
		if err != nil || content != want {
			t.Errorf("the version at or above %d holds %q, %v; want %q", revision, content, err, want)
		}
	}
	if v, _, err := f.s.FindVersion(ctx, n.ID, 5); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("the version at or above 5 = %+v, %v; want ErrNotFound", v, err)
	}
	v, _, err := f.s.FindVersion(ctx, n.ID, 3)
	want := domain.Version{ID: four.ID, Base: &one, Revision: 4, ByteSize: 4, CreatedAt: four.At, UpdatedAt: four.At,
		Changeset: domain.VersionChangeset{ID: session.ID, Kind: domain.ChangesetVersionRestore, Client: domain.ClientAPI, By: f.alice,
			CreatedAt: session.At}}
	if err != nil || !reflect.DeepEqual(v, want) {
		t.Errorf("the version at or above 3 = %+v, %v; want %+v", v, err, want)
	}
}

// A changeset's version of a page is written again only on its own
// revision: a write of the changeset on another's in between would make
// it hold that one's revisions too (M8/P1 review C4).
func TestAVersionGoesOnFromItsOwnRevision(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	n := f.page(t, f.eng, nil, "A", 0)
	first := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetEdit, Client: domain.ClientWeb, By: f.alice, At: now()}
	between := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetEdit, Client: domain.ClientWeb, By: f.alice,
		At: now()}
	write := func(c app.Changeset, base, revision int) error {
		sum := sha256.Sum256([]byte("x"))
		return f.s.RecordRevision(ctx, app.Revision{ID: uuid.NewV7(), ChangesetID: c.ID, NodeID: n.ID, Base: &base, Revision: revision,
			Content: "x", Hash: sum[:], ByteSize: 1, At: now()})
	}
	for _, c := range []app.Changeset{first, between} {
		if err := f.s.CreateChangeset(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	if err := write(first, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := write(first, 2, 3); err != nil {
		t.Errorf("the changeset's version on its own revision: %v", err)
	}
	if err := write(between, 3, 4); err != nil {
		t.Fatal(err)
	}
	if err := write(first, 4, 5); err == nil {
		t.Error("the changeset's version went on from another's revision")
	}
	if got := f.count(t, "SELECT count(*) FROM page_revisions WHERE changeset_id = $1 AND base_revision = 1 AND revision = 3",
		first.ID); got != 1 {
		t.Errorf("%d versions of the first changeset from 1 to 3, want it kept", got)
	}
}

// FindVersion reads the row as it is: its ids, its base and its times,
// first and last written, apart.
func TestFindVersionReadsTheRow(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	n := f.page(t, f.eng, nil, "A", 0)
	f.versions(t, n.ID, 3)
	id, cs := versionID(t, f, n.ID, 2)
	one := 1
	want := domain.Version{ID: id, Base: &one, Revision: 2, ByteSize: 2, CreatedAt: now().Add(2*time.Minute + time.Second),
		UpdatedAt: now().Add(2*time.Minute + 2*time.Second), Changeset: domain.VersionChangeset{ID: cs, Kind: domain.ChangesetEdit,
			Client: domain.ClientWeb, By: f.alice, CreatedAt: now().Add(2 * time.Minute)}}
	v, content, err := f.s.FindVersion(ctx, n.ID, 2)
	if err != nil || !reflect.DeepEqual(v, want) || content != "v2" {
		t.Errorf("FindVersion(2) = %+v, %q, %v; want %+v, \"v2\"", v, content, err, want)
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
	if versions, err := f.s.ListVersions(ctx, n.ID, newest, 10); err != nil || len(versions) != 2 {
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
	of := f.open(t, other.ID, f.eng, f.alice, now().Add(time.Minute))
	othersCS := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetEdit, Client: domain.ClientWeb, By: f.alice,
		At: now()}
	if err := f.s.CreateChangeset(ctx, othersCS); err != nil {
		t.Fatal(err)
	}
	if err := f.s.SetSessionWrite(ctx, of.ID, othersCS.ID, 2); err != nil {
		t.Fatal(err)
	}
	none("a session not written, beside another page's written")
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
