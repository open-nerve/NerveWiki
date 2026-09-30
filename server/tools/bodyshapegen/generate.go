package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/util"
	"go.yaml.in/yaml/v3"
)

const bodyshapeImport = "github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"

// checkers maps the Go types that have a bodyshape format checker to the
// checker's name. A string format generated as any other Go type but string
// fails the generation: its wrong values could not be answered field by
// field.
func checkers() map[codegen.SimpleTypeSpec]string {
	return map[codegen.SimpleTypeSpec]string{
		{Type: "time.Time", Import: "time"}: "FormatTime",
		{Type: "uuid.UUID", Import: "uuid"}: "FormatUUID",
	}
}

// typeNames lists the JSON types in bodyshape's bit order.
func typeNames() []string {
	return []string{"null", "boolean", "integer", "number", "string", "array", "object"}
}

// node is one bodyshape.Node before it is written out.
type node struct {
	types    []string // JSON types; empty accepts any
	format   string   // checker name, or ""
	props    map[string]int
	required []string
	extra    int // node index, closed or open
	items    int // node index or open
}

const (
	closed = -1
	open   = -2
)

type generator struct {
	types codegen.TypeMapping
	nodes []node
	seen  map[*openapi3.Schema]int
}

// generate returns the Go source of the table for the module described by
// specPath, in the package and with the type-mapping of configPath.
func generate(specPath, configPath string) ([]byte, error) {
	var cfg codegen.Configuration
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", configPath, err)
	}
	if cfg.PackageName == "" {
		return nil, fmt.Errorf("%s: no package", configPath)
	}
	// The same merge as oapi-codegen's Generate (pkg/codegen/codegen.go:162-165).
	g := &generator{types: codegen.DefaultTypeMapping, seen: map[*openapi3.Schema]int{}}
	if cfg.OutputOptions.TypeMapping != nil {
		g.types = codegen.DefaultTypeMapping.Merge(*cfg.OutputOptions.TypeMapping)
	}
	doc, err := util.LoadSwagger(specPath)
	if err != nil {
		return nil, err
	}

	roots := map[string]int{}
	// Paths, methods and properties are visited in sorted order, so the node
	// numbers are the same on every run.
	paths := doc.Paths.Map()
	for _, path := range slices.Sorted(maps.Keys(paths)) {
		ops := paths[path].Operations()
		for _, method := range slices.Sorted(maps.Keys(ops)) {
			body := ops[method].RequestBody
			if body == nil || body.Value == nil {
				continue
			}
			media := body.Value.Content.Get("application/json")
			if media == nil || media.Schema == nil {
				continue
			}
			pattern := strings.ToUpper(method) + " " + path
			root, err := g.add(media.Schema, "")
			if err != nil {
				return nil, fmt.Errorf("%s: %w", pattern, err)
			}
			roots[pattern] = root
		}
	}
	return g.source(cfg.PackageName, filepath.Base(specPath), roots)
}

// add returns the node for s, adding it and its children first. at is the
// JSON path, for error messages.
func (g *generator) add(ref *openapi3.SchemaRef, at string) (int, error) {
	s := ref.Value
	if i, ok := g.seen[s]; ok {
		return i, nil
	}
	fail := func(format string, args ...any) (int, error) {
		where := at
		if where == "" {
			where = "request body"
		}
		return 0, fmt.Errorf("%s: %s", where, fmt.Sprintf(format, args...))
	}
	for _, ext := range []string{"x-go-type", "x-go-type-import"} {
		if _, ok := s.Extensions[ext]; ok {
			return fail("%s bypasses the type-mapping, so the generated Go type is unknown", ext)
		}
	}
	switch {
	case len(s.AllOf) > 0, len(s.OneOf) > 0, s.Not != nil:
		return fail("allOf, oneOf and not are not supported")
	case s.Nullable:
		return fail("nullable is OpenAPI 3.0; write type: [X, 'null']")
	case len(s.AnyOf) > 0:
		return g.addNullable(s, at, fail)
	}

	i := len(g.nodes)
	g.nodes = append(g.nodes, node{}) // reserved first: a schema may refer to itself
	g.seen[s] = i
	n := node{extra: open, items: open}
	if s.Type != nil {
		for _, t := range s.Type.Slice() {
			if !slices.Contains(typeNames(), t) {
				return fail("unknown type %q", t)
			}
		}
		n.types = s.Type.Slice()
	}
	if format, spelled := stringFormat(s); format != "" && s.Type.Includes("string") {
		spec := g.types.String.Resolve(format)
		switch checker, ok := checkers()[spec]; {
		case ok:
			n.format = checker
		case spec.Type != "string":
			return fail("%s is generated as %s, which has no bodyshape checker", spelled, spec.Type)
		}
	}
	if s.Type.Includes("integer") {
		if problem := uncheckedNumber("integer", g.types.Integer, s.Format, "int", "int64"); problem != "" {
			return fail("%s", problem)
		}
	}
	if s.Type.Includes("number") {
		if problem := uncheckedNumber("number", g.types.Number, s.Format, "float64"); problem != "" {
			return fail("%s", problem)
		}
	}
	if s.Type.Includes("object") || len(s.Properties) > 0 {
		n.props = map[string]int{}
		for _, name := range slices.Sorted(maps.Keys(s.Properties)) {
			child, err := g.add(s.Properties[name], join(at, name))
			if err != nil {
				return 0, err
			}
			n.props[name] = child
		}
		n.required = slices.Sorted(slices.Values(s.Required))
		switch ap := s.AdditionalProperties; {
		case ap.Schema != nil:
			child, err := g.add(ap.Schema, join(at, "*"))
			if err != nil {
				return 0, err
			}
			n.extra = child
		case ap.Has != nil && !*ap.Has:
			n.extra = closed
		}
	}
	if s.Items != nil {
		child, err := g.add(s.Items, at+"[]")
		if err != nil {
			return 0, err
		}
		n.items = child
	}
	g.nodes[i] = n
	return i, nil
}

