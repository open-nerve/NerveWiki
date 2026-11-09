package page

import (
	"context"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
)

// Baseline, SessionHolder and RootNode are what an upgrade's start reads
// of the notebook's pages (M11 design 4.4): the baseline it recorded, an
// edit session alive, a node at the root by its title key.
type (
	Baseline      = app.Baseline
	SessionHolder = app.SessionHolder
	RootNode      = app.KeyedRootNode
)

// Upgrades are the page module's part in an upgrade's start (M11/P1
// design 3.12), built from the pool alone (v0.1 design 13.1 item 11):
// each method runs in the transaction ctx carries, the start's, which
// holds the notebook's row FOR NO KEY UPDATE; Analyze runs after it.
type Upgrades struct {
	store *postgresadapter.Store
}

// NewUpgrades is over pool.
func NewUpgrades(pool *pgxpool.Pool) *Upgrades {
	return &Upgrades{store: postgresadapter.New(pool)}
}

// Capture records the upgrade's baseline of the notebook, a row for each
// node alive, and answers its nodes and pages (M11 design 4.4 step 11).
func (u *Upgrades) Capture(ctx context.Context, upgradeID, notebookID uuid.UUID) (Baseline, error) {
	return u.store.CaptureBaseline(ctx, upgradeID, notebookID)
}

// Protect keeps the source root rootID the upgrade's start created from a
// delete (M11 design 4.4 step 12).
func (u *Upgrades) Protect(ctx context.Context, upgradeID, notebookID, rootID uuid.UUID) error {
	return u.store.ProtectSourceRoot(ctx, upgradeID, notebookID, rootID)
}

// SetWanted sets each baseline page's count of the distinct wanted targets
// it links to (M11 design 4.4 step 13), in one statement by id; none for
// an empty map. A page not of the baseline changes nothing.
func (u *Upgrades) SetWanted(ctx context.Context, upgradeID uuid.UUID, wanted map[uuid.UUID]int) error {
	if len(wanted) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(wanted))
	for id := range wanted {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	counts := make([]int32, len(ids))
	for i, id := range ids {
		counts[i] = int32(wanted[id])
	}
	return u.store.SetBaselineWanted(ctx, upgradeID, ids, counts)
}

// Analyze analyzes the upgrades' rows, after the start committed (M11
// design 4.4 step 16).
func (u *Upgrades) Analyze(ctx context.Context) error { return u.store.AnalyzeUpgradeNodes(ctx) }

// ClearExpiredSessions deletes the notebook's edit sessions expired at at,
// tombstones among them: a heartbeat that read an earlier time then finds
// no row to keep alive (M11 design 4.4 step 7).
func (u *Upgrades) ClearExpiredSessions(ctx context.Context, notebookID uuid.UUID, at time.Time) error {
	_, err := u.store.DeleteExpiredSessionsOfNotebook(ctx, notebookID, at)
	return err
}

// LiveSessions are the first 100 edit sessions alive in the notebook at
// at, by page, and how many there are (M11 design 4.4 step 7).
func (u *Upgrades) LiveSessions(ctx context.Context, notebookID uuid.UUID, at time.Time) ([]SessionHolder, int, error) {
	return u.store.LiveSessionsOfNotebook(ctx, notebookID, at)
}

// RootNodes are the nodes alive at the notebook's root whose title keys
// are among keys (M11 design 4.4 step 8).
func (u *Upgrades) RootNodes(ctx context.Context, notebookID uuid.UUID, keys []string) ([]RootNode, error) {
	return u.store.RootNodesByKey(ctx, notebookID, keys)
}
