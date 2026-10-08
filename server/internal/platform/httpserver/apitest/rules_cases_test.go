package apitest

import (
	"slices"
	"strings"
	"testing"
)

// ruleCasesBase follows every authoring rule; each case of
// TestAuthoringRulesReportViolations breaks exactly one.
const ruleCasesBase = `
openapi: 3.1.0
info: {title: rule cases, version: v0}
x-problem-codes: [bad_request, internal_error]
tags:
  - name: things
paths:
  /api/v0/things:
    get:
      operationId: listThings
      tags: [things]
      security: [{bearer: []}]
      x-problem-codes: [not_found]
      parameters:
        - {name: kind, in: query, schema: {type: string, enum: [a, b]}}
      responses:
        '200':
          description: The things.
          content:
            application/json:
              schema: {$ref: '#/components/schemas/Thing'}
        default:
          $ref: '#/components/responses/Problem'
components:
  securitySchemes:
    bearer: {type: http, scheme: bearer}
  responses:
    Problem:
      description: Error.
      content:
        application/problem+json:
          schema: {$ref: '#/components/schemas/Problem'}
  schemas:
    Problem:
      type: object
      additionalProperties: false
      properties:
        code: {type: string}
    Thing:
      type: object
      additionalProperties: false
      properties:
        name: {type: string}
        note: {type: [string, 'null']}
        labels: {type: array, items: {type: string}}
    Target:
      anyOf:
        - $ref: '#/components/schemas/Thing'
        - {type: 'null'}
    Named:
      type: object
      allOf:
        - $ref: '#/components/schemas/Thing'
        - {required: [name]}
`

// ruleCasesModules are the modules of the cases: things declares the base's
// path, and stuff is another module.
func ruleCasesModules() []string { return []string{"stuff", "things"} }

// A code prefixed with a module passes: the module's own, or another
// module's, which refused; and the platform codes.
func TestModuleCodesPass(t *testing.T) {
	doc := parse(t, strings.Replace(ruleCasesBase, "x-problem-codes: [not_found]",
		"x-problem-codes: [things.taken, stuff.not_found, forbidden, validation_failed]", 1))
	if got := authoringViolations(doc, ruleCasesModules()); len(got) != 0 {
		t.Errorf("violations = %q, want none", got)
	}
}

