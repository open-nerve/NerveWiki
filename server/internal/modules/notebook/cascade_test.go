package notebook_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The registrants of a workspace membership's end and restore on a real
// database (M3/P3 design 3.2), built as bootstrap builds them and run in a
// transaction as the workspace module runs them: each tells the
// visibility's subscriber after its writes, in that transaction, and the
// subscriber's failure rolls the writes back.

// noSlugs fails: rule two names no workspace where nothing is refused.
type noSlugs struct{}

func (noSlugs) Slugs(context.Context, []uuid.UUID) (map[uuid.UUID]string, error) {
	return nil, errors.New("no workspace is to be named")
}

// inTx runs f in a transaction of the fixture's pool.
func (f fixture) inTx(fn func(ctx context.Context) error) error {
	return postgres.NewTxManager(f.pool, 5*time.Second).WithinTx(context.Background(), fn)
}

// The end of alice's membership of acme, by herself: rule two lets it
// through, eng alone hers beside bob's ended membership; her membership of
// eng ends and eng is ownerless when the subscriber is told of her.
func TestTheMembershipEndTellsTheVisibility(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "the subscriber follows", true: "the subscriber fails"}[fail], func(t *testing.T) {
			f := newFixture(t)
			probe := "SELECT (n.ownerless_since IS NOT NULL)::text || ' ' || (m.ended_at IS NOT NULL)::text FROM notebooks n " +
				"JOIN notebook_members m ON m.notebook_id = n.id AND m.user_id = '" + f.alice.String() + "' WHERE n.id = '" + f.eng.String() + "'"
			w := &watcher{pool: f.pool, probe: probe, fail: fail}
			end := notebook.NewMembershipEnd(f.pool, noSlugs{}, []notebook.VisibilitySubscriber{w})
			e := notebook.WorkspaceMembershipEnd{UserID: f.alice, WorkspaceIDs: []uuid.UUID{f.acme}, Voluntary: true, By: f.alice, At: testNow()}

			err := f.inTx(func(ctx context.Context) error {
				if err := end.VetoMembershipEnd(ctx, e); err != nil {
					return err
				}
				return end.MembershipEnded(ctx, e)
			})

			want := notebook.VisibilityChange{WorkspaceID: f.acme, UserIDs: []uuid.UUID{f.alice}, At: testNow()}
			if len(w.got) != 1 || !sameChange(w.got[0], want) || len(w.seen) != 1 || w.seen[0] != "true true" {
				t.Errorf("told %+v, seeing %q; want %+v, seeing eng ownerless, alice's membership ended", w.got, w.seen, want)
			}
			committed := "true true"
			if fail {
				committed = "false false"
			}
			var got string
			if err := f.pool.QueryRow(context.Background(), probe).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if (err != nil) != fail || got != committed {
				t.Errorf("the end: %v, then %q; want an error %v, %q", err, got, fail, committed)
			}
		})
	}
}

// Alice back in acme as a guest: eng, ownerless of her, is hers again,
// recorded, when the subscriber is told of her, though a guest's role
// reaches no notebook by its access.
func TestTheMembershipRestoreTellsTheVisibility(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "the subscriber follows", true: "the subscriber fails"}[fail], func(t *testing.T) {
			f := newFixture(t)
			f.orphanEng(t)
			probe := "SELECT (n.ownerless_since IS NULL)::text || ' ' || m.role || ' ' || (m.ended_at IS NULL)::text || ' ' || " +
				"(SELECT count(*) FROM notebook_audit_events WHERE action = 'returned')::text FROM notebooks n " +
				"JOIN notebook_members m ON m.notebook_id = n.id AND m.user_id = '" + f.alice.String() + "' WHERE n.id = '" + f.eng.String() + "'"
			w := &watcher{pool: f.pool, probe: probe, fail: fail}
			restore := notebook.NewMembershipRestore(f.pool, []notebook.VisibilitySubscriber{w})
			r := notebook.WorkspaceMembershipRestore{WorkspaceID: f.acme, UserID: f.alice, Role: shared.WorkspaceGuest, By: f.alice, At: testNow()}

			var returned int
			err := f.inTx(func(ctx context.Context) error {
				var err error
				returned, err = restore.MembershipRestored(ctx, r)
				return err
			})

			want := notebook.VisibilityChange{WorkspaceID: f.acme, UserIDs: []uuid.UUID{f.alice}, At: testNow()}
			if len(w.got) != 1 || !sameChange(w.got[0], want) || len(w.seen) != 1 || w.seen[0] != "true admin true 1" {
				t.Errorf("told %+v, seeing %q; want %+v, seeing eng returned and recorded", w.got, w.seen, want)
			}
			committed, wantReturned := "true admin true 1", 1
			if fail {
				committed, wantReturned = "false admin false 0", 0
			}
			var got string
			if err := f.pool.QueryRow(context.Background(), probe).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if (err != nil) != fail || got != committed || returned != wantReturned {
				t.Errorf("the restore: %d returned, %v, then %q; want %d, an error %v, %q", returned, err, got, wantReturned, fail, committed)
			}
		})
	}
}
