package httpadapter

import "github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"

// ImportPolicy is importPolicy, for the tests.
func ImportPolicy(limits Limits) httpserver.StreamPolicy { return importPolicy(limits) }