// TestAuthoringRulesReportViolations proves each check of
// authoringViolations on a hand-built document, since the real contract
// passes them all.
func TestAuthoringRulesReportViolations(t *testing.T) {
	if got := authoringViolations(parse(t, ruleCasesBase), ruleCasesModules()); len(got) != 0 {
		t.Fatalf("the base document breaks rules: %q", got)
	}
	const (
		noProblem = "GET /api/v0/things: has no default response whose application/problem+json schema is the Problem component"
		nullable  = ": uses nullable, the OpenAPI 3.0 keyword; write type: [T, 'null']"
		constant  = ": uses const, which oapi-codegen turns into interface{}; write a single-value enum"
		nullEnum  = `: has null in enum, which adds a "<nil>" Go constant; write anyOf: [{$ref: …}, {type: 'null'}]`
		open      = ": object schema does not set additionalProperties: false"
		thing     = "    Thing:\n      type: object\n      additionalProperties: false\n"
	)
	tests := []ruleCase{
		{"path outside /api/v0/", "  /api/v0/things:", "  /things:", "/things: does not start with /api/v0/"},
		{"path ending with /", "  /api/v0/things:", "  /api/v0/things/:", "/api/v0/things/: ends with /"},
		{"operationId not lower camelCase", "operationId: listThings", "operationId: ListThings",
			`GET /api/v0/things: operationId "ListThings" is not lower camelCase`},
		{"no operationId", "      operationId: listThings\n", "", `GET /api/v0/things: operationId "" is not lower camelCase`},
		{"no tag", "      tags: [things]\n", "", "GET /api/v0/things: has no tag"},
		{"undeclared tag", "tags: [things]", "tags: [stuff]", `GET /api/v0/things: tag "stuff" is not declared in the top-level tags`},
		{"no default response", "        default:\n          $ref: '#/components/responses/Problem'\n", "", noProblem},
		{"default with another schema", "schema: {$ref: '#/components/schemas/Problem'}", "schema: {$ref: '#/components/schemas/Thing'}", noProblem},
		{"default as application/json", "        application/problem+json:\n", "        application/json:\n", noProblem},
		{"nullable", "note: {type: [string, 'null']}", "note: {type: string, nullable: true}", "components/schemas/Thing/properties/note" + nullable},
		{"nullable: false", "note: {type: [string, 'null']}", "note: {type: string, nullable: false}", "components/schemas/Thing/properties/note" + nullable},
		{"const", "name: {type: string}", "name: {type: string, const: thing}", "components/schemas/Thing/properties/name" + constant},
		{"const: null", "name: {type: string}", "name: {const: null}", "components/schemas/Thing/properties/name" + constant},
		{"null in enum", "note: {type: [string, 'null']}", "note: {type: [string, 'null'], enum: [a, null]}", "components/schemas/Thing/properties/note" + nullEnum},
		{"in items", "items: {type: string}", "items: {type: string, nullable: true}", "components/schemas/Thing/properties/labels/items" + nullable},
		{"in anyOf", "- {type: 'null'}", "- {const: null}", "components/schemas/Target/anyOf/1" + constant},
		{"in allOf", "- {required: [name]}", "- {required: [name], nullable: true}", "components/schemas/Named/allOf/1" + nullable},
		{"in a parameter", "enum: [a, b]", "enum: [a, null]", "GET /api/v0/things parameters/kind" + nullEnum},
		{"in an inline response schema", "schema: {$ref: '#/components/schemas/Thing'}", "schema: {type: string, const: x}",
			"GET /api/v0/things responses/200 application/json" + constant},
		{"open object", thing, "    Thing:\n      type: object\n", "components/schemas/Thing" + open},
		{"additionalProperties: true", thing, "    Thing:\n      type: object\n      additionalProperties: true\n", "components/schemas/Thing" + open},
		{"open inline object", "        labels: {type: array, items: {type: string}}\n",
			"        labels: {type: array, items: {type: string}}\n        nested: {type: object, properties: {a: {type: string}}}\n",
			"components/schemas/Thing/properties/nested" + open},
		{"open object in items", "items: {type: string}", "items: {type: object, properties: {a: {type: string}}}",
			"components/schemas/Thing/properties/labels/items" + open},
		{"composition with properties of its own", "    Named:\n      type: object\n",
			"    Named:\n      type: object\n      properties: {id: {type: string}}\n", "components/schemas/Named" + open},
		{"parameter with content", "- {name: kind, in: query, schema: {type: string, enum: [a, b]}}",
			"- {name: kind, in: query, content: {application/json: {schema: {type: string}}}}",
			"GET /api/v0/things parameters/kind: declares content; write schema"},
		{"component parameter with content", "  schemas:\n    Problem:",
			"  parameters:\n    Kind: {name: kind, in: query, content: {application/json: {schema: {type: string}}}}\n  schemas:\n    Problem:",
			"components/parameters/Kind: declares content; write schema"},
		{"path parameter with content", "  /api/v0/things:\n    get:",
			"  /api/v0/things:\n    parameters:\n      - {name: view, in: query, content: {application/json: {schema: {type: string}}}}\n    get:",
			"/api/v0/things parameters/view: declares content; write schema"},
		{"request body that is not an object", "      parameters:\n",
			"      requestBody:\n        content:\n          application/json:\n            schema: {type: array, items: {type: string}}\n      parameters:\n",
			"GET /api/v0/things requestBody: application/json schema is not an object"},
		{"request body component that is not an object", "  schemas:\n    Problem:",
			"  requestBodies:\n    Tags:\n      content:\n        application/json:\n          schema: {type: array, items: {type: string}}\n  schemas:\n    Problem:",
			"components/requestBodies/Tags: application/json schema is not an object"},
		{"no top-level codes", "x-problem-codes: [bad_request, internal_error]\n", "", "top level: has no x-problem-codes"},
		{"module code at the top level", "[bad_request, internal_error]", "[bad_request, things.taken]",
			`top level: x-problem-codes holds "things.taken", which is not a platform code`},
		{"no security", "      security: [{bearer: []}]\n", "",
			"GET /api/v0/things: declares no security; write [{bearer: []}], or [] for a public operation"},
		{"undeclared scheme", "    bearer: {type: http, scheme: bearer}", "    other: {type: http, scheme: bearer}",
			`GET /api/v0/things: security scheme "bearer" is not declared in components.securitySchemes`},
		{"anonymous allowed", "security: [{bearer: []}]", "security: [{}]",
			"GET /api/v0/things: security is neither [] nor [{bearer: []}]; OpenAPI reads its requirements as alternatives"},
		{"a token or none", "security: [{bearer: []}]", "security: [{bearer: []}, {}]",
			"GET /api/v0/things: security is neither [] nor [{bearer: []}]; OpenAPI reads its requirements as alternatives"},
		{"another scheme", "security: [{bearer: []}]", "security: [{apiKey: []}]",
			"GET /api/v0/things: security is neither [] nor [{bearer: []}]; OpenAPI reads its requirements as alternatives"},
		{"bearer with scopes", "security: [{bearer: []}]", "security: [{bearer: [read]}]",
			"GET /api/v0/things: security is neither [] nor [{bearer: []}]; OpenAPI reads its requirements as alternatives"},
		{"no operation codes", "      x-problem-codes: [not_found]\n", "",
			"GET /api/v0/things: has no x-problem-codes; write [] when it answers only the top-level codes"},
		{"codes not a list", "x-problem-codes: [not_found]", "x-problem-codes: not_found",
			"GET /api/v0/things: x-problem-codes is not a list"},
		{"code misspelled", "x-problem-codes: [not_found]", "x-problem-codes: [Things.Taken]",
			`GET /api/v0/things: problem code "Things.Taken" is not spelled [module.]lower_snake`},
		{"code of no module", "x-problem-codes: [not_found]", "x-problem-codes: [nowhere.taken]",
			`GET /api/v0/things: problem code "nowhere.taken" is prefixed with "nowhere", which is not a module: want one of ["stuff" "things"]`},
		{"unprefixed code that is not the platform's", "x-problem-codes: [not_found]", "x-problem-codes: [taken]",
			`GET /api/v0/things: problem code "taken" has no module prefix and is not a platform code`},
	}
	reportsEach(t, ruleCasesBase, tests)
}

