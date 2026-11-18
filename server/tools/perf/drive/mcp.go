package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/open-nerve/NerveWiki/server/tools/perf/client"
)

// MaxBodyValues is the MCP entry's bound on a body's JSON values
// (platform/httpserver/mcpserver/edge.go), MaxOtherBytes the bytes past
// which a body takes a large body's slot.
const (
	MaxBodyValues = 10_000
	MaxOtherBytes = 64 << 10
)

// valueMembers are _meta's members of n values in each of the shapes
// mcpserver's valuesShapes makes (values_test.go), written out: many
// members, nested empty arrays, a long flat array.
func valueMembers(shape byte, n int) string {
	switch shape {
	case 'a':
		m := make([]string, n)
		for i := range m {
			m[i] = fmt.Sprintf(`"m%d":1`, i)
		}
		return strings.Join(m, ",")
	case 'b':
		return `"a":[` + strings.TrimSuffix(strings.Repeat("[],", n-1), ",") + `]`
	case 'c':
		return `"a":[` + strings.TrimSuffix(strings.Repeat("1,", n-1), ",") + `]`
	}
	panic(fmt.Sprintf("no shape %q", shape))
}

// MCPSend is what a credential sends in a shape: an MCP body, or a REST
// content write of Content when Body is nil.
type MCPSend struct {
	Body    []byte
	Content string
}

// MCPShape is the three credentials' sends of a shape at the rep's path
// prefix (design 3.6's B3): 1 a write of a 5 MiB normal body each; 2a, 2b
// and 2c a write whose _meta is of MaxBodyValues values in the body, past
// MaxOtherBytes, each; 3 a value more, a shape each credential; 4 the
// first two 1, the third a REST write of the same body.
func MCPShape(shape, prefix string, seed uint64, size int) ([3]MCPSend, error) {
	var out [3]MCPSend
	body := NormalBody(seed, size)
	write := func(cred int, content, meta string) []byte {
		args := map[string]string{"path": fmt.Sprintf("%s/%s-%d.md", prefix, shape, cred), "content": content}
		b := client.Call("write", args, meta, 0)
		if len(b) <= MaxOtherBytes {
			b = client.Call("write", args, meta, MaxOtherBytes+8<<10-len(b))
		}
		return b
	}
	// The values of a write of no members: the message, jsonrpc, id,
	// method, params, name, arguments, path, content and _meta.
	base, err := client.CountValues(client.Call("write", map[string]string{"path": "p", "content": "x"}, "", 0))
	if err != nil {
		return out, err
	}
	switch shape {
	case "1":
		for c := range out {
			out[c].Body = write(c, body, "")
		}
	case "2a", "2b", "2c":
		for c := range out {
			out[c].Body = write(c, "x", valueMembers(shape[1], MaxBodyValues-base))
		}
	case "3":
		for c, s := range []byte("abc") {
			out[c].Body = write(c, "x", valueMembers(s, MaxBodyValues-base+1))
		}
	case "4":
		out[0].Body, out[1].Body, out[2].Content = write(0, body, ""), write(1, body, ""), body
	default:
		return out, fmt.Errorf("no shape %q: 1, 2a, 2b, 2c, 3 or 4", shape)
	}
	return out, nil
}

// MCPRun is B3's: Reps times, Gap apart, the three credentials send a
// shape's at once to the notebook's entry; shape 4's REST write goes to a
// page of the notebook made first.
type MCPRun struct {
	Clients  [3]*client.Client
	Notebook string
	Shape    string
	Seed     uint64
	Size     int
	Reps     int
	Gap      time.Duration
}

// Run runs it: the records are the clients' log's, op mcp-<shape> and
// rest-<shape>.
func (m MCPRun) Run(ctx context.Context) error {
	prefix := fmt.Sprintf("b3-%d", time.Now().UnixMilli())
	var target page
	if m.Shape == "4" {
		p, err := createPage(ctx, m.Clients[2], "setup-page", m.Notebook, "", prefix, "x")
		if err != nil {
			return err
		}
		target = p
	}
	for rep := range m.Reps {
		if rep > 0 {
			if err := sleep(ctx, m.Gap); err != nil {
				return err
			}
		}
		sends, err := MCPShape(m.Shape, fmt.Sprintf("%s/%d", prefix, rep), m.Seed, m.Size)
		if err != nil {
			return err
		}
		var wg sync.WaitGroup
		for c, s := range sends {
			wg.Go(func() {
				if s.Body != nil {
					_, _, _ = m.Clients[c].MCP(ctx, "mcp-"+m.Shape, m.Notebook, s.Body)
					return
				}
				// Each rep's content another, so that each is a write.
				in, err := json.Marshal(map[string]any{"content": changed(s.Content, rep), "base_revision": target.Revision})
				if err != nil {
					return
				}
				rec, answer, err := m.Clients[c].Send(ctx, client.Request{Op: "rest-" + m.Shape, Method: http.MethodPut,
					Path: "/api/v0/pages/" + url.PathEscape(target.ID) + "/content", Body: bytes.NewReader(in),
					ContentType: "application/json"}, nil)
				var p page
				if err == nil && rec.Status == http.StatusOK && json.Unmarshal(answer, &p) == nil {
					target.Revision = p.Revision
				}
			})
		}
		wg.Wait()
	}
	return nil
}
