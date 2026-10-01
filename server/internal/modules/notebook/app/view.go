package app

import (
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// View is a notebook as the API answers it: with the caller's effective
// role in it and its count of active members.
type View struct {
	Notebook    domain.Notebook
	Role        shared.NotebookRole
	MemberCount int
}