// ruleCase replaces the first old of a base document with new, which breaks
// one rule: want is the one violation.
type ruleCase struct {
	name, old, new, want string
}

// reportsEach runs each case on base.
func reportsEach(t *testing.T, base string, tests []ruleCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(base, tt.old) {
				t.Fatalf("the base document has no %q", tt.old)
			}
			doc := parse(t, strings.Replace(base, tt.old, tt.new, 1))
			if got := authoringViolations(doc, ruleCasesModules()); !slices.Equal(got, []string{tt.want}) {
				t.Errorf("violations = %q, want %q", got, tt.want)
			}
		})
	}
}

// rawCasesPath is a raw path for ruleCasesBase: a download with the
// statuses of ranges and conditional requests, and an upload in parts.
const rawCasesPath = `  /api/v0/things/{thing_id}/content:
    parameters:
      - {name: thing_id, in: path, required: true, schema: {type: string, format: uuid}}
    get:
      operationId: getThingContent
      tags: [things]
      security: []
      x-raw: true
      x-problem-codes: [not_found]
      responses:
        '200':
          description: The bytes.
          content:
            '*/*':
              schema: {type: string, format: binary}
        '206':
          description: A range of the bytes.
          content:
            '*/*':
              schema: {type: string, format: binary}
        '304': {description: Not modified.}
        '416': {description: No such range.}
        default:
          $ref: '#/components/responses/Problem'
    put:
      operationId: putThingContent
      tags: [things]
      security: [{bearer: []}]
      x-raw: true
      x-problem-codes: [not_found]
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              additionalProperties: false
              properties:
                label: {type: string}
                file: {type: string, format: binary}
      responses:
        '204': {description: Stored.}
        default:
          $ref: '#/components/responses/Problem'
`

