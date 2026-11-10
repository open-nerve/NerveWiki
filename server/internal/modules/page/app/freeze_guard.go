package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Frontmatters tell where a page's frontmatter ends (Markdown.FrontmatterEnd).
type Frontmatters interface{ FrontmatterEnd(content string) int }

// UpgradeNodes are an upgrade's rows the freeze reads, in the transaction
// ctx carries.
type UpgradeNodes interface {
	// Protected are those of ids that have a row of the upgrade upgradeID:
	// its baseline's and its protected source root's.
	Protected(ctx context.Context, upgradeID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error)
	// BaselineContent is the content now of the page id when it is of the
	// upgrade's baseline, false when it is not (M11 design 4.6): read under
	// the page's content row lock, which the unit holds as it writes it.
	BaselineContent(ctx context.Context, upgradeID, id uuid.UUID) (string, bool, error)
}

// FreezeGuard keeps an upgrading notebook's content read-only (M11 design
// 4.5): a guard of the writes and a vetoer of the openings, first of both.
// A write of a notebook with no upgrade, or of the upgrade's own unit, it
// lets by at once, reading nothing. The initiator's agent's writes it
// admits it holds to the guardrails (M11 design 4.6).
type FreezeGuard struct {
	auth   shared.Authorizer
	admin  shared.Action
	nodes  UpgradeNodes
	md     Frontmatters
	admits func(n domain.NotebookState, own uuid.UUID, client domain.Client, by uuid.UUID) domain.Freeze
}

// NewFreezeGuard is the guard over auth, the authorizer the units decide
// with, asking admin of the initiator's MCP writes while upgrading: the
// notebook module's notebook.change_mode, which bootstrap hands it; nodes
// read the upgrade's rows its guardrails keep, md where a page's
// frontmatter ends.
func NewFreezeGuard(auth shared.Authorizer, admin shared.Action, nodes UpgradeNodes, md Frontmatters) *FreezeGuard {
	return &FreezeGuard{auth: auth, admin: admin, nodes: nodes, md: md, admits: domain.FreezeAdmits}
}

// GuardWrite judges s by the upgrade its unit read under the notebook's
// lock (domain.FreezeAdmits): a stale unit of an upgrade is
// ErrStaleUpgradeWrite, a defect's error; a refused write
// notebook.upgrading, which the unit marks with the step's first node; the
// initiator's MCP write while upgrading asks admin once a unit (Grants),
// notebook.upgrading when it is not granted, the initiator no admin now;
// granted, it is held to the guardrails. A freeze it does not know is a
// defect's error: it lets nothing by it was not told to. Any other error,
// the database's, is answered as it is.
func (g *FreezeGuard) GuardWrite(ctx context.Context, s Step) error {
	switch f := g.admits(s.Notebook, s.UpgradeOwn, s.Client, s.By); f {
	case domain.FreezeOpen:
		return nil
	case domain.FreezeStale:
		return domain.ErrStaleUpgradeWrite
	case domain.FreezeRefused:
		return domain.ErrUpgrading
	case domain.FreezeAsk:
	default:
		return fmt.Errorf("page: freeze %d unknown", f)
	}
	if err := s.Grants.Ask(ctx, g.admin, decideAction(g.auth, s, g.admin)); err != nil {
		if errors.Is(err, shared.Forbidden()) {
			return domain.ErrUpgrading
		}
		return err
	}
	return g.guardrails(ctx, s)
}

// guardrails are the initiator's agent's limits (M11 design 4.6): a
// delete of a node the upgrade started with or of its seeded source root
// (P1's, deletion); a move or a rename that leaves the links to it as they
// are, page.upgrade_keep_links marked with the step's root; a write of a
// page of the baseline whose body, past its frontmatter, is not what it
// was before the step, but for the system's rewrite of its links (body).
func (g *FreezeGuard) guardrails(ctx context.Context, s Step) error {
	switch s.Operation {
	case domain.OpDelete:
		return g.deletion(ctx, s)
	case domain.OpMove, domain.OpRename:
		if !s.Options.UpdateLinks || s.KeepLinks {
			return domain.Refuse(domain.ErrUpgradeKeepLinks, s.Changes[0].NodeID)
		}
	case domain.OpContent:
		if !s.Rewrite {
			return g.body(ctx, s)
		}
	}
	return nil
}

// body refuses a write of a page of the baseline whose body is not the one
// before the step, marked with the page: page.upgrade_batch_rewrite when a
// batch folded a move's rewrite into it, page.upgrade_immutable otherwise
// (M11 design 4.6). It compares the body of the content the page holds
// now, under the lock of its content row the unit took, with the step's,
// which the unit hashed (StepContent.BodyHash). A page not of the baseline
// it lets by, reading no content; a step without the body's hash is a
// defect's error.
func (g *FreezeGuard) body(ctx context.Context, s Step) error {
	id := s.Changes[0].NodeID
	if s.Content == nil || s.Content.BodyHash == nil {
		return fmt.Errorf("page: a content step of page %s while upgrading carries no body hash", id)
	}
	before, baseline, err := g.nodes.BaselineContent(ctx, s.Notebook.Upgrade.ID, id)
	if err != nil || !baseline {
		return err
	}
	if bytes.Equal(domain.UpgradeBodyHash(before, g.md.FrontmatterEnd(before)), s.Content.BodyHash) {
		return nil
	}
	if s.Content.Folded {
		return domain.Refuse(domain.ErrUpgradeBatchRewrite, id)
	}
	return domain.Refuse(domain.ErrUpgradeImmutable, id)
}

// deletion keeps every node the upgrade started with and the source root
// its start created (M11/P1), page.upgrade_protected marked with the first
// of them in the subtree's order (Changes, top-down), not the subtree's
// root, which may be a node the agent created; a node created since may
// go.
func (g *FreezeGuard) deletion(ctx context.Context, s Step) error {
	ids := make([]uuid.UUID, len(s.Changes))
	for i, c := range s.Changes {
		ids[i] = c.NodeID
	}
	protected, err := g.nodes.Protected(ctx, s.Notebook.Upgrade.ID, ids)
	if err != nil {
		return err
	}
	for _, c := range s.Changes {
		if protected[c.NodeID] {
			return domain.Refuse(domain.ErrUpgradeProtected, c.NodeID)
		}
	}
	return nil
}

// VetoEditSession refuses every opening in a notebook with an upgrade
// under way, whatever its state (M11 design 4.5): MCP opens no session,
// and the initiator's web is read-only as everyone's is.
func (g *FreezeGuard) VetoEditSession(_ context.Context, o SessionOpening) error {
	if o.Notebook.Upgrade.ID != (uuid.UUID{}) {
		return domain.ErrUpgrading
	}
	return nil
}
