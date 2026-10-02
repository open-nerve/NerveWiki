package markdown

// JSONOf is jsonOf, for the tests outside the package.
func JSONOf(v any) any { return jsonOf(v) }

// Words is an extension's test double: it takes the words written
// "@@word@@", and renders each in a mark with what it fetched for the page.
func Words() Extension { return wordsRendered(fetchFor) }
