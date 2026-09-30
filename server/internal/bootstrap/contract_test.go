package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The whole-program tests (M0/P4 design 3.6): the wired app against the
// contract, operation by operation, so an operation added later is covered
// without a new test.

// The routes registered under /api/, but for the platform's fallback, are the
// contract's operations: a route the contract does not describe, or an
// operation no module serves, fails here.
func TestAPIRoutesAreTheContractsOperations(t *testing.T) {
	contract := apitest.Load(t)
	a := buildApp(t, testConfig(t, unreachableDB, false), sampleMigrations())

	var got []string
	for _, pattern := range a.router.Patterns() {
		path := pattern
		if _, rest, ok := strings.Cut(pattern, " "); ok {
			path = rest
		}
		if strings.HasPrefix(path, "/api/") && pattern != "/api/" {
			got = append(got, pattern)
		}
	}
	slices.Sort(got)
	var want []string
	for _, op := range contract.Operations() {
		want = append(want, op.Pattern())
	}
	if len(want) == 0 || !slices.Equal(got, want) {
		t.Errorf("routes under /api/ = %q, want the contract's operations %q", got, want)
	}
}

// Every operation's default response, the problem, declares the headers a
// problem may carry: Retry-After (shared.RateLimited, shared.ServerBusy) and
// WWW-Authenticate (every 401). Each module declares its own Problem
// response, so a module that leaves one out fails here.
func TestEveryProblemResponseDeclaresItsHeaders(t *testing.T) {
	contract := apitest.Load(t)

	for _, op := range contract.Operations() {
		for _, header := range []string{"Retry-After", "WWW-Authenticate"} {
			if !slices.Contains(op.ProblemHeaders, header) {
				t.Errorf("%s: the default response declares %q, want %s among them", op.Pattern(), op.ProblemHeaders, header)
			}
		}
	}
}

// The operations the modules declare public are exactly the contract's
// operations with security: [] (M1/P1 design 3.9), as the wired app answers
// them without a token: every other operation answers 401 with the Bearer
// challenge before its handler runs, a public one never does. A module list
// that misses an operation, or names one the contract protects, fails here.
func TestPublicOperationsAreTheContractsPublicOperations(t *testing.T) {
	contract := apitest.Load(t)
	a := buildApp(t, testConfig(t, unreachableDB, false), sampleMigrations())

	for _, op := range contract.Operations() {
		req := httptest.NewRequest(op.Method, op.Target(), nil)
		rec := httptest.NewRecorder()

		a.router.ServeHTTP(rec, req)

		res := rec.Result()
		contract.CheckResponse(t, req, res)
		var p httpserver.Problem
		_ = json.Unmarshal(rec.Body.Bytes(), &p)
		// The detail is the authentication middleware's: a handler that
		// finds no caller answers 401 too, but only after it has run.
		turnedAway := res.StatusCode == http.StatusUnauthorized && res.Header.Get("WWW-Authenticate") == "Bearer" &&
			p.Detail == "This operation requires a bearer token."
		if turnedAway == op.Public {
			t.Errorf("%s without a token = %d %s, WWW-Authenticate %q; want the middleware's 401 exactly when the contract protects it (public: %v)",
				op.Pattern(), res.StatusCode, rec.Body, res.Header.Get("WWW-Authenticate"), op.Public)
		}
	}
}

