package apitest

import (
	"slices"
	"testing"
)

const operationsContract = `
openapi: 3.1.0
info: {title: operations, version: v0}
x-problem-codes: [bad_request]
paths:
  /api/v0/things:
    post:
      operationId: createThing
      security: []
      x-problem-codes: []
      responses:
        '204': {description: created}
    get:
      operationId: listThings
      security: []
      x-problem-codes: []
      responses:
        '204': {description: none}
        default:
          description: problem
          headers:
            Retry-After: {schema: {type: integer}}
            X-Trace: {schema: {type: string}}
  /api/v0/things/{thing_id}:
    parameters:
      - {name: thing_id, in: path, required: true, schema: {type: string, format: uuid}}
    get:
      operationId: getThing
      security: [{bearer: []}]
      x-problem-codes: []
      responses:
        '204': {description: none}
components:
  securitySchemes:
    bearer: {type: http, scheme: bearer}
`

func TestOperations(t *testing.T) {
	ops := contractFrom(t, operationsContract).Operations()

	want := []Operation{
		{ID: "listThings", Method: "GET", Path: "/api/v0/things", Public: true, ProblemHeaders: []string{"Retry-After", "X-Trace"}},
		{ID: "getThing", Method: "GET", Path: "/api/v0/things/{thing_id}"},
		{ID: "createThing", Method: "POST", Path: "/api/v0/things", Public: true},
	}
	if !slices.EqualFunc(ops, want, func(a, b Operation) bool {
		return a.ID == b.ID && a.Pattern() == b.Pattern() && a.Public == b.Public && slices.Equal(a.ProblemHeaders, b.ProblemHeaders)
	}) {
		t.Errorf("Operations() =\n%+v\nwant, sorted by pattern, public or not, and with the default response's headers,\n%+v", ops, want)
	}
}
