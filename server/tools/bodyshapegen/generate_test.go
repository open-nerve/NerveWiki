package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/things.golden") //nolint:gochecknoglobals // go test parses flags declared at package level

const (
	thingsSpec = "testdata/things.yaml"
	thingsConf = "testdata/oapi-codegen.yaml"
)

// The golden file is the reviewed output: every supported construct, the
// type-mapping (uuid checked, email left to the domain, numbers as int, int64
// and float64), a reference into another file, and one root per operation
// with a JSON body.
func TestGenerateMatchesTheGoldenFile(t *testing.T) {
	got, err := generate(thingsSpec, thingsConf)
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "things.golden")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("generate() differs from %s (go test -run Golden -update rewrites it):\n%s", golden, got)
	}
}

// Maps are iterated in random order: numbering must not depend on it, or
// make gen-check reports a difference on an unchanged description.
func TestGenerateIsDeterministic(t *testing.T) {
	first, err := generate(thingsSpec, thingsConf)
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		again, err := generate(thingsSpec, thingsConf)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(again, first) {
			t.Fatal("two runs of generate() on the same input differ")
		}
	}
}

// Without the module template's type-mapping, uuid is generated as
// openapi_types.UUID, which has no checker: the table follows the mapping.
func TestGenerateFollowsTheTypeMapping(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "oapi-codegen.yaml")
	if err := os.WriteFile(conf, []byte("package: gen\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := generate(thingsSpec, conf)

	if err == nil || !strings.Contains(err.Error(), `format "email" is generated as openapi_types.Email`) {
		t.Errorf("generate() = %v, want the default email mapping rejected", err)
	}
}

func TestGenerateRejectsWhatItCannotCheck(t *testing.T) {
	tests := []struct{ name, property, want string }{
		{"date", "{type: string, format: date}", `due: format "date" is generated as openapi_types.Date`},
		{"byte", "{type: string, format: byte}", `due: format "byte" is generated as []byte`},
		{"int32", "{type: integer, format: int32}", `due: integer format "int32" is generated as int32, whose range bodyshape does not check`},
		{"uint", "{type: integer, format: uint}", `due: integer format "uint" is generated as uint, whose range bodyshape does not check`},
		{"nullable int8", "{type: [integer, 'null'], format: int8}", `due: integer format "int8" is generated as int8`},
		{"unknown integer format", "{type: integer, format: int128}", `due: integer format "int128" is not in the type-mapping`},
		{"float", "{type: number, format: float}", `due: number format "float" is generated as float32, whose range bodyshape does not check`},
		{"unknown number format", "{type: number, format: decimal}", `due: number format "decimal" is not in the type-mapping`},
		{"x-go-type", "{type: string, x-go-type: civil.Date}", "due: x-go-type bypasses the type-mapping"},
		{"x-go-type-import", "{type: string, x-go-type-import: {path: time}}", "due: x-go-type-import bypasses the type-mapping"},
		{"oneOf", "{oneOf: [{type: string}, {type: integer}]}", "due: allOf, oneOf and not are not supported"},
		{"allOf", "{allOf: [{type: string}]}", "due: allOf, oneOf and not are not supported"},
		{"not", "{not: {type: string}}", "due: allOf, oneOf and not are not supported"},
		{"anyOf of two types", "{anyOf: [{type: string}, {type: integer}]}", "due: anyOf is supported only as [X, {type: 'null'}]"},
		{"nullable", "{type: string, nullable: true}", "due: nullable is OpenAPI 3.0"},
		{"contentMediaType", "{type: string, contentMediaType: image/png}",
			`due: contentMediaType, read as format "binary", is generated as openapi_types.File`},
		{"contentEncoding base64", "{type: string, contentEncoding: base64}",
			`due: contentEncoding base64, read as format "byte", is generated as []byte`},
		{"nested", "{type: object, properties: {at: {type: array, items: {type: string, format: date}}}}", `due.at[]: format "date"`},
		{"nested number", "{type: array, items: {anyOf: [{type: integer, format: uint64}, {type: 'null'}]}}", `due[]: integer format "uint64" is generated as uint64`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := generate(bodySpec(t, tt.property), thingsConf)

			if err == nil || !strings.Contains(err.Error(), "POST /api/v0/x: "+tt.want) {
				t.Errorf("generate() = %v, want an error containing %q", err, "POST /api/v0/x: "+tt.want)
			}
		})
	}
}

// oapi-codegen reads contentMediaType and contentEncoding only when format is
// absent, and maps no contentEncoding but base64: these stay strings.
func TestGenerateAcceptsTheStringsItCanCheck(t *testing.T) {
	for _, property := range []string{
		"{type: string, contentEncoding: base64url}",
		"{type: string, format: uuid, contentEncoding: base64}",
		"{type: string, format: date-time, contentMediaType: text/plain}",
	} {
		if _, err := generate(bodySpec(t, property), thingsConf); err != nil {
			t.Errorf("generate() of %s = %v, want nil", property, err)
		}
	}
}

// A number passes when it is generated as a Go type whose whole range
// bodyshape checks: int or int64 for an integer, float64 for a number.
func TestGenerateAcceptsTheNumbersItCanCheck(t *testing.T) {
	for _, property := range []string{
		"{type: integer}",
		"{type: integer, format: int64}",
		"{type: [integer, 'null']}",
		"{type: number}",
		"{type: number, format: double}",
	} {
		if _, err := generate(bodySpec(t, property), thingsConf); err != nil {
			t.Errorf("generate() of %s = %v, want nil", property, err)
		}
	}
}

// Without the module template's number mapping, a number is generated as
// float32, which bodyshape cannot check: the table follows the mapping.
func TestGenerateRejectsTheDefaultNumberMapping(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "oapi-codegen.yaml")
	if err := os.WriteFile(conf, []byte("package: gen\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := generate(bodySpec(t, "{type: number}"), conf)

	want := "POST /api/v0/x: due: number is generated as float32, whose range bodyshape does not check"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("generate() = %v, want an error containing %q", err, want)
	}
}

// bodySpec writes a description whose one operation, POST /api/v0/x, takes an
// object with the one property due, and returns its path.
func bodySpec(t *testing.T, property string) string {
	t.Helper()
	spec := filepath.Join(t.TempDir(), "m.yaml")
	src := "openapi: 3.1.0\ninfo: {title: t, version: v0}\npaths:\n  /api/v0/x:\n    post:\n" +
		"      requestBody:\n        content:\n          application/json:\n            schema:\n" +
		"              type: object\n              properties:\n                due: " + property + "\n" +
		"      responses: {'204': {description: none}}\n"
	if err := os.WriteFile(spec, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return spec
}
