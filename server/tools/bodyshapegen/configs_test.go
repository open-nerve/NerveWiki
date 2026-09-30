package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"go.yaml.in/yaml/v3"
)

// typeOptions are the options of an oapi-codegen configuration that decide
// the Go types of the generated models.
type typeOptions struct {
	Compatibility  codegen.CompatibilityOptions
	NameNormalizer string
	NullableType   bool
	TypeMapping    *codegen.TypeMapping
}

func readTypeOptions(t *testing.T, path string) typeOptions {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg codegen.Configuration
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return typeOptions{
		Compatibility:  cfg.Compatibility,
		NameNormalizer: cfg.OutputOptions.NameNormalizer,
		NullableType:   cfg.OutputOptions.NullableType,
		TypeMapping:    cfg.OutputOptions.TypeMapping,
	}
}

// A module's request body may reference a component of api/common.yaml: the
// strict handler then decodes into the type apigen generated, while this
// tool builds the table from the module's configuration. The two agree only
// if every configuration generates types the same way.
func TestEveryConfigurationGeneratesTheSameTypes(t *testing.T) {
	const apigen = "../../internal/platform/httpserver/apigen/oapi-codegen.yaml"
	modules, err := filepath.Glob("../../internal/modules/*/adapter/http/gen/oapi-codegen.yaml")
	if err != nil || len(modules) == 0 {
		t.Fatalf("module configurations = %q, %v; want at least one", modules, err)
	}
	want := readTypeOptions(t, apigen)
	if want.TypeMapping == nil || !want.NullableType {
		t.Fatalf("%s sets no type-mapping or nullable-type: %+v", apigen, want)
	}
	for _, path := range modules {
		if got := readTypeOptions(t, path); !reflect.DeepEqual(got, want) {
			t.Errorf("%s generates types differently from %s:\n%+v\nwant\n%+v", path, apigen, got, want)
		}
	}
}
