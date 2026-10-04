package markdown

import (
	"bytes"
	"errors"
	"io"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	// maxYAMLNodes is how many values a frontmatter may hold once its
	// aliases are expanded: a line of aliases must not make every parse
	// and every reading blow up (M4 design 4, "frontmatter").
	maxYAMLNodes = 10000
	// maxYAMLDepth is how deep its values may nest.
	maxYAMLDepth = 64
	// minYAMLRepeated is how many bytes of keys and scalars aliases may
	// repeat however short the YAML: a few aliases of a long value must not
	// make every reading blow up either. Past the larger of it and the
	// YAML's size the frontmatter is not valid.
	minYAMLRepeated = 100_000
	// minYAMLPaths, yamlPathsRatio and maxYAMLPathsRatio are how many
	// bytes the paths of the noted strings may take, each its keys over
	// again (M6/P2 fix check 2 C1): long keys over a list of thousands must
	// not copy them for each item either. Past the larger of minYAMLPaths,
	// a path of a hundred bytes for each value at the limit of values (fix
	// check 3 L3), and twice the YAML's size, the frontmatter is not valid;
	// nor past maxYAMLPathsRatio times its size, a path of some 128 bytes
	// for each value written in two, so that a short YAML's paths are no
	// more for its size than the parse budget counts it for (fix check 4
	// L1): with aliases at the limit of values beside them, a content of
	// 4 KB peaks at some 0.95 of what its take counts (fix check 5), so 64
	// is the most. Twice: the keys an alias repeats are in the paths of the
	// values under them too.
	minYAMLPaths      = 100 * maxYAMLNodes
	yamlPathsRatio    = 2
	maxYAMLPathsRatio = 64
)

var errInvalid = errors.New("not a frontmatter's YAML")

// properties parses a frontmatter's YAML into its properties: a mapping,
// read by YAML 1.2's core schema with its aliases expanded (the fixtures'
// rule 1). Empty YAML is a frontmatter without properties; anything else
// that is not a mapping, or goes past the limits, is not valid. The YAML
// library's errors are dropped: they may quote the content. It also gives
// the strings written on one line, src being the content's from at.
func properties(src []byte, at int) ([]Property, []Scalar, bool) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var doc yaml.Node
	switch err := dec.Decode(&doc); {
	case errors.Is(err, io.EOF):
		return nil, nil, true
	case err != nil:
		return nil, nil, false
	}
	var more yaml.Node
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return nil, nil, false // a second document, or an error after the first
	}
	if len(doc.Content) != 1 {
		return nil, nil, false
	}
	top := doc.Content[0]
	if top.Kind != yaml.MappingNode {
		return nil, nil, false
	}
	r := reader{
		budget: max(len(src), minYAMLRepeated), pathsBudget: pathsBudget(len(src)),
		src: src, at: at,
	}
	v, err := r.value(top, 0)
	if err != nil {
		return nil, nil, false
	}
	return v.([]Property), r.scalars, true
}

// pathsBudget is how many bytes the paths of a YAML of n bytes may take.
func pathsBudget(n int) int {
	return min(maxYAMLPathsRatio*n, max(yamlPathsRatio*n, minYAMLPaths))
}

// reader walks the YAML's nodes, counting what it expands: the nodes, and
// the bytes of keys and scalars an alias repeats, up to budget. It notes
// the strings written on one line on the way (scalars.go), counting the
// bytes of their paths, up to pathsBudget.
type reader struct {
	nodes, aliased, repeated, budget int
	paths, pathsBudget               int

	src     []byte   // the YAML
	at      int      // where src starts in the content
	lines   []int    // where each line of src starts, once a scalar needs them
	last    position // where the last scalar looked up starts
	path    []string // the keys and indexes down to the value being read
	scalars []Scalar
}

func (r *reader) value(n *yaml.Node, depth int) (any, error) {
	r.nodes++
	if r.nodes > maxYAMLNodes || depth > maxYAMLDepth {
		return nil, errInvalid
	}
	switch n.Kind {
	case yaml.AliasNode:
		r.aliased++
		defer func() { r.aliased-- }()
		return r.value(n.Alias, depth)
	case yaml.ScalarNode:
		if err := r.repeat(n); err != nil {
			return nil, err
		}
		v, err := scalar(n)
		if s, ok := v.(string); ok && err == nil && r.aliased == 0 {
			err = r.note(n, s)
		}
		return v, err
	case yaml.SequenceNode:
		if !tagIs(n, "!!seq") {
			return nil, errInvalid
		}
		items := make([]any, 0, len(n.Content))
		for i, c := range n.Content {
			r.path = append(r.path, strconv.Itoa(i))
			v, err := r.value(c, depth+1)
			r.path = r.path[:len(r.path)-1]
			if err != nil {
				return nil, err
			}
			items = append(items, v)
		}
		return items, nil
	case yaml.MappingNode:
		if !tagIs(n, "!!map") {
			return nil, errInvalid
		}
		props := make([]Property, 0, len(n.Content)/2)
		seen := map[string]bool{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, err := r.key(n.Content[i])
			if err != nil {
				return nil, err
			}
			if seen[key] {
				return nil, errInvalid
			}
			seen[key] = true
			r.path = append(r.path, key)
			v, err := r.value(n.Content[i+1], depth+1)
			r.path = r.path[:len(r.path)-1]
			if err != nil {
				return nil, err
			}
			props = append(props, Property{Key: key, Value: v})
		}
		return props, nil
	}
	return nil, errInvalid
}

