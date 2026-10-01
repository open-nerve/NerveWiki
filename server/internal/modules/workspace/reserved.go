package workspace

import "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"

// ReservedSlugs are the names no workspace may take, by why (M2/P1 design
// 3.6): bootstrap checks the server's against the paths it serves.
type ReservedSlugs = domain.ReservedSlugs

// Reserved returns the list of reserved slugs.
func Reserved() ReservedSlugs {
	return domain.Reserved()
}
