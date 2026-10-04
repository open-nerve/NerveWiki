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

// factsPerValue is the most the facts keep for a value of the frontmatter
// beyond FactsRatio times the content (M6/P2 fix check M-1): a value
// written in two bytes keeps its property, its scalar and maybe a property
// link, some 300 bytes.
const factsPerValue = 400

// Limit is the most f, the facts of a content of n bytes, keep:
// FactsRatio times n, and factsPerValue for each value of the frontmatter,
// whose count the YAML's limit bounds (markdowntest.CheckCosts).
func (f Facts) Limit(n int) int {
	return FactsRatio*max(n, 0) + factsPerValue*values(f.frontmatter.Properties)
}

// values is how many values props hold, each property one, its value's
// items and nested properties too.
func values(props []Property) int {
	n := 0
	for _, p := range props {
		n += 1 + valuesIn(p.Value)
	}
	return n
}

func valuesIn(v any) int {
	switch v := v.(type) {
	case []any:
		n := len(v)
		for _, item := range v {
			n += valuesIn(item)
		}
		return n
	case []Property:
		return values(v)
	}
	return 0
}
