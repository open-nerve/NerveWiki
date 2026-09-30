// Command bodyshapegen writes the request-body structure table of one module
// (M0/P4 design 3.4): for every operation with a JSON request body, the
// schema's JSON types, properties, required properties, additionalProperties
// and the string formats the generated code decodes into Go types. The
// platform's bodyshape middleware checks each request body against it before
// the generated strict handler decodes the body.
//
// It reads the module's API description with oapi-codegen's own loader and
// the module's oapi-codegen configuration, so the formats follow the same
// type-mapping as server.gen.go, and a number generated as a Go type whose
// range the check does not cover fails the generation. make gen-go runs it
// for every module:
//
//	go -C server/tools run ./bodyshapegen -config <oapi-codegen.yaml> -out <bodyshape.gen.go> <api/modules/m.yaml>
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	config := flag.String("config", "", "the module's oapi-codegen configuration")
	out := flag.String("out", "", "the Go file to write")
	flag.Parse()
	if *config == "" || *out == "" || flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: bodyshapegen -config <oapi-codegen.yaml> -out <bodyshape.gen.go> <module.yaml>")
		os.Exit(2)
	}
	src, err := generate(flag.Arg(0), *config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bodyshapegen: %s: %v\n", flag.Arg(0), err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "bodyshapegen: %v\n", err)
		os.Exit(1)
	}
}
