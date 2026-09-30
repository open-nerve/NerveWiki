//go:build race

package bodyshape

// raceEnabled tells the tests that the race detector is on. It makes the
// code allocate several times more, so allocation budgets are checked only
// in a build without it (make test runs one).
const raceEnabled = true
