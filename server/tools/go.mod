module github.com/open-nerve/NerveWiki/server/tools

go 1.27

toolchain go1.27.1

// The code generators, apart from the server module so that their
// dependencies stay out of it. kin-openapi and yaml stay at the versions
// oapi-codegen v2.8.0 requires: bodyshapegen reads the description with
// oapi-codegen's own loader, and an upgrade here would change oapi-codegen too.

tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen

require (
	github.com/getkin/kin-openapi v0.142.0
	github.com/oapi-codegen/oapi-codegen/v2 v2.8.0
	go.yaml.in/yaml/v3 v3.0.4
)

require (
	github.com/dprotaso/go-yit v0.0.0-20220510233725-9ba8df137936 // indirect
	github.com/go-openapi/jsonpointer v0.23.1 // indirect
	github.com/go-openapi/swag/jsonname v0.26.0 // indirect
	github.com/oasdiff/yaml v0.1.1 // indirect
	github.com/oasdiff/yaml3 v0.0.14 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	github.com/speakeasy-api/jsonpath v0.6.3 // indirect
	github.com/speakeasy-api/openapi v1.24.0 // indirect
	github.com/vmware-labs/yaml-jsonpath v0.3.2 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	golang.org/x/tools v0.48.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