// Every operation with a JSON body answers a body that breaks its structure
// with 400 and every broken field, and lets null through where the schema
// allows it; a body that can be read two ways (a property twice, at the top
// and in a nested object; bytes that are not UTF-8) gets 400 too, before its
// structure is checked (M1/P1 design 3.9). Operations that need a token get
// a valid one, so the body check, not the authentication, answers.
func TestBodiesThatBreakTheStructureAnswer400(t *testing.T) {
	contract := apitest.Load(t)
	base := startApp(t, testConfig(t, pgtest.NewDatabase(t), false), migrations.FS())
	token := registerAccount(t, contract, base, "body-cases@example.com").AccessToken

	cases := 0
	for _, op := range contract.Operations() {
		if !op.HasJSONBody() {
			continue
		}
		for _, c := range op.BodyCases() {
			cases++
			t.Run(op.Pattern()+"/"+c.Name, func(t *testing.T) {
				auth := token
				if op.Public {
					auth = ""
				}
				req := newRequest(t, op.Method, base+op.Target(), auth, c.Body)
				res, body := sendRequest(t, req)

				contract.CheckResponse(t, req, res)
				if c.Accepted {
					if res.StatusCode == http.StatusBadRequest {
						t.Errorf("%s = 400 %s, want it accepted", c.Body, body)
					}
					return
				}
				var p struct {
					Code   string                 `json:"code"`
					Errors []apitest.FieldProblem `json:"errors"`
				}
				if err := json.Unmarshal(body, &p); err != nil {
					t.Fatalf("decode %s: %v", body, err)
				}
				if res.StatusCode != http.StatusBadRequest || p.Code != "bad_request" || !slices.Equal(p.Errors, c.Fields) {
					t.Errorf("%s = %d %s, want 400 bad_request with %v", c.Body, res.StatusCode, body, c.Fields)
				}
			})
		}
	}
	// register has a body: none found means the derivation broke.
	if cases == 0 {
		t.Fatal("no body case derived from the contract")
	}
}

// The answer to a body that breaks the structure stays small, whatever the
// body (M1/P1 design 3.9): each problem at a path of at most 256 bytes, at
// most 16 problems. Nerve's review sent such bodies to an anonymous
// operation: 16 problems under one name of about 1 MiB, each repeating the
// name, got an answer of about 100 MB. 32 KiB holds 16 paths of 256 bytes
// that the encoding writes 6 bytes each, with their messages.
func TestTheAnswerToABrokenBodyStaysSmall(t *testing.T) {
	contract := apitest.Load(t)
	base := startApp(t, testConfig(t, pgtest.NewDatabase(t), false), migrations.FS())

	angles := strings.Repeat("<", 1<<20-400)
	var twice, notUTF8 []string
	for i := range 16 {
		key := fmt.Sprintf(`"k%02d"`, i)
		twice = append(twice, key+":1,"+key+":1")
		notUTF8 = append(notUTF8, key+":\"\xff\"")
	}
	tests := []struct {
		name string
		body string
	}{
		{"names twice under a name of 1 MiB", `{"` + angles + `":{` + strings.Join(twice, ",") + `}}`},
		{"strings that are not UTF-8 under a name of 1 MiB", `{"` + angles + `":{` + strings.Join(notUTF8, ",") + `}}`},
		{"names twice under 9,990 names of 90 bytes",
			strings.Repeat(`{"`+angles[:90]+`":`, 9990) + `{` + strings.Join(twice, ",") + `}` + strings.Repeat("}", 9990)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newRequest(t, http.MethodPost, base+"/api/v0/auth/register", "", []byte(tt.body))
			res, body := sendRequest(t, req)

			// The answer is not printed: on a failure it may be 100 MB.
			contract.CheckResponse(t, req, res)
			var p struct {
				Code   string                 `json:"code"`
				Errors []apitest.FieldProblem `json:"errors"`
			}
			if err := json.Unmarshal(body, &p); err != nil {
				t.Fatalf("decode the answer of %d bytes: %v", len(body), err)
			}
			if res.StatusCode != http.StatusBadRequest || p.Code != "bad_request" || len(p.Errors) == 0 {
				t.Errorf("answer = %d %s with %d problems, want 400 bad_request with some", res.StatusCode, p.Code, len(p.Errors))
			}
			if len(body) > 32<<10 {
				t.Errorf("the answer is %d bytes, want at most 32 KiB", len(body))
			}
			longest := 0
			for _, f := range p.Errors {
				longest = max(longest, len(f.Field))
			}
			if longest > 256 {
				t.Errorf("a path of %d bytes, want at most 256", longest)
			}
		})
	}
}