// A raw operation (x-raw: true) may take multipart/form-data, answer */*
// and the statuses of ranges and conditional requests; no other operation
// may, through a component either, and the parts and the bytes have the
// shapes the handlers read and write: a closed object of strings, binary
// bytes, never null. A response's headers are schemas the rules see; a
// referenced response is checked once, where it is defined.
func TestRawOperationRulesReportViolations(t *testing.T) {
	base := strings.Replace(ruleCasesBase, "\ncomponents:\n", "\n"+rawCasesPath+"components:\n", 1)
	if got := authoringViolations(parse(t, base), ruleCasesModules()); len(got) != 0 || !strings.Contains(base, "x-raw") {
		t.Fatalf("the base document with the raw path breaks rules, or lacks it: %q", got)
	}
	reportsEach(t, base, []ruleCase{
		{"*/* in an operation that is not raw", "            application/json:\n              schema: {$ref: '#/components/schemas/Thing'}",
			"            '*/*':\n              schema: {type: string, format: binary}",
			"GET /api/v0/things responses/200 */*: only a raw operation (x-raw: true) answers */*"},
		{"206 in an operation that is not raw", "        default:\n", "        '206': {description: part}\n        default:\n",
			"GET /api/v0/things responses/206: only a raw operation (x-raw: true) answers 206"},
		{"304 in an operation that is not raw", "        default:\n", "        '304': {description: same}\n        default:\n",
			"GET /api/v0/things responses/304: only a raw operation (x-raw: true) answers 304"},
		{"416 in an operation that is not raw", "        default:\n", "        '416': {description: none}\n        default:\n",
			"GET /api/v0/things responses/416: only a raw operation (x-raw: true) answers 416"},
		{"x-raw that is not true", "      x-raw: true\n      x-problem-codes: [not_found]\n      requestBody:",
			"      x-raw: 'true'\n      x-problem-codes: [not_found]\n      requestBody:",
			"PUT /api/v0/things/{thing_id}/content requestBody multipart/form-data: only a raw operation (x-raw: true) takes multipart/form-data"},
		{"*/* that is not binary", "schema: {type: string, format: binary}", "schema: {type: string}",
			"GET /api/v0/things/{thing_id}/content responses/200 */*: schema is not type: string, format: binary"},
		{"multipart in an operation that is not raw", "      parameters:\n",
			"      requestBody:\n        content:\n          multipart/form-data:\n            schema: {type: object, additionalProperties: false, properties: {a: {type: string}}}\n      parameters:\n",
			"GET /api/v0/things requestBody multipart/form-data: only a raw operation (x-raw: true) takes multipart/form-data"},
		{"another media type", "          multipart/form-data:\n", "          text/plain:\n",
			"PUT /api/v0/things/{thing_id}/content requestBody text/plain: a request body is application/json, or multipart/form-data in a raw operation"},
		{"part that is not a string", "label: {type: string}", "label: {type: integer}",
			"PUT /api/v0/things/{thing_id}/content requestBody multipart/form-data/properties/label: is not a string; a part is text or, with format: binary, a file"},
		{"parts that are not an object", "            schema:\n              type: object\n              additionalProperties: false\n              properties:\n                label: {type: string}\n                file: {type: string, format: binary}\n",
			"            schema: {type: string}\n",
			"PUT /api/v0/things/{thing_id}/content requestBody multipart/form-data: schema is not an object"},
		{"*/* in a response component", "  responses:\n    Problem:\n",
			"  responses:\n    Download:\n      description: The bytes.\n      content:\n        '*/*':\n          schema: {type: string, format: binary}\n    Problem:\n",
			"components/responses/Download */*: only a raw operation (x-raw: true) answers */*"},
		{"multipart in a request body component", "  schemas:\n    Problem:",
			"  requestBodies:\n    Upload:\n      content:\n        multipart/form-data:\n          schema: {type: object, additionalProperties: false, properties: {a: {type: string}}}\n  schemas:\n    Problem:",
			"components/requestBodies/Upload multipart/form-data: only a raw operation (x-raw: true) takes multipart/form-data"},
		{"*/* that may be null", "schema: {type: string, format: binary}", "schema: {type: [string, 'null'], format: binary}",
			"GET /api/v0/things/{thing_id}/content responses/200 */*: schema is not type: string, format: binary"},
		{"*/* without a schema", "            '*/*':\n              schema: {type: string, format: binary}\n        '206':",
			"            '*/*': {}\n        '206':",
			"GET /api/v0/things/{thing_id}/content responses/200 */*: schema is not type: string, format: binary"},
		{"part that may be null", "label: {type: string}", "label: {type: [string, 'null']}",
			"PUT /api/v0/things/{thing_id}/content requestBody multipart/form-data/properties/label: is not a string; a part is text or, with format: binary, a file"},
		{"parts without a schema", "            schema:\n              type: object\n              additionalProperties: false\n              properties:\n                label: {type: string}\n                file: {type: string, format: binary}\n",
			"            {}\n",
			"PUT /api/v0/things/{thing_id}/content requestBody multipart/form-data: schema is not an object"},
		{"open parts", "              additionalProperties: false\n              properties:\n                label:",
			"              properties:\n                label:",
			"PUT /api/v0/things/{thing_id}/content requestBody multipart/form-data: object schema does not set additionalProperties: false"},
		{"a referenced response's header", "    Problem:\n      description: Error.\n",
			"    Problem:\n      description: Error.\n      headers:\n        Retry-After: {schema: {type: integer, nullable: true}}\n",
			"components/responses/Problem headers/Retry-After: uses nullable, the OpenAPI 3.0 keyword; write type: [T, 'null']"},
	})
}
