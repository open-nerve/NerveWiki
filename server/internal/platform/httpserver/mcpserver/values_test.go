package mcpserver

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"testing"
)

// Pattern's bound on a body's JSON values (M9/P2 S1 review A2-I1).

// valuesShapes are the shapes a client makes many values of in few bytes,
// as members of an object of n values: many members, an array of empty
// arrays, an array of numbers.
func valuesShapes() []struct {
	name    string
	members func(n int) string
} {
	return []struct {
		name    string
		members func(n int) string
	}{
		{"many members", func(n int) string {
			members := make([]string, n)
			for i := range members {
				members[i] = fmt.Sprintf(`"m%d":1`, i)
			}
			return strings.Join(members, ",")
		}},
		{"nested empty arrays", func(n int) string { return `"a":[` + strings.TrimSuffix(strings.Repeat("[],", n-1), ",") + `]` }},
		{"a long flat array", func(n int) string { return `"a":[` + strings.TrimSuffix(strings.Repeat("1,", n-1), ",") + `]` }},
	}
}

// A body of MaxBodyValues values is the SDK's to answer, in each shape; a
// value more is 400 bad_request before it, the body's too_long naming the
// bound: of a call past MaxOtherBytes, the large bodies' threshold, or
// not, of a ping, before the 413 of a ping past it and the 403 of a
// session's access token.
func TestABodyPastMaxBodyValuesIs400(t *testing.T) {
	s := serveMCP(t, setup{})
	// The message, jsonrpc, id, method, params, its name, its arguments,
	// their notebook and _meta: 9 values; a ping's message, jsonrpc, id,
	// method, params and _meta: 6.
	call := func(members string) string {
		return `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"notebook":"` +
			visible.String() + `"},"_meta":{` + members + `}}}`
	}
	ping := func(members string) string {
		return `{"jsonrpc":"2.0","id":3,"method":"ping","params":{"_meta":{` + members + `}}}`
	}
	type request struct {
		name, token, body string
		status            int
	}
	var requests []request
	for _, shape := range valuesShapes() {
		pingAt := http.StatusOK
		if shape.name == "many members" {
			pingAt = http.StatusRequestEntityTooLarge // past 64 KiB, and not a call
		}
		requests = append(requests,
			request{"a call of " + shape.name + " at the bound", patToken, call(shape.members(MaxBodyValues - 9)), http.StatusOK},
			request{"a call of " + shape.name + " past it", patToken, call(shape.members(MaxBodyValues - 8)), http.StatusBadRequest},
			request{"a ping of " + shape.name + " at the bound", patToken, ping(shape.members(MaxBodyValues - 6)), pingAt},
			request{"a ping of " + shape.name + " past it", patToken, ping(shape.members(MaxBodyValues - 5)), http.StatusBadRequest})
	}
	requests = append(requests,
		request{"a session's access token and a call at the bound", sessionToken, call(valuesShapes()[0].members(MaxBodyValues - 9)),
			http.StatusForbidden},
		request{"a session's access token and a call past it", sessionToken, call(valuesShapes()[0].members(MaxBodyValues - 8)),
			http.StatusBadRequest})
	wantField := map[string]any{"field": "", "code": "too_long",
		"message": "has more than 10,000 JSON values: each object, array, string, number, true, false and null is one"}
	long := 0
	for _, r := range requests {
		if len(r.body) > MaxOtherBytes && r.status == http.StatusOK {
			long++
		}
		a := s.post(t, context.Background(), Pattern, r.token, r.body, v2025())
		p := a.decoded(t)
		fields, _ := p["errors"].([]any)
		var field map[string]any
		if len(fields) == 1 {
			field, _ = fields[0].(map[string]any)
		}
		switch {
		case a.status != r.status:
			t.Errorf("%s, %d bytes = %d %.300s, want %d", r.name, len(r.body), a.status, a.body, r.status)
		case r.status == http.StatusOK && p["result"] == nil:
			t.Errorf("%s = %.300s, want the SDK's result", r.name, a.body)
		case r.status == http.StatusBadRequest && (p["code"] != "bad_request" ||
			p["detail"] != "The request body does not match the API description." || len(fields) != 1 || !maps.Equal(field, wantField)):
			t.Errorf("%s = %.300s, want bad_request, the body too_long", r.name, a.body)
		}
	}
	if long == 0 {
		t.Error("no body past MaxOtherBytes is served: a large body at the bound is not covered")
	}
}