// Every path or query parameter whose Go type rejects some strings answers
// a wrong value with 400 that names it (M1/P1 design 3.9), before
// authentication: parameters bind first, so no token is sent.
func TestParametersThatDoNotBindAnswer400(t *testing.T) {
	contract := apitest.Load(t)
	base := startApp(t, testConfig(t, pgtest.NewDatabase(t), false), migrations.FS())

	cases := 0
	for _, op := range contract.Operations() {
		for _, c := range op.ParamCases() {
			cases++
			t.Run(op.Pattern()+"/"+c.Name, func(t *testing.T) {
				req := newRequest(t, op.Method, base+c.Target, "", nil)
				res, body := sendRequest(t, req)

				contract.CheckResponse(t, req, res)
				var p struct {
					Code   string                 `json:"code"`
					Errors []apitest.FieldProblem `json:"errors"`
				}
				if err := json.Unmarshal(body, &p); err != nil {
					t.Fatalf("decode %s: %v", body, err)
				}
				want := []apitest.FieldProblem{{Field: c.Field, Code: "invalid_format"}}
				if res.StatusCode != http.StatusBadRequest || p.Code != "bad_request" || !slices.Equal(p.Errors, want) {
					t.Errorf("%s = %d %s, want 400 bad_request with %v", c.Target, res.StatusCode, body, want)
				}
			})
		}
	}
	// revokeApiToken has a uuid in its path (M1/P3): none found means the
	// derivation broke.
	if cases == 0 {
		t.Fatal("no parameter case derived from the contract")
	}
}

// Every operation that needs a token takes a personal access token as it
// takes an access token (M1/P3 design 3.9): called with a valid one of an
// account of its own, it answers anything but 401. The body, when there is
// one, is empty: the structure check answers after the authentication.
func TestEveryOperationAcceptsAPersonalAccessToken(t *testing.T) {
	contract := apitest.Load(t)
	base := startApp(t, testConfig(t, pgtest.NewDatabase(t), false), migrations.FS())

	protected := 0
	for i, op := range contract.Operations() {
		if op.Public {
			continue
		}
		protected++
		t.Run(op.Pattern(), func(t *testing.T) {
			access := registerAccount(t, contract, base, fmt.Sprintf("pat-%d@example.com", i)).AccessToken
			pat := createToken(t, contract, base, access)
			var body []byte
			if op.HasJSONBody() {
				body = []byte(`{}`)
			}
			req := newRequest(t, op.Method, base+op.Target(), pat, body)
			res, answer := sendRequest(t, req)

			contract.CheckResponse(t, req, res)
			if res.StatusCode == http.StatusUnauthorized {
				t.Errorf("%s with a personal access token = 401 %s, want it authenticated", op.Pattern(), answer)
			}
		})
	}
	if protected == 0 {
		t.Fatal("no operation needs a token")
	}
}

// createToken creates a personal access token with accessToken, the
// password being registerAccount's, and returns it.
func createToken(t *testing.T, contract *apitest.Contract, base, accessToken string) string {
	t.Helper()
	req := newRequest(t, http.MethodPost, base+"/api/v0/me/api-tokens", accessToken,
		[]byte(`{"name":"whole program","current_password":"Tr0ub4dor&3"}`))
	contract.CheckRequest(t, req)
	res, body := sendRequest(t, req)
	contract.CheckResponse(t, req, res)
	var created struct {
		Token string `json:"token"`
	}
	if res.StatusCode != http.StatusCreated || json.Unmarshal(body, &created) != nil {
		t.Fatalf("create a token = %d %s, want 201 with the token", res.StatusCode, body)
	}
	return created.Token
}

// authTokens is the AuthTokens answer.
type authTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// registerAccount signs up email through the app and returns its tokens.
func registerAccount(t *testing.T, contract *apitest.Contract, base, email string) authTokens {
	t.Helper()
	req := newRequest(t, http.MethodPost, base+"/api/v0/auth/register", "",
		[]byte(`{"email":"`+email+`","password":"Tr0ub4dor&3"}`))
	contract.CheckRequest(t, req)
	res, body := sendRequest(t, req)
	contract.CheckResponse(t, req, res)
	var tokens authTokens
	if res.StatusCode != http.StatusCreated || json.Unmarshal(body, &tokens) != nil {
		t.Fatalf("register %s = %d %s, want 201 with tokens", email, res.StatusCode, body)
	}
	return tokens
}

// newRequest builds a request with an optional bearer token and JSON body.
func newRequest(t *testing.T, method, url, token string, body []byte) *http.Request {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// sendRequest sends req and returns the response with its body read, which
// also stays readable in res.Body.
func sendRequest(t *testing.T, req *http.Request) (*http.Response, []byte) {
	t.Helper()
	res, err := client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	res.Body = io.NopCloser(bytes.NewReader(body))
	return res, body
}
