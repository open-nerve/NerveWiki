// Package notebook is the module of notebooks and their members (v0.1
// design 3.3; M3 design). Its root is what bootstrap sees: NewFacts for the
// access module's facts; Purgers for the purge; Actions for the
// composition's checks.
package notebook

import (
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Actions are the module's actions: bootstrap checks they are the access
// module's rule table.
func Actions() []shared.Action {
	return domain.Actions()
}
