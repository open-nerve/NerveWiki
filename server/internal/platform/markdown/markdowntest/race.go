//go:build race

package markdowntest

// Race tells that the race detector is on: it makes the code several
// times slower, so the checks of costs, and of HTML too large to render
// often under it, are made in a build without (make test-go).
const Race = true
