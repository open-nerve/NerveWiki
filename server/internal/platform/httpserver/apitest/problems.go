package apitest

import (
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// problemCodesKey is the extension that lists the problem codes an
// operation can answer; at the top level, the codes every operation can
// answer (v0.1 design 6.1).
const problemCodesKey = "x-problem-codes"

// problemCodes reads the x-problem-codes of ext: present reports whether the
// extension is there at all.
func problemCodes(ext map[string]any) (codes []string, present bool, err error) {
	raw, present := ext[problemCodesKey]
	if !present {
		return nil, false, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, true, fmt.Errorf("%s is not a list", problemCodesKey)
	}
	for _, item := range list {
		code, ok := item.(string)
		if !ok {
			return nil, true, fmt.Errorf("%s holds %v, which is not a string", problemCodesKey, item)
		}
		codes = append(codes, code)
	}
	return codes, true, nil
}

// longLivedKey marks an operation that holds its response open, such as
// the event stream (M5 design 4.10).
const longLivedKey = "x-long-lived"

// longLived reports whether op is marked x-long-lived: true.
func longLived(op *openapi3.Operation) bool {
	v, _ := op.Extensions[longLivedKey].(bool)
	return v
}

// needsToken reports whether op declares a bearer requirement; such an
// operation can also answer unauthorized.
func needsToken(op *openapi3.Operation) bool {
	return op.Security != nil && len(*op.Security) > 0
}

// checkProblemCode fails a problem whose code op may not answer: the
// top-level codes, op's own, and unauthorized when op needs a token. The
// code is recorded for Main.
func (c *Contract) checkProblemCode(op *openapi3.Operation, header http.Header, body []byte) error {
	if media, _, _ := mime.ParseMediaType(header.Get("Content-Type")); media != "application/problem+json" {
		return nil
	}
	var p struct{ Code string }
	if err := json.Unmarshal(body, &p); err != nil {
		return fmt.Errorf("problem body: %w", err)
	}
	top, _, err := problemCodes(c.doc.Extensions)
	if err != nil {
		return err
	}
	own, _, err := problemCodes(op.Extensions)
	if err != nil {
		return err
	}
	allowed := slices.Concat(top, own)
	if needsToken(op) {
		allowed = append(allowed, "unauthorized")
	}
	if !slices.Contains(allowed, p.Code) {
		return fmt.Errorf("problem code %q is not declared: %s may answer %q", p.Code, op.OperationID, allowed)
	}
	answered.record(op.OperationID, p.Code)
	return nil
}

// answered records, process-wide, the problem codes each operation answered
// through CheckResponse: Main reads what every test of the binary recorded.
var answered = &recorder{codes: map[string]map[string]bool{}} //nolint:gochecknoglobals // shared by every test of the binary and Main

type recorder struct {
	mu    sync.Mutex
	codes map[string]map[string]bool // operationId → code → answered
}

func (r *recorder) record(operationID, code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.codes[operationID] == nil {
		r.codes[operationID] = map[string]bool{}
	}
	r.codes[operationID][code] = true
}

func (r *recorder) snapshot() map[string]map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]map[string]bool, len(r.codes))
	for op, codes := range r.codes {
		out[op] = maps.Clone(codes)
	}
	return out
}

// Main runs the tests of a module's HTTP adapter, then fails the run unless
// every code that the module's operations declare in their own
// x-problem-codes was answered by some test through CheckResponse (v0.1
// design 6.1): a declared code that nothing answers is a wrong contract. A
// run that failed already, or was narrowed by -run, -skip or -short, is not
// checked. The module is the one whose adapter/http calls Main, so a TestMain
// copied from another module checks its own codes. Every module's
// adapter/http tests use it:
//
//	func TestMain(m *testing.M) { apitest.Main(m) }
func Main(m *testing.M) {
	_, file, _, _ := runtime.Caller(1)
	module, ok := moduleOf(file)
	if !ok {
		fmt.Fprintf(os.Stderr, "apitest.Main is called from %s, outside a module's adapter/http\n", file)
		os.Exit(2)
	}
	code := m.Run()
	if code == 0 && !narrowed() {
		doc, err := loadModule(module)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if missing := unanswered(doc, answered.snapshot()); len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "api/modules/%s.yaml declares problem codes that no test answered through CheckResponse:\n", module)
			for _, entry := range missing {
				fmt.Fprintln(os.Stderr, "  "+entry)
			}
			code = 1
		}
	}
	os.Exit(code)
}

// moduleOf returns the module whose HTTP adapter holds the source file file,
// e.g. "instance" for …/internal/modules/instance/adapter/http/main_test.go.
func moduleOf(file string) (string, bool) {
	_, rest, ok := strings.Cut(filepath.ToSlash(filepath.Dir(file)), "/internal/modules/")
	name, dir, _ := strings.Cut(rest, "/")
	return name, ok && name != "" && dir == "adapter/http"
}

// narrowed reports whether this run leaves tests out.
func narrowed() bool {
	for _, name := range []string{"test.run", "test.skip"} {
		if f := flag.Lookup(name); f != nil && f.Value.String() != "" {
			return true
		}
	}
	return testing.Short()
}

// unanswered lists "operationId: code" for each code of doc's operations
// that answered does not hold, sorted.
func unanswered(doc *openapi3.T, answered map[string]map[string]bool) []string {
	var missing []string
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			codes, _, _ := problemCodes(op.Extensions)
			for _, code := range codes {
				if !answered[op.OperationID][code] {
					missing = append(missing, op.OperationID+": "+code)
				}
			}
		}
	}
	slices.Sort(missing)
	return missing
}

// loadModule loads api/modules/<module>.yaml.
func loadModule(module string) (*openapi3.T, error) {
	loader := newLoader()
	loader.IsExternalRefsAllowed = true // module files reference ../common.yaml
	path := filepath.Join(apiDir(), "modules", module+".yaml")
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	return doc, nil
}
