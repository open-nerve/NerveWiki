//go:build race

package markdown_test

// raceEnabled tells the tests that the race detector is on. It makes the
// code several times slower, so costs are checked only in a build without
// it (make test runs one).
const raceEnabled = true
