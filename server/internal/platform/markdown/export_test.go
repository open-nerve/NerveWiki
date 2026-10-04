package markdown

// JSONOf is jsonOf, for the tests outside the package.
func JSONOf(v any) any { return jsonOf(v) }

// FactsBase is what the facts of any content keep beyond the rest,
// ParseRatio the times its facts a parse holds, and MinTake the least a
// content takes.
const (
	FactsBase  = factsBase
	ParseRatio = parseRatio
	MinTake    = minTake
)

// SourceOf is the text d's parser read.
func SourceOf(d *Document) []byte { return d.source }

// Words is an extension's test double: it takes the words written
// "@@word@@", and renders each in a mark with what it fetched for the page.
func Words() Extension { return wordsRendered(fetchFor) }
