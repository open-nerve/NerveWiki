package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// MCPVersion is the protocol's version the serve speaks
// (bootstrap/mcp_client_test.go).
const MCPVersion = "2025-11-25"

// MCP posts a notebook entry's JSON-RPC, by hand (design 3.4.5): /mcp?
// notebook=<id>, its headers as bootstrap/mcp_bodies_test.go's.
func (c *Client) MCP(ctx context.Context, op, notebook string, body []byte) (Record, []byte, error) {
	h := http.Header{}
	h.Set("Accept", "application/json, text/event-stream")
	h.Set("Mcp-Protocol-Version", MCPVersion)
	return c.Send(ctx, Request{Op: op, Method: http.MethodPost, Path: "/mcp?notebook=" + url.QueryEscape(notebook),
		Body: bytes.NewReader(body), ContentType: "application/json", Header: h}, nil)
}

// Call is a tools/call of tool with args, its _meta's members given as
// written (empty for none) and pad spaces after them.
func Call(tool string, args map[string]string, meta string, pad int) []byte {
	a, err := json.Marshal(args)
	if err != nil {
		panic(err) // strings marshal
	}
	name, err := json.Marshal(tool)
	if err != nil {
		panic(err)
	}
	var b bytes.Buffer
	b.WriteString(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":`)
	b.Write(name)
	b.WriteString(`,"arguments":`)
	b.Write(a)
	b.WriteString(`,"_meta":{`)
	b.WriteString(meta)
	b.WriteString(strings.Repeat(" ", pad))
	b.WriteString(`}}}`)
	return b.Bytes()
}

// CountValues counts body's JSON values as bodyshape does (platform/
// httpserver/bodyshape): the body's own and each object, array, string,
// number and literal in it one; a member's name none.
func CountValues(body []byte) (int, error) {
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	// Each open object: whether its next string is a member's name.
	var objects []bool
	var kinds []byte // the open containers, '{' or '['
	n := 0
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, err
		}
		inObject := len(kinds) > 0 && kinds[len(kinds)-1] == '{'
		if delim, ok := tok.(json.Delim); ok && (delim == '}' || delim == ']') {
			kinds = kinds[:len(kinds)-1]
			if delim == '}' {
				objects = objects[:len(objects)-1]
			}
			continue
		}
		if inObject && objects[len(objects)-1] {
			objects[len(objects)-1] = false // the name read; its value next
			continue
		}
		if inObject {
			objects[len(objects)-1] = true // a value read; a name next
		}
		n++
		if delim, ok := tok.(json.Delim); ok {
			kinds = append(kinds, byte(delim))
			if delim == '{' {
				objects = append(objects, true)
			}
		}
	}
	if len(kinds) != 0 {
		return 0, fmt.Errorf("%d containers left open", len(kinds))
	}
	return n, nil
}
