package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The module's actions (M6/P5 design 7): each has a row in the access
// module's rule table (bootstrap's actions test), all reads of a notebook,
// the landing its writers' alone (M6/P6 design 2).
const (
	// ActionListBacklinks lists the pages that link to a page.
	ActionListBacklinks shared.Action = "backlink.list"
	// ActionReadProperties reads a page's properties and their links.
	ActionReadProperties shared.Action = "page_property.read"
	// ActionListTags lists a notebook's tags.
	ActionListTags shared.Action = "tag.list"
	// ActionReadTag lists the pages with a tag.
	ActionReadTag shared.Action = "tag.read"
	// ActionListLinkTargets lists what a notebook's links may lead to, for
	// the editor's completion.
	ActionListLinkTargets shared.Action = "link_target.list"
	// ActionReadLinkLanding reads where a page made for a link would go.
	ActionReadLinkLanding shared.Action = "link_landing.read"
)

// Actions lists the module's actions.
func Actions() []shared.Action {
	return []shared.Action{ActionListBacklinks, ActionReadProperties, ActionListTags, ActionReadTag, ActionListLinkTargets, ActionReadLinkLanding}
}
