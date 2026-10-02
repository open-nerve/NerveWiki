package markdown

import (
	"bytes"
	"errors"
	"io"
	"math"
	"math/big"
	"regexp"
	"strconv"

	"go.yaml.in/yaml/v3"
)

const (
	// maxYAMLNodes is how many values a frontmatter may hold once its
	// aliases are expanded: a line of aliases must not make every parse
	// and every reading blow up (M4 design 4, "frontmatter").
	maxYAMLNodes = 10000
	// maxYAMLDepth is how deep its values may nest.
	maxYAMLDepth = 64
)

var errInvalid = errors.New("not a frontmatter's YAML")

// properties parses a frontmatter's YAML into its properties: a mapping,
// read by YAML 1.2's core schema with its aliases expanded (the fixtures'
// rule 1). Empty YAML is a frontmatter without properties; anything else
// that is not a mapping, or goes past the limits, is not valid. The YAML
// library's errors are dropped: they may quote the content.
func properties(src []byte) ([]Property, bool) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var doc yaml.Node
	switch err := dec.Decode(&doc); {
	case errors.Is(err, io.EOF):
		return nil, true
	case err != nil:
		return nil, false
	}
	var more yaml.Node
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return nil, false // a second document, or an error after the first
	}
	if len(doc.Content) != 1 {
		return nil, false
	}
	top := doc.Content[0]
	if top.Kind != yaml.MappingNode {
		return nil, false
	}
	r := reader{}
	v, err := r.value(top, 0)
	if err != nil {
		return nil, false
	}
	return v.([]Property), true
}

// reader walks the YAML's nodes, counting what it expands.
type reader struct{ nodes int }

func (r *reader) value(n *yaml.Node, depth int) (any, error) {
	r.nodes++
	if r.nodes > maxYAMLNodes || depth > maxYAMLDepth {
		return nil, errInvalid
	}
	switch n.Kind {
	case yaml.AliasNode:
		return r.value(n.Alias, depth)
	case yaml.ScalarNode:
		return scalar(n)
	case yaml.SequenceNode:
		if !tagIs(n, "!!seq") {
			return nil, errInvalid
		}
		items := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := r.value(c, depth+1)
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
			v, err := r.value(n.Content[i+1], depth+1)
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
		n = n.Alias
	}
	if n.Kind != yaml.ScalarNode {
		return "", errInvalid
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
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	}
	return v.(string), nil
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
		return integer(s, 10)
	case coreOct.MatchString(s):
		return integer(s[2:], 8)
	case coreHex.MatchString(s):
		return integer(s[2:], 16)
	case coreFloat.MatchString(s):
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsInf(f, 0) {
			return s
		}
		return f
	}
	return s
}

// integer is an int64, or past its range the nearest float64, as a
// JavaScript number would hold it.
func integer(digits string, base int) any {
	if v, err := strconv.ParseInt(digits, base, 64); err == nil {
		return v
	}
	b, _ := new(big.Int).SetString(digits, base)
	f, _ := new(big.Float).SetInt(b).Float64()
	return f
}