// key is a mapping key as text: a scalar's value written as JSON writes it
// as a key ("10" for 010, "true", "null").
func (r *reader) key(n *yaml.Node) (string, error) {
	r.nodes++
	if n.Kind == yaml.AliasNode {
		r.aliased++
		defer func() { r.aliased-- }()
		n = n.Alias
	}
	if n.Kind != yaml.ScalarNode {
		return "", errInvalid
	}
	if err := r.repeat(n); err != nil {
		return "", err
	}
	v, err := scalar(n)
	if err != nil {
		return "", err
	}
	switch v := v.(type) {
	case nil:
		return "null", nil
	case bool:
		return strconv.FormatBool(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return jsonNumber(v), nil
	}
	return v.(string), nil
}

// jsonNumber is f as JavaScript writes it, and JSON but for -0: in decimal
// from 1e-6 up to 1e21, past them with an exponent without leading zeros;
// zero, negative or not, is 0.
func jsonNumber(f float64) string {
	if f == 0 {
		return "0"
	}
	format := byte('f')
	if abs := math.Abs(f); abs < 1e-6 || abs >= 1e21 {
		format = 'e'
	}
	s := strconv.FormatFloat(f, format, -1, 64)
	if n := len(s); format == 'e' && s[n-4] == 'e' && s[n-3] == '-' && s[n-2] == '0' {
		s = s[:n-2] + s[n-1:] // 1e-07 is 1e-7
	}
	return s
}

// repeat counts the scalar n's bytes if an alias repeats it.
func (r *reader) repeat(n *yaml.Node) error {
	if r.aliased == 0 {
		return nil
	}
	r.repeated += len(n.Value)
	if r.repeated > r.budget {
		return errInvalid
	}
	return nil
}

// tagIs tells whether n carries no tag of its own, or tag.
func tagIs(n *yaml.Node, tag string) bool {
	return n.Style&yaml.TaggedStyle == 0 || n.Tag == tag
}

var (
	coreNull  = regexp.MustCompile(`^(?:~|null|Null|NULL)?$`)
	coreBool  = regexp.MustCompile(`^(?:true|True|TRUE|false|False|FALSE)$`)
	coreInt   = regexp.MustCompile(`^[-+]?[0-9]+$`)
	coreOct   = regexp.MustCompile(`^0o[0-7]+$`)
	coreHex   = regexp.MustCompile(`^0x[0-9a-fA-F]+$`)
	coreFloat = regexp.MustCompile(`^[-+]?(?:\.[0-9]+|[0-9]+(?:\.[0-9]*)?)(?:[eE][-+]?[0-9]+)?$`)
	coreInf   = regexp.MustCompile(`^(?:[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN))$`)
)

// scalar is a scalar's value. A quoted or block scalar is a string; a plain
// one is resolved by the core schema: dates, "yes" and the like stay
// strings, 010 is ten, and an infinity or NaN, which JSON cannot hold,
// keeps its text. An explicit tag must be the core schema's and fit.
func scalar(n *yaml.Node) (any, error) {
	if n.Style&yaml.TaggedStyle != 0 {
		switch n.Tag {
		case "!!str":
			return n.Value, nil
		case "!!null", "!!bool", "!!int", "!!float":
			v := resolve(n.Value)
			if i, ok := v.(int64); ok && n.Tag == "!!float" && coreInt.MatchString(n.Value) {
				v = float64(i) // a float may be written as an integer in decimal
			}
			if kindOf(v) != n.Tag {
				return nil, errInvalid
			}
			return v, nil
		default:
			return nil, errInvalid
		}
	}
	if n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return n.Value, nil
	}
	return resolve(n.Value), nil
}

func kindOf(v any) string {
	switch v.(type) {
	case nil:
		return "!!null"
	case bool:
		return "!!bool"
	case int64:
		return "!!int"
	case float64:
		return "!!float"
	}
	if s, ok := v.(string); ok && coreInf.MatchString(s) {
		return "!!float"
	}
	return "!!str"
}

func resolve(s string) any {
	switch {
	case coreNull.MatchString(s):
		return nil
	case coreBool.MatchString(s):
		return s[0] == 't' || s[0] == 'T'
	case coreInt.MatchString(s):
		return integer(s, s, 10)
	case coreOct.MatchString(s):
		return integer(s, s[2:], 8)
	case coreHex.MatchString(s):
		return integer(s, s[2:], 16)
	case coreFloat.MatchString(s):
		f, err := strconv.ParseFloat(s, 64)
		if err != nil { // out of range
			return s
		}
		return f
	}
	return s
}

// integer is the integer written s, its digits in base: an int64, or past
// its range the nearest float64, as a JavaScript number would hold it, or
// past a float64's s itself, which JSON cannot hold. Digits past those of
// the largest float64, its sign and leading zeros aside, are past it at
// once: big.Int would take a time and memory of the square of their
// number to tell (M4–M5 Codex review R5).
func integer(s, digits string, base int) any {
	if v, err := strconv.ParseInt(digits, base, 64); err == nil {
		return v
	}
	if len(strings.TrimLeft(strings.TrimLeft(digits, "+-"), "0")) > floatDigits(base) {
		return s
	}
	b, _ := new(big.Int).SetString(digits, base)
	f, _ := new(big.Float).SetInt(b).Float64()
	if math.IsInf(f, 0) {
		return s
	}
	return f
}

// floatDigits is how many digits in base the largest float64, about
// 1.8e308 or 2^1024, is written with: an integer of more is past it.
func floatDigits(base int) int {
	switch base {
	case 8:
		return 342
	case 16:
		return 256
	}
	return 309
}
