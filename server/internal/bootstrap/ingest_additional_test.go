package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

func testIngestKindGate(t *testing.T) {
	for _, kind := range []string{"import", "version_restore", "trash_restore", "revert"} {
		t.Run(kind, func(t *testing.T) {
			f := newIngestFixture(t, nil)
			var cs, id string
			switch kind {
			case "import":
				job := f.tm.imported(t, "alice", f.nb, "", vaultZip(t, vaultFile{"Imported.md", "[[raw/S]]"}))
				if job.State != "succeeded" {
					t.Fatalf("real import did not succeed: %+v", job)
				}
				ids := queryStrings(t, f.tm.pool, "SELECT id::text FROM nodes WHERE notebook_id=$1 AND name='Imported'", f.nb)
				if len(ids) != 1 {
					t.Fatalf("imported IDs %v", ids)
				}
				id = ids[0]
				cs = ingestCS(t, f.tm, id, 1)
			case "version_restore":
				id = f.p
				f.write(t, id, "[[raw/S]]")
				f.write(t, id, "removed")
				f.tm.send(t, versionRestore("alice", id, 2, 3), http.StatusOK)
				cs = ingestCS(t, f.tm, id, 4)
			case "trash_restore":
				id = f.p
				f.write(t, id, "[[raw/S]]")
				f.tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)
				cs = f.tm.restored(t, "alice", id).ChangesetID
			case "revert":
				id = f.p
				f.write(t, id, "[[raw/S]]")
				removed := f.write(t, id, "removed")
				cs = f.tm.reverted(t, "alice", removed)
			}
			if kinds := queryStrings(t, f.tm.pool, "SELECT kind FROM changesets WHERE id=$1", cs); !slices.Equal(kinds, []string{kind}) {
				t.Fatalf("actual operation kind %v, want %s", kinds, kind)
			}
			f.tm.resolves(t, id, f.s)
			assertIngestMarks(t, f.tm, cs)
			// A later ordinary edit on this very page must still mark new T.
			fresh := f.write(t, id, "[[raw/S]] [[raw/T]]")
			assertIngestMarks(t, f.tm, fresh, f.t)
			checkPages(t, f.tm.pool)
			checkLinks(t, f.tm.pool)
		})
	}
}

func testIngestUnknownFollow(t *testing.T) {
	for _, referenced := range []bool{true, false} {
		name := "unknown_f_unreferenced"
		if referenced {
			name = "unknown_follow_closure"
		}
		t.Run(name, func(t *testing.T) {
			head := &ingestMissingOldHead{}
			f := newIngestFixture(t, &wiring{page: func(d *page.Deps) {
				for i, observer := range d.Observers {
					if _, ok := observer.(linkIndex); ok {
						d.Observers[i] = ingestMissingHeadObserver{real: observer, state: head}
					}
				}
			}})
			a := f.tm.upload(t, "alice", f.nb, f.raw, "a.txt", "asset A")
			source := f.tm.createPageWith(t, "alice", f.nb, f.raw, "Follow", "source body [A](a.txt)")
			if referenced {
				f.tm.createPageWith(t, "alice", f.nb, "", "Q", "[[raw/Follow]]")
			}
			// Preserve the real participant's current representation during its
			// plan/rewrite. Then remove only the old head inside its original
			// transaction immediately before linking observes the actual write.
			head.source = source
			head.armed.Store(true)
			cs := f.batch(t, opMove("raw/a.txt", "raw/b.txt"), opWriteOn("P.md", "[A](raw/b.txt) [[raw/T]]", versionOf(f.p, 1)))
			if head.armed.Load() {
				t.Fatal("missing-old-head observer never executed")
			}
			want := []string{f.t}
			if !referenced {
				want = append(want, a.ID)
			}
			assertIngestMarks(t, f.tm, cs, want...)
			f.tm.resolves(t, source, a.ID)
			if content := f.tm.content(t, "alice", source); content.Revision != 2 {
				t.Fatalf("real source participant did not rewrite: %+v", content)
			}
			checkPages(t, f.tm.pool)
			checkLinks(t, f.tm.pool)
		})
	}
}

type ingestMissingOldHead struct {
	armed  atomic.Bool
	source string
}
type ingestMissingHeadObserver struct {
	real  page.PageObserver
	state *ingestMissingOldHead
}

func (o ingestMissingHeadObserver) PagesChanged(ctx context.Context, e page.Event) error {
	if o.state.armed.CompareAndSwap(true, false) {
		tx, ok := postgres.TxFrom(ctx)
		if !ok {
			return fmt.Errorf("missing old head fixture lost real transaction")
		}
		var contentRevision, indexRevision int
		if err := tx.QueryRow(ctx, "SELECT c.revision,i.revision FROM page_contents c JOIN indexed_pages i ON i.node_id=c.node_id WHERE c.node_id=$1", o.state.source).Scan(&contentRevision, &indexRevision); err != nil {
			return err
		}
		if contentRevision != 2 || indexRevision != 1 {
			return fmt.Errorf("missing old head fixture did not reach true system rewrite: content%d index%d", contentRevision, indexRevision)
		}
		tag, err := tx.Exec(ctx, "DELETE FROM indexed_pages WHERE node_id=$1", o.state.source)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("missing old head fixture deleted%d rows", tag.RowsAffected())
		}
	}
	return o.real.PagesChanged(ctx, e)
}
