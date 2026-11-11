package page

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/markdown"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Baseline, SessionHolder and RootNode are what an upgrade's start reads
// of the notebook's pages (M11 design 4.4): the baseline it recorded, an
// edit session alive, a node at the root by its title key.
type (
	Baseline      = app.Baseline
	SessionHolder = app.SessionHolder
	RootNode      = app.KeyedRootNode
)

// UpgradeRoots, BaselineNode and Preserved are what an upgrade's check
// reads of the notebook's pages (M11 design 4.7, M11/P2 design 3.9): its
// root's AGENTS and its proposed source root, its baseline rows, what of
// them it kept.
type (
	UpgradeRoots = app.UpgradeRoots
	BaselineNode = app.BaselineNode
	Preserved    = app.Preserved
)

// ReservedKeys are the title keys no node may take at an LLM Wiki's root
// (M10 design 4.9): an upgrade's check's gate 3 reads them here, not in a
// copy of the wiki module's (M11 design 4.7).
func ReservedKeys() []string { return domain.ReservedKeys() }

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

// Left is what the rollback of the upgrade has left to do in the notebook
// (M11 decision 226): the baseline nodes not in place, the baseline pages
// not written back, the new nodes alive; in the transaction ctx carries,
// when it carries one.
func (u *Upgrades) Left(ctx context.Context, upgradeID, notebookID uuid.UUID) (RollbackLeft, error) {
	return u.store.RollbackLeft(ctx, upgradeID, notebookID)
}

// DeleteBaseline deletes the upgrade's rows, baseline and protected, as
// it ends (M11 design 4.10), in the transaction ctx carries.
func (u *Upgrades) DeleteBaseline(ctx context.Context, upgradeID uuid.UUID) error {
	_, err := u.store.DeleteUpgradeNodes(ctx, upgradeID)
	return err
}

// RootNodes are the nodes alive at the notebook's root whose title keys
// are among keys (M11 design 4.4 step 8).
func (u *Upgrades) RootNodes(ctx context.Context, notebookID uuid.UUID, keys []string) ([]RootNode, error) {
	return u.store.RootNodesByKey(ctx, notebookID, keys)
}

// Roots are gates 1 and 2 of the upgrade's check (M11/P2 design 3.9): the
// page at the notebook's root keyed agents, none for an attachment, and
// whether its body past its frontmatter and a byte order mark, trimmed, is
// not empty, as AgentsBody reads it; whether rootID is a live page of the
// notebook. Each by its key, in the snapshot ctx carries.
func (u *Upgrades) Roots(ctx context.Context, notebookID, rootID uuid.UUID) (UpgradeRoots, error) {
	var out UpgradeRoots
	nodes, err := u.store.RootNodesByKey(ctx, notebookID, []string{shared.TitleKey(domain.AgentsTitle)})
	if err != nil {
		return UpgradeRoots{}, err
	}
	for _, n := range nodes {
		if n.Kind != domain.KindPage {
			continue
		}
		c, err := u.store.PageContent(ctx, n.ID)
		if err != nil {
			return UpgradeRoots{}, err
		}
		out.AgentsID = &n.ID
		out.AgentsBody = strings.TrimSpace(domain.UpgradeBody(c.Content, markdownadapter.FrontmatterEnd(c.Content))) != ""
	}
	root, err := u.store.FindNodeIn(ctx, notebookID, rootID)
	switch {
	case errors.Is(err, app.ErrNotFound):
	case err != nil:
		return UpgradeRoots{}, err
	default:
		out.RootPage = root.Kind == domain.KindPage
	}
	return out, nil
}

// Baseline is the upgrade's baseline rows (M11/P2 design 3.9), in the
// snapshot ctx carries.
func (u *Upgrades) Baseline(ctx context.Context, upgradeID uuid.UUID) ([]BaselineNode, error) {
	return u.store.UpgradeGateBaseline(ctx, upgradeID)
}

// Preserved is gate 7 of the upgrade's check, in one pass (M11/P2 design
// 3.9), in the snapshot ctx carries.
func (u *Upgrades) Preserved(ctx context.Context, upgradeID uuid.UUID) (Preserved, error) {
	return u.store.UpgradePreserved(ctx, upgradeID)
}

// Changesets is how many changesets the upgrade wrote (M11/P2 design 3.9),
// in the snapshot ctx carries.
func (u *Upgrades) Changesets(ctx context.Context, upgradeID uuid.UUID) (int, error) {
	return u.store.UpgradeChangesets(ctx, upgradeID)
}
