package markdown

// JSONOf is jsonOf, for the tests outside the package.
func JSONOf(v any) any { return jsonOf(v) }

// FactsBase is what the facts of any content keep beyond the rest, and
// ParseRatio the times its facts a parse holds.
const (
	FactsBase  = factsBase
	ParseRatio = parseRatio
)

// SourceOf is the text d's parser read.
func SourceOf(d *Document) []byte { return d.source }

// Words is an extension's test double: it takes the words written
// "@@word@@", and renders each in a mark with what it fetched for the page.
func Words() Extension { return wordsRendered(fetchFor) }
