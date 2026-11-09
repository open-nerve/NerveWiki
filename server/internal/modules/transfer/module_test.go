package transfer_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// New panics without the rollback's action or end (M11/P1 design 3.19,
// P1 review A-N5): a rollback would start that no admin could cancel, or
// end that ends no upgrade.
func TestNewWantsTheRollbacksActionAndEnd(t *testing.T) {
	for name, d := range map[string]transfer.Deps{
		"no RollbackEnd":    {RollbackAction: "notebook.change_mode"},
		"no RollbackAction": {RollbackEnd: noRollbackEnd{}},
		"neither":           {},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("New() with %s did not panic", name)
				}
			}()
			transfer.New(d)
		}()
	}
}
