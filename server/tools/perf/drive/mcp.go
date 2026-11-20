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

// MaxMCPBodyBytes is /mcp's body bound (bootstrap/mcp.go's mcpBodyLimit):
// a call's 5 MiB of content and 1 MiB besides, each byte at JSON's longest
// escape, six bytes, and 64 KiB.
const MaxMCPBodyBytes = 6*(MaxContentBytes+1<<20) + 64<<10

// EscapedContent is MaxContentBytes of '<', each of which encoding/json's
// Marshal, escaping HTML, writes as six bytes, \u003c: M9/P2's content at
// the bound (02-P2-writes-history.md:2704).
func EscapedContent() string { return strings.Repeat("<", MaxContentBytes) }

// MCPShapeAtBound is a shape with each MCP body at /mcp's bound,
// MaxMCPBodyBytes, as M9/P2 measured the large bodies (02-P2-writes-
// history.md:353, :2714; M12/P4/S3's review B, I2): 1 a write of
// EscapedContent, _meta a long string to the bound; 2a, 2b and 2c that
// write, _meta MaxBodyValues values in the body in the shape, the long
// string among them; 3 a value more, a shape each credential; 4 the first
// two 1, the third a REST write of the same content; L a write of "x",
// _meta the long string alone.
func MCPShapeAtBound(shape, prefix string) ([3]MCPSend, error) {
	var out [3]MCPSend
	content := EscapedContent()
	// write is the credential's write of content, _meta the members and a
	// long string after them that brings the body to the bound.
	write := func(cred int, content, members string) ([]byte, error) {
		args := map[string]string{"path": fmt.Sprintf("%s/%s-%d.md", prefix, shape, cred), "content": content}
		if members != "" {
			members += ","
		}
		n := MaxMCPBodyBytes - len(client.Call("write", args, members+`"pad":""`, 0))
		if n < 0 {
			return nil, fmt.Errorf("shape %s: %d bytes past the bound before its long string", shape, -n)
		}
		return client.Call("write", args, members+`"pad":"`+strings.Repeat("a", n)+`"`, 0), nil
	}
	// The values of a write of no members, as MCPShape's; the long string
	// is one more.
	base, err := client.CountValues(client.Call("write", map[string]string{"path": "p", "content": "x"}, "", 0))
	if err != nil {
		return out, err
	}
	for c := range out {
		var b []byte
		switch shape {
		case "1":
			b, err = write(c, content, "")
		case "2a", "2b", "2c":
			b, err = write(c, content, valueMembers(shape[1], MaxBodyValues-base-1))
		case "3":
			b, err = write(c, content, valueMembers("abc"[c], MaxBodyValues-base))
		case "4":
			if c == 2 {
				out[c].Content = content
				continue
			}
			b, err = write(c, content, "")
		case "L":
			b, err = write(c, "x", "")
		default:
			return out, fmt.Errorf("no shape %q at the bound: 1, 2a, 2b, 2c, 3, 4 or L", shape)
		}
		if err != nil {
			return out, err
		}
		out[c].Body = b
	}
	return out, nil
}

// MCPRun is B3's: Reps times, Gap apart, the three credentials send a
// shape's at once to the notebook's entry, at /mcp's bound when Bound
// (MCPShapeAtBound; Seed and Size then unused); shape 4's REST write goes
// to a page of the notebook made first.
type MCPRun struct {
	Clients  [3]*client.Client
	Notebook string
	Shape    string
	Seed     uint64
	Size     int
	Bound    bool
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
		at := fmt.Sprintf("%s/%d", prefix, rep)
		var sends [3]MCPSend
		var err error
		if m.Bound {
			sends, err = MCPShapeAtBound(m.Shape, at)
		} else {
			sends, err = MCPShape(m.Shape, at, m.Seed, m.Size)
		}
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
