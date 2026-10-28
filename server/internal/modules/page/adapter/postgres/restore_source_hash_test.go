package postgresadapter_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The port's optional values distinguish absent from explicit NULL. Seed
// stale hashes deliberately: normal trash has NULL, which cannot detect an
// accidental clear of an absent value. IDs are supplied out of creation order.
func TestRestoreNodesMaintainsOptionalSourceHashes(t *testing.T) {
	for _, mode := range []string{"nil map", "empty map", "mixed values"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			at := now()
			var nodes []domain.Node
			for i, name := range []string{"Absent", "Clear", "First hash", "Second hash", "Other time", "Already live", "Outside IDs"} {
				n := f.page(t, f.eng, nil, name, float64(i))
				f.versions(t, n.ID, 2)
				f.exec(t, "UPDATE page_contents SET revision = 2 WHERE node_id = $1", n.ID)
				if i != 5 {
					deletedAt := at
					if i == 4 {
						deletedAt = at.Add(-time.Minute)
					}
					f.trashed(t, deletedAt, 1, 0, 1, n.ID)
				}
				f.exec(t, "UPDATE nodes SET source_hash = $2 WHERE id = $1", n.ID, sourceSum("old "+name))
				nodes = append(nodes, n)
			}
			missing := domain.Node{ID: uuid.NewV7(), Name: "Missing", NameKey: "missing"}
			back := restoration(at, nil, 7, f.alice, nodes[3], nodes[0], nodes[5], nodes[1], missing, nodes[4], nodes[2])
			if mode != "nil map" {
				back.SourceHashes = map[uuid.UUID][]byte{}
			}
			if mode == "mixed values" {
				back.SourceHashes = map[uuid.UUID][]byte{
					nodes[1].ID: nil, nodes[2].ID: sourceSum("first"), nodes[3].ID: sourceSum("second"),
					nodes[4].ID: sourceSum("wrong time"), nodes[5].ID: nil, missing.ID: sourceSum("missing"),
					// A value for an ID outside this restoration is never used.
					nodes[6].ID: {1},
				}
			}
			rows, err := f.s.RestoreNodes(t.Context(), back)
			if want := (app.RestoredRows{Nodes: 4, Contents: 4, CurrentVersions: 4, Creations: 4}); err != nil || rows != want {
				t.Fatalf("RestoreNodes = %+v, %v; want %+v", rows, err, want)
			}
			for i, n := range nodes {
				want := sourceSum("old " + n.Name)
				if mode == "mixed values" {
					switch i {
					case 1:
						want = nil
					case 2:
						want = sourceSum("first")
					case 3:
						want = sourceSum("second")
					}
				}
				if got := f.sourceHash(t, n.ID); !bytes.Equal(got, want) {
					t.Errorf("%s hash = %x, want %x", n.Name, got, want)
				}
				alive := i < 4 || i == 5
				if got := f.count(t, "SELECT count(*) FROM nodes WHERE id = $1 AND (deleted_at IS NULL) = $2", n.ID, alive); got != 1 {
					t.Errorf("%s live state differs from %t", n.Name, alive)
				}
				if i < 4 && (f.count(t, "SELECT count(*) FROM page_contents WHERE node_id = $1 AND deleted_at IS NULL", n.ID) != 1 ||
					f.count(t, "SELECT count(*) FROM page_revisions WHERE node_id = $1 AND deleted_at IS NULL", n.ID) != 2) {
					t.Errorf("%s did not restore its content and both versions", n.Name)
				}
			}
			if f.count(t, "SELECT count(*) FROM nodes WHERE id = $1", missing.ID) != 0 {
				t.Error("restoration manufactured a missing node")
			}
		})
	}
}

