package page

// GrepTurns are how many calls m's grep lets read and match at once, and
// how many patterns it lets parse at once (M9 closeout A-F3).
func (m *Module) GrepTurns() (slots, parses int) {
	return m.grep.Turns()
}