// addNullable handles anyOf: [X, {type: 'null'}], "X or null": X's node
// with null added.
func (g *generator) addNullable(s *openapi3.Schema, at string, fail func(string, ...any) (int, error)) (int, error) {
	var other *openapi3.SchemaRef
	nulls := 0
	for _, alt := range s.AnyOf {
		if alt.Value.Type != nil && alt.Value.Type.Is("null") {
			nulls++
			continue
		}
		other = alt
	}
	if len(s.AnyOf) != 2 || nulls != 1 {
		return fail("anyOf is supported only as [X, {type: 'null'}]")
	}
	x, err := g.add(other, at)
	if err != nil {
		return 0, err
	}
	n := g.nodes[x]
	if len(n.types) > 0 && !slices.Contains(n.types, "null") {
		n.types = append(slices.Clone(n.types), "null")
	}
	g.nodes = append(g.nodes, n)
	g.seen[s] = len(g.nodes) - 1
	return len(g.nodes) - 1, nil
}

// stringFormat returns the format by which oapi-codegen picks a string's Go
// type, and how the schema spelled it. Without format, an OpenAPI 3.1 string
// with contentMediaType is generated as format "binary", one with
// contentEncoding base64 as format "byte" (oapi-codegen v2.8.0,
// pkg/codegen/schema.go:1294-1330); nervewiki's descriptions are all 3.1.
func stringFormat(s *openapi3.Schema) (format, spelled string) {
	switch {
	case s.Format != "":
		return s.Format, fmt.Sprintf("format %q", s.Format)
	case s.ContentMediaType != "":
		return "binary", `contentMediaType, read as format "binary",`
	case s.ContentEncoding == "base64":
		return "byte", `contentEncoding base64, read as format "byte",`
	}
	return "", ""
}

// uncheckedNumber says why a number of JSON type t with format is generated as
// a Go type whose range bodyshape does not check, or returns "" when it is one
// of covered. bodyshape's Integer is a literal that an int64 holds, its Number
// one that a float64 holds: a narrower or unsigned integer, or float32, would
// let values through that the generated decoder then rejects without a field
// error. A format the mapping does not know falls back to the default type,
// which need not hold the range the format names.
func uncheckedNumber(t string, m codegen.FormatMapping, format string, covered ...string) string {
	what := t
	if format != "" {
		what = fmt.Sprintf("%s format %q", t, format)
		if _, known := m.Formats[format]; !known {
			return what + " is not in the type-mapping; the default Go type need not hold the range it names"
		}
	}
	if spec := m.Resolve(format); spec.Import != "" || !slices.Contains(covered, spec.Type) {
		return fmt.Sprintf("%s is generated as %s, whose range bodyshape does not check", what, spec.Type)
	}
	return ""
}

func join(at, name string) string {
	if at == "" {
		return name
	}
	return at + "." + name
}

// source renders the table as a gofmt-ed Go file.
func (g *generator) source(pkg, specName string, roots map[string]int) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by bodyshapegen from %s. DO NOT EDIT.\n\n", specName)
	fmt.Fprintf(&b, "package %s\n\nimport %q\n\n", pkg, bodyshapeImport)
	b.WriteString("// BodyShapes returns the structure of every JSON request body of this module,\n")
	b.WriteString("// by route pattern. The platform's bodyshape middleware checks each request\n")
	b.WriteString("// body against it before the strict handler decodes it.\n")
	b.WriteString("func BodyShapes() *bodyshape.Table {\n\treturn &bodyshape.Table{\n\t\tNodes: []bodyshape.Node{\n")
	for i, n := range g.nodes {
		fmt.Fprintf(&b, "\t\t\t/* %d */ {Types: %s", i, typeExpr(n.types))
		if n.format != "" {
			fmt.Fprintf(&b, ", Format: bodyshape.%s", n.format)
		}
		fmt.Fprintf(&b, ", Extra: %s, Items: %s", indexExpr(n.extra), indexExpr(n.items))
		if n.props != nil {
			b.WriteString(", Props: map[string]int{")
			for j, name := range slices.Sorted(maps.Keys(n.props)) {
				if j > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%q: %d", name, n.props[name])
			}
			b.WriteString("}")
		}
		if len(n.required) > 0 {
			fmt.Fprintf(&b, ", Required: %#v", n.required)
		}
		b.WriteString("},\n")
	}
	b.WriteString("\t\t},\n\t\tRoots: map[string]int{\n")
	for _, pattern := range slices.Sorted(maps.Keys(roots)) {
		fmt.Fprintf(&b, "\t\t\t%q: %d,\n", pattern, roots[pattern])
	}
	b.WriteString("\t\t},\n\t}\n}\n")
	src, err := format.Source(b.Bytes())
	if err != nil {
		return nil, errors.Join(errors.New("format the generated source"), err)
	}
	return src, nil
}

func typeExpr(types []string) string {
	var bits []string
	for _, t := range typeNames() {
		if slices.Contains(types, t) {
			bits = append(bits, "bodyshape."+goTypeName(t))
		}
	}
	if len(bits) == 0 {
		return "bodyshape.Any"
	}
	return strings.Join(bits, " | ")
}

func goTypeName(t string) string {
	return strings.ToUpper(t[:1]) + t[1:]
}

func indexExpr(i int) string {
	switch i {
	case closed:
		return "bodyshape.Closed"
	case open:
		return "bodyshape.Open"
	}
	return fmt.Sprint(i)
}
