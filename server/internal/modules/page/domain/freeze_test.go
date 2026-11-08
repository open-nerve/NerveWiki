package domain_test

import (
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The freeze of a write (M11/P1 design 3.4) by the notebook's upgrade's
// state, the write's client and author, and the upgrade its unit writes
// for: a notebook with no upgrade lets everything by; upgrading asks of
// its initiator's MCP write alone, refusing his web's and API's and
// another's MCP; reviewing and rolling back refuse his MCP write too; the
// upgrade's own unit is let by in each state, a unit of another upgrade,
// or of one the notebook no longer has, is stale.
func TestFreezeAdmits(t *testing.T) {
	initiator, other := uuid.NewV7(), uuid.NewV7()
	current, stale := uuid.NewV7(), uuid.NewV7()
	agent := domain.MCPClient("claude-code")
	in := func(state domain.UpgradeState) domain.NotebookState {
		return domain.NotebookState{Mode: domain.ModeStandard, Upgrade: domain.NotebookUpgrade{ID: current, State: state, By: initiator}}
	}
	none := domain.NotebookState{Mode: domain.ModeStandard}
	for _, tt := range []struct {
		name   string
		n      domain.NotebookState
		own    uuid.UUID
		client domain.Client
		by     uuid.UUID
		want   domain.Freeze
	}{
		{"no upgrade, anyone's web write", none, uuid.UUID{}, domain.ClientWeb, other, domain.FreezeOpen},
		{"no upgrade, anyone's MCP write", none, uuid.UUID{}, agent, other, domain.FreezeOpen},
		{"upgrading, the initiator's MCP write", in(domain.UpgradeUpgrading), uuid.UUID{}, agent, initiator, domain.FreezeAsk},
		{"upgrading, the initiator's web write", in(domain.UpgradeUpgrading), uuid.UUID{}, domain.ClientWeb, initiator, domain.FreezeRefused},
		{"upgrading, the initiator's API write", in(domain.UpgradeUpgrading), uuid.UUID{}, domain.ClientAPI, initiator, domain.FreezeRefused},
		{"upgrading, another's MCP write", in(domain.UpgradeUpgrading), uuid.UUID{}, agent, other, domain.FreezeRefused},
		{"upgrading, another's web write", in(domain.UpgradeUpgrading), uuid.UUID{}, domain.ClientWeb, other, domain.FreezeRefused},
		{"reviewing, the initiator's MCP write", in(domain.UpgradeReviewing), uuid.UUID{}, agent, initiator, domain.FreezeRefused},
		{"rolling back, the initiator's MCP write", in(domain.UpgradeRollingBack), uuid.UUID{}, agent, initiator, domain.FreezeRefused},
		{"upgrading, the upgrade's own unit", in(domain.UpgradeUpgrading), current, domain.ClientWeb, other, domain.FreezeOpen},
		{"rolling back, the upgrade's own unit", in(domain.UpgradeRollingBack), current, domain.ClientWeb, initiator, domain.FreezeOpen},
		{"reviewing, the upgrade's own unit", in(domain.UpgradeReviewing), current, agent, initiator, domain.FreezeOpen},
		{"rolling back, another upgrade's unit", in(domain.UpgradeRollingBack), stale, domain.ClientWeb, initiator, domain.FreezeStale},
		{"upgrading, another upgrade's unit by its initiator's MCP", in(domain.UpgradeUpgrading), stale, agent, initiator, domain.FreezeStale},
		{"no upgrade now, a unit of one", none, stale, domain.ClientWeb, initiator, domain.FreezeStale},
	} {
		if got := domain.FreezeAdmits(tt.n, tt.own, tt.client, tt.by); got != tt.want {
			t.Errorf("%s: FreezeAdmits = %d, want %d", tt.name, got, tt.want)
		}
	}
}