func TestRestoreNodesRejectsMalformedSourceHashesWithoutChangingRows(t *testing.T) {
	for _, size := range []int{0, 31, 33} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			f := newFixture(t)
			root := f.page(t, f.eng, nil, "Root", 0)
			child := f.page(t, f.eng, &root.ID, "Child", 0)
			f.trashed(t, now(), 2, 0, 2, root.ID, child.ID)
			before := restorationRows(t, f)
			back := restoration(now(), nil, 0, f.alice, root, child)
			back.SourceHashes = map[uuid.UUID][]byte{root.ID: sourceSum("valid first"), child.ID: make([]byte, size)}
			if _, err := f.s.RestoreNodes(t.Context(), back); err == nil {
				t.Fatal("malformed non-NULL hash was accepted")
			}
			if !maps.Equal(restorationRows(t, f), before) {
				t.Error("malformed hash changed rows before refusing")
			}
		})
	}
}

func TestRestoreNodesRollsBackHashesWithTheGroupAndHistory(t *testing.T) {
	f := newFixture(t)
	root := f.page(t, f.eng, nil, "Root", 0)
	child := f.page(t, f.eng, &root.ID, "Child", 0)
	cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: "edit", Client: domain.ClientAPI, By: f.alice, At: now()}
	if err := f.s.CreateChangeset(t.Context(), cs); err != nil {
		t.Fatal(err)
	}
	for _, n := range []domain.Node{root, child} {
		f.versions(t, n.ID, 2)
		f.exec(t, "UPDATE page_contents SET revision = 2 WHERE node_id = $1", n.ID)
		state := n.State()
		if err := f.s.RecordItems(t.Context(), []app.Item{{ID: uuid.NewV7(), ChangesetID: cs.ID,
			Change: domain.Change{NodeID: n.ID, After: &state}, At: now()}}); err != nil {
			t.Fatal(err)
		}
	}
	f.trashed(t, now(), 2, 0, 2, root.ID, child.ID)
	before := restorationRows(t, f)
	back := restoration(now(), nil, 4, f.alice, root, child)
	back.SourceHashes = map[uuid.UUID][]byte{root.ID: sourceSum("root"), child.ID: sourceSum("child")}
	failure := errors.New("fail after restored rows")
	err := postgres.NewTxManager(f.pool, time.Second).WithinTx(t.Context(), func(ctx context.Context) error {
		rows, err := f.s.RestoreNodes(ctx, back)
		if err != nil {
			return err
		}
		if want := (app.RestoredRows{Nodes: 2, Contents: 2, CurrentVersions: 2, Creations: 2}); rows != want {
			return fmt.Errorf("RestoreNodes = %+v; want %+v", rows, want)
		}
		var nodes, contents, versions, items int
		if err := postgres.DB(ctx, f.pool).QueryRow(ctx, `SELECT
			(SELECT count(*) FROM nodes WHERE deleted_at IS NULL AND
				((id = $1 AND source_hash = $3) OR (id = $2 AND source_hash = $4))),
			(SELECT count(*) FROM page_contents WHERE deleted_at IS NULL),
			(SELECT count(*) FROM page_revisions WHERE deleted_at IS NULL),
			(SELECT count(*) FROM changeset_items WHERE deleted_at IS NULL)`,
			root.ID, child.ID, sourceSum("root"), sourceSum("child")).Scan(&nodes, &contents, &versions, &items); err != nil {
			return err
		}
		if nodes != 2 || contents != 2 || versions != 4 || items != 2 {
			return fmt.Errorf("restored transaction rows = %d/%d/%d/%d, want 2/2/4/2", nodes, contents, versions, items)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("rollback = %v, want injected failure after observed writes", err)
	}
	if !maps.Equal(restorationRows(t, f), before) {
		t.Error("rollback changed node hashes, content or history")
	}
}

func restorationRows(t *testing.T, f fixture) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range []string{"nodes", "page_contents", "page_revisions", "changeset_items"} {
		var rows string
		if err := f.pool.QueryRow(t.Context(), "SELECT coalesce(string_agg(to_jsonb(x)::text, E'\\n' ORDER BY to_jsonb(x)::text), '') FROM "+table+" x").Scan(&rows); err != nil {
			t.Fatal(err)
		}
		out[table] = rows
	}
	return out
}
