package httpadapter

import "github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"

// contentBodyLimit is a body that holds a page's content: its largest, each
// byte written as JSON's longest escape of one byte, "\u00XX", and room
// for the other members (M4/P4 design 3.8).
const contentBodyLimit = 6*domain.MaxContentBytes + 64<<10

// BodyLimits are the module's routes whose body may be larger than
// server.max_body_bytes, with their limit: the two that take a content.
func BodyLimits() map[string]int64 {
	return map[string]int64{
		"PUT /api/v0/pages/{page_id}/content":        contentBodyLimit,
		"POST /api/v0/notebooks/{notebook_id}/pages": contentBodyLimit,
	}
}
