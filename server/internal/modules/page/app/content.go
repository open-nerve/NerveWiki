package app

import "github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"

// parsed checks content as field and parses it: what a write of it does
// before its transaction (M4 design 4, "parse timing"). Its 422 comes
// before the 404 and the 403, which the unit answers: it tells nothing of
// the page (M4 design 5, "the order of the codes").
func parsed(md Markdown, field, content string) (Parsed, error) {
	if err := domain.CheckContent(field, content); err != nil {
		return nil, err
	}
	return md.Parse(content), nil
}
