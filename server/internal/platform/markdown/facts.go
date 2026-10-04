package markdown

import "math"

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
// link, some 300 bytes; its scalar's path aside, which repeats the keys
// above it, and is counted by its bytes. factsBase is what the facts of
// any content keep beyond them: a small content's slices grow by doubling
// (M6/P2 fix check 2 L2).
const (
	factsPerValue = 400
	factsBase     = 4 << 10
)

// Limit is the most f, the facts of a content of n bytes, keep: factsBase,
// FactsRatio times n, factsPerValue for each value of the frontmatter,
// whose count the YAML's limit bounds, and the bytes of its strings' paths,
// which it bounds too (markdowntest.CheckCosts). A size past
// math.MaxInt/(2*FactsRatio) counts as that, the limit not wrapping
// around (M6/P2 fix check 2 L5).
func (f Facts) Limit(n int) int {
	n = min(max(n, 0), math.MaxInt/(2*FactsRatio))
	return factsBase + FactsRatio*n + factsPerValue*values(f.frontmatter.Properties) + paths(f.frontmatter.Scalars)
}

// paths is how many bytes the paths of scalars take.
func paths(scalars []Scalar) int {
	n := 0
	for _, s := range scalars {
		n += len(s.Path)
	}
	return n
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
