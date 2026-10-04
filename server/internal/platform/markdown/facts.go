package markdown

// Facts is what a parse found that outlives its tree (M6 design 4.7): the
// frontmatter and what each extension took, neither the tree nor the
// content. A write keeps them through its unit, where its Document would
// hold some 300 times its content at worst.
type Facts struct {
	frontmatter Frontmatter
	extracted   map[string]any
}

// Frontmatter is the content's frontmatter.
func (f Facts) Frontmatter() Frontmatter { return f.frontmatter }

// Extracted is what the extension named name took, nil for none.
func (f Facts) Extracted(name string) any { return f.extracted[name] }
