package mcpserver

import (
	"encoding"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

// notebookParameter is the account entry's argument of a Notebook tool:
// the notebook's id, which the notebook entry takes from its address.
const notebookParameter = "notebook"

// inputOf is the input schema of a tool named name whose arguments decode
// into in, as tools/list answers it (M9/P1 design 3.3.3), and the
// structure table that checks a call's arguments against the same schema,
// its root under name. jsonschema-go derives it from in, and it is
// normalized: a required field, an array and an array's items take no
// null; an optional scalar keeps its ["null", T], a model often sending
// null for an argument it leaves out. With notebook, the account entry's
// "notebook" comes first among the properties and the required. A type
// it cannot hold panics: no struct; a field of a type with a JSON form of
// its own (uuid.UUID, time.Time: an id is a string field, a time no
// input), a map, an interface or a float32; a struct embedded under a
// JSON name; a type that holds itself; an optional field that is no
// pointer, which would refuse null; a field of in named "notebook"; and a
// keyword the table has no place for.
func inputOf(name string, in reflect.Type, notebook bool) (*jsonschema.Schema, *bodyshape.Table) {
	if in.Kind() != reflect.Struct {
		panic(fmt.Sprintf("mcpserver: tool %s: its input %v is no struct", name, in))
	}
	if err := checkFields(in, map[reflect.Type]bool{}); err != nil {
		panic(fmt.Sprintf("mcpserver: tool %s: its input %v: %v", name, in, err))
	}
	schema, err := jsonschema.ForType(in, &jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Sprintf("mcpserver: tool %s: the schema of %v: %v", name, in, err))
	}
	if _, ok := schema.Properties[notebookParameter]; ok {
		panic(fmt.Sprintf("mcpserver: tool %s: its input %v has a %q, which is the entry's", name, in, notebookParameter))
	}
	normalize(schema, true)
	if notebook {
		if schema.Properties == nil {
			schema.Properties = map[string]*jsonschema.Schema{}
		}
		schema.Properties[notebookParameter] = &jsonschema.Schema{Type: "string", Description: "The notebook's id, from notebooks."}
		schema.PropertyOrder = slices.Concat([]string{notebookParameter}, schema.PropertyOrder)
		schema.Required = slices.Concat([]string{notebookParameter}, schema.Required)
	}
	written, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("mcpserver: tool %s: writing its schema: %v", name, err))
	}
	var tree map[string]any
	if err := json.Unmarshal(written, &tree); err != nil {
		panic(fmt.Sprintf("mcpserver: tool %s: reading its schema: %v", name, err))
	}
	t := &bodyshape.Table{Roots: map[string]int{name: 0}}
	if err := addNode(t, tree, "arguments"); err != nil {
		panic(fmt.Sprintf("mcpserver: tool %s: %v", name, err))
	}
	return schema, t
}

// checkFields walks the fields of t, a struct, and of the structs, the
// pointers and the slices in them, for what inputOf refuses: jsonschema-go
// writes a type with a JSON form of its own as a string or as an array of
// bounded integers, which a schema does not tell apart from a string or a
// slice, so the Go type is read. It flattens a struct embedded under a
// JSON name, which encoding/json decodes as a member of that name. path
// holds the structs being walked: one met again holds itself, which would
// walk forever.
func checkFields(t reflect.Type, path map[reflect.Type]bool) error {
	if path[t] {
		return fmt.Errorf("%v holds itself: a schema of properties and items has no cycle", t)
	}
	path[t] = true
	defer delete(path, t)
	for _, f := range reflect.VisibleFields(t) {
		name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		if f.Anonymous && name != "" && name != "-" {
			return fmt.Errorf("field %s is embedded under the JSON name %q: its schema and its decoding differ", f.Name, name)
		}
		if !f.IsExported() || f.Anonymous || name == "-" {
			continue
		}
		optional := slices.ContainsFunc(strings.Split(opts, ","), func(o string) bool { return o == "omitempty" || o == "omitzero" })
		if optional && f.Type.Kind() != reflect.Pointer {
			return fmt.Errorf("field %s is optional but no pointer: it would refuse null", f.Name)
		}
		if err := checkType(f.Type, path); err != nil {
			return fmt.Errorf("field %s: %w", f.Name, err)
		}
	}
	return nil
}

