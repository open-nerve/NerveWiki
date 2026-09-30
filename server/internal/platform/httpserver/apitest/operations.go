package apitest

import (
	"maps"
	"slices"
	"strings"
)

// Operation is one operation of the contract, as the whole-program tests of
// bootstrap see it (M0/P4 design 3.6).
type Operation struct {
	ID     string // operationId
	Method string // upper case
	Path   string
	// ProblemHeaders are the headers its default response, the problem,
	// declares, sorted.
	ProblemHeaders []string
}

// Pattern is the route pattern the generated code registers, e.g.
// "GET /api/v0/instance".
func (o Operation) Pattern() string { return o.Method + " " + o.Path }

// Operations lists every operation of the contract, sorted by pattern.
func (c *Contract) Operations() []Operation {
	var ops []Operation
	for path, item := range c.doc.Paths.Map() {
		for method, op := range item.Operations() {
			o := Operation{ID: op.OperationID, Method: strings.ToUpper(method), Path: path}
			if problem := op.Responses.Default(); problem != nil && problem.Value != nil {
				o.ProblemHeaders = slices.Sorted(maps.Keys(problem.Value.Headers))
			}
			ops = append(ops, o)
		}
	}
	slices.SortFunc(ops, func(a, b Operation) int { return strings.Compare(a.Pattern(), b.Pattern()) })
	return ops
}
