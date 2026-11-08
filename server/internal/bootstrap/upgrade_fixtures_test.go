package bootstrap

import (
	"context"
	"testing"
	"uuid"
)

// The upgrade's whole-program tests (M11/P1 design 7.1) set an upgrade by
// SQL, as its start will (M11/P1/S1 has no start yet; M10/P1/S2 built its
// guard's skeleton the same way): the notebook's four columns, and a
// baseline row of each node alive, as Capture writes them.

// userOf is the account id of name in tm.
func (tm acmeTeam) userOf(t testing.TB, name string) string {
	t.Helper()
	return queryStrings(t, tm.pool, "SELECT id::text FROM users WHERE email = $1", name+"@example.com")[0]
}

// upgrade sets the notebook nb being upgraded by name's account, in state,
// with a baseline row of each node of nb alive, and answers the upgrade's
// id.
func (tm acmeTeam) upgrade(t testing.TB, nb, by, state string) string {
	t.Helper()
	id := uuid.NewV7().String()
	tm.exec(t, `UPDATE notebooks SET upgrade_id = $2, upgrade_state = $3, upgrade_by_id = $4, upgrade_at = now() WHERE id = $1`,
		nb, id, state, tm.userOf(t, by))
	tm.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT $2, n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision, c.content_hash, c.byte_size,
			CASE WHEN n.kind = 'page' THEN 0 END, c.content_hash
		FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id AND n.kind = 'page'
		WHERE n.notebook_id = $1 AND n.deleted_at IS NULL`, nb, id)
	return id
}

// exec runs a statement of the test's own on tm's database.
func (tm acmeTeam) exec(t testing.TB, sql string, args ...any) {
	t.Helper()
	if _, err := tm.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}