// checkType is checkFields for one field's type. jsonschema-go bounds the
// integers narrower than int64, keywords the table refuses, but not a
// float32: the table's number, a float64's, would pass what it cannot
// hold.
func checkType(t reflect.Type, path map[reflect.Type]bool) error {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	marshaler, text := reflect.TypeFor[json.Marshaler](), reflect.TypeFor[encoding.TextMarshaler]()
	switch {
	case t.Implements(marshaler) || reflect.PointerTo(t).Implements(marshaler) ||
		t.Implements(text) || reflect.PointerTo(t).Implements(text):
		return fmt.Errorf("%v has a JSON form of its own: an id is a string, a time no input", t)
	case t.Kind() == reflect.Map, t.Kind() == reflect.Interface, t.Kind() == reflect.Array:
		return fmt.Errorf("%v is a %v: a schema of properties and items holds none", t, t.Kind())
	case t.Kind() == reflect.Float32:
		return fmt.Errorf("%v is narrower than a number: a float64 is", t)
	case t.Kind() == reflect.Struct:
		return checkFields(t, path)
	}
	return nil
}

// normalize takes null out of s where it is refused: s required, an
// array, or an array's items; and so on down its properties and items.
func normalize(s *jsonschema.Schema, required bool) {
	if required || s.Type == "array" || slices.Contains(s.Types, "array") {
		if types := slices.DeleteFunc(slices.Clone(s.Types), func(t string) bool { return t == "null" }); len(types) == 1 {
			s.Type, s.Types = types[0], nil
		} else if len(s.Types) > 0 {
			s.Types = types
		}
	}
	for _, name := range slices.Sorted(maps.Keys(s.Properties)) {
		normalize(s.Properties[name], slices.Contains(s.Required, name))
	}
	if s.Items != nil {
		normalize(s.Items, true)
	}
}

// addNode appends the node of schema, written as JSON, to t, and those
// of its properties and items after it; path names it in an error. Only
// the keywords the table holds may be there.
func addNode(t *bodyshape.Table, schema map[string]any, path string) error {
	for key := range schema {
		switch key {
		case "type", "properties", "required", "additionalProperties", "items", "description":
		default:
			return fmt.Errorf("%s has the keyword %q: a value's checks are the use case's", path, key)
		}
	}
	types, err := typesOf(schema["type"])
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	at := len(t.Nodes)
	t.Nodes = append(t.Nodes, bodyshape.Node{Types: types, Extra: bodyshape.Open, Items: bodyshape.Open})
	node := t.Nodes[at]
	switch extra := schema["additionalProperties"].(type) {
	case nil:
	case bool:
		if !extra {
			node.Extra = bodyshape.Closed
		}
	default:
		return fmt.Errorf("%s: additionalProperties is a schema", path)
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		node.Props = map[string]int{}
		for _, name := range slices.Sorted(maps.Keys(props)) {
			prop, ok := props[name].(map[string]any)
			if !ok {
				return fmt.Errorf("%s.%s is no schema", path, name)
			}
			node.Props[name] = len(t.Nodes)
			if err := addNode(t, prop, path+"."+name); err != nil {
				return err
			}
		}
	}
	if required, ok := schema["required"].([]any); ok {
		for _, r := range required {
			node.Required = append(node.Required, fmt.Sprint(r))
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		node.Items = len(t.Nodes)
		if err := addNode(t, items, path+"[]"); err != nil {
			return err
		}
	}
	t.Nodes[at] = node
	return nil
}

// typesOf is a schema's type, one name or several, as the table's Types.
func typesOf(v any) (bodyshape.Type, error) {
	names, ok := v.([]any)
	if !ok {
		names = []any{v}
	}
	var types bodyshape.Type
	for _, name := range names {
		switch name {
		case "null":
			types |= bodyshape.Null
		case "boolean":
			types |= bodyshape.Boolean
		case "integer":
			types |= bodyshape.Integer
		case "number":
			types |= bodyshape.Number
		case "string":
			types |= bodyshape.String
		case "array":
			types |= bodyshape.Array
		case "object":
			types |= bodyshape.Object
		default:
			return 0, fmt.Errorf("the type %v", name)
		}
	}
	return types, nil
}
