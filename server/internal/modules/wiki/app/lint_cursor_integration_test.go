package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestLintJudgesRealCursorKeys(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	for _, tc := range []struct {
		name, inner string
		prefix      []byte
		status      int
		code        string
	}{
		{name: "canonical-YQ", inner: "YQ", prefix: []byte("a"), status: 404, code: "notebook.not_found"},
		{name: "noncanonical-YR", inner: "YR", status: 400, code: "bad_request"},
		{
			name: "prefix-256", inner: base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte("a"), 256)),
			prefix: bytes.Repeat([]byte("a"), 256), status: 404, code: "notebook.not_found",
		},
		{
			name: "prefix-257", inner: base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte("a"), 257)),
			status: 400, code: "bad_request",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, trace, events, _ := lintExecFixture(t)
			var bound *domain.LintPosition
			deps.Cursors = lintExecCursors{
				decode: func(raw string) (DecodedCursor, error) {
					trace.add("decode")
					cursor, err := linking.DecodeLintCursor(raw)
					if err != nil {
						return nil, err
					}
					return lintExecDecoded(func(scope LintScope, rule domain.LintRule) (domain.LintPosition, error) {
						trace.add("bind")
						if scope.Contract != nil || scope.AreaRootID != nil {
							return domain.LintPosition{}, errors.New("S54 fixture requires implicit scope")
						}
						position, err := cursor.PositionFor(linking.CursorBinding{NotebookID: scope.NotebookID}, linking.Rule(rule))
						if err != nil {
							return domain.LintPosition{}, err
						}
						out := domain.LintPosition{
							Begin: position.Begin, Pages: position.Pages, Prefix: slices.Clone(position.Prefix),
							Hash: position.Hash, ID: position.ID, CreatedAt: position.CreatedAt,
						}
						bound = &out
						return out, nil
					}), nil
				},
				encode: func(LintScope, domain.LintRule, domain.LintPosition) (string, error) {
					trace.add("encode")
					return "", errors.New("S54 hidden notebook must exit before Encode")
				},
			}
			deps.Authorizer = lintExecAuth(func(_ context.Context, actor shared.Actor, action shared.Action, target shared.Target) (shared.Grant, error) {
				trace.add("authorize")
				if actor.UserID != lintExecID(10) || action != domain.ActionLintRead || target.WorkspaceID != lintExecID(2) || target.NotebookID != lintExecID(1) {
					return shared.Grant{}, errors.New("S54 unexpected authorization identity")
				}
				return shared.Grant{}, shared.ErrNotVisible
			})

			// The compact envelope and its field order are independent wire input.
			// Do not construct these samples through either production encoder.
			payload := `{"v":1,"p":{"v":1,"r":"wanted","n":"00000000-0000-0000-0000-000000000001","s":"I","c":1,"k":"` + tc.inner + `","h":"` + hash + `"}}`
			raw := base64.RawURLEncoding.EncodeToString([]byte(payload))
			if len(raw) > 2048 {
				t.Fatal("S54 fixture exceeded outer cursor bound")
			}
			rule := "wanted"
			got, err := NewLint(deps).Execute(lintExecActor(t.Context(), 10), LintScope{NotebookID: lintExecID(1)}, LintQuery{
				Rule: &rule, Limit: lintExecPtr(1), Cursor: &raw,
			})
			var problem *shared.Error
			if !errors.As(err, &problem) {
				t.Fatalf("S54 cursor precedence: expected %d/%s, got %v", tc.status, tc.code, err)
			}
			if problem.ProblemStatus() != tc.status || problem.Code != tc.code {
				t.Errorf("S54 cursor precedence: got %d/%s; want %d/%s", problem.ProblemStatus(), problem.Code, tc.status, tc.code)
			}
			if !reflect.DeepEqual(got, domain.LintResult{}) {
				t.Errorf("S54 error exposed partial result: %+v", got)
			}
			calls := []string{"decode"}
			if tc.status == 400 {
				if len(problem.Fields) != 1 || problem.Fields[0].Field != "cursor" || problem.Fields[0].Code != "invalid_format" {
					t.Errorf("S54 cursor field: got %+v; want cursor/invalid_format", problem.Fields)
				}
				if bound != nil {
					t.Errorf("S54 rejected key reached binding: %+v", bound)
				}
			} else {
				calls = []string{"decode", "bind", "workspace", "authorize"}
				want := domain.LintPosition{Pages: 1, Prefix: tc.prefix, Hash: hash}
				if !reflect.DeepEqual(bound, &want) {
					t.Errorf("S54 valid key position: got %+v; want %+v", bound, want)
				}
				if len(problem.Fields) != 0 {
					t.Errorf("S54 valid key returned field errors: %+v", problem.Fields)
				}
			}
			if gotCalls := trace.copy(); !slices.Equal(gotCalls, calls) {
				t.Errorf("S54 cursor calls=%v; want %v", gotCalls, calls)
			}
			select {
			case event := <-events.c:
				t.Errorf("S54 hidden request reached real Runs: %+v", event)
			default:
			}
		})
	}
}
