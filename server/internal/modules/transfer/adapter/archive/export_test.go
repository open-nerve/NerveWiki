package archiveadapter

// WithMaxDirectory is a's archives, an import's directory of at most n
// bytes read, for the tests outside the package.
func WithMaxDirectory(a Archives, n int64) Archives {
	a.maxDirectory = n
	return a
}
