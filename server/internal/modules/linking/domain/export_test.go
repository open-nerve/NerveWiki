package domain

// Kept is kept of w's writing again, for the tests to read a writing back
// as Written does.
func (w Rewriting) Kept(was, now Facts, from []Step, tree Tree) bool {
	return kept(was, now, w.Leads, from, tree)
}
