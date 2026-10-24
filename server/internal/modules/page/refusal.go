package page

import (
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Refusal is the code of err's refusal and the node it is about, when
// err is one a guard or a precheck marked (M10 design 4.6): the
// transfer module's jobs read it through the composition root.
func Refusal(err error) (code string, nodeID uuid.UUID, ok bool) {
	var r *domain.Refused
	var problem *shared.Error
	if !errors.As(err, &r) || !errors.As(r.Err, &problem) {
		return "", uuid.UUID{}, false
	}
	return problem.Code, r.NodeID, true
}
