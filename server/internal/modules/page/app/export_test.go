package app

// WithMaxWork is l with its globs' work a call at most maxWork: a test's
// bound, less than domain.MaxGlobWork.
func (l *ListByPattern) WithMaxWork(maxWork int64) *ListByPattern {
	c := *l
	c.maxWork = maxWork
	return &c
}
