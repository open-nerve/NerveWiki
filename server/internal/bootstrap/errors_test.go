package bootstrap

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// shared.Error reaches the platform only by structure: each Kind becomes its
// status, with the error's code, detail, fields and Retry-After, even
// wrapped.
func TestEveryKindBecomesItsProblem(t *testing.T) {
	tests := []struct {
		err        error
		want       string
		retryAfter string
		challenge  string // WWW-Authenticate
	}{
		{shared.Invalid(shared.FieldError{Field: "title", Code: shared.FieldRequired, Message: "is required"}),
			`{"status":422,"code":"validation_failed","title":"Unprocessable Entity","detail":"The request has invalid values.",` +
				`"errors":[{"field":"title","code":"required","message":"is required"}]}`, "", ""},
		{shared.NewError(shared.KindBadRequest, "things.bad_cursor", "d"), `{"status":400,"code":"things.bad_cursor","title":"Bad Request","detail":"d"}`, "", ""},
		{shared.Unauthenticated(), `{"status":401,"code":"unauthorized","title":"Unauthorized","detail":"Authentication is required."}`, "", "Bearer"},
		{shared.Forbidden(), `{"status":403,"code":"forbidden","title":"Forbidden","detail":"Your role does not allow this."}`, "", ""},
		{shared.NewError(shared.KindNotFound, "things.missing", "d"), `{"status":404,"code":"things.missing","title":"Not Found","detail":"d"}`, "", ""},
		{shared.NewError(shared.KindConflict, "things.taken", "d"), `{"status":409,"code":"things.taken","title":"Conflict","detail":"d"}`, "", ""},
		{shared.RateLimited(1500 * time.Millisecond),
			`{"status":429,"code":"rate_limited","title":"Too Many Requests","detail":"Too many requests; retry later."}`, "2", ""},
		{fmt.Errorf("save: %w", shared.ServerBusy(time.Second)),
			`{"status":503,"code":"server_busy","title":"Service Unavailable","detail":"The server is busy; retry shortly."}`, "1", ""},
	}
	errs := httpserver.NewAPIErrors(slog.New(slog.DiscardHandler))
	for _, tt := range tests {
		rec := httptest.NewRecorder()

		errs.Write(rec, httptest.NewRequest(http.MethodGet, "/api/v0/things", nil), tt.err)

		sent := rec.Result().Header // as the status went out, not as set after it
		if got := rec.Body.String(); got != tt.want+"\n" || sent.Get("Retry-After") != tt.retryAfter || sent.Get("WWW-Authenticate") != tt.challenge {
			t.Errorf("Write(%v) = %s Retry-After %q WWW-Authenticate %q, want %s Retry-After %q WWW-Authenticate %q",
				tt.err, got, sent.Get("Retry-After"), sent.Get("WWW-Authenticate"), tt.want, tt.retryAfter, tt.challenge)
		}
	}
}

// The field codes of internal/shared and the contract's FieldError.code enum
// are one closed set (v0.1 design 6.1): a code on only one side fails.
func TestFieldCodesAreTheContractsEnum(t *testing.T) {
	enum := slices.Sorted(slices.Values(apitest.Load(t).Enum(t, "FieldError", "code")))
	codes := slices.Sorted(slices.Values(shared.FieldCodes()))

	if !slices.Equal(codes, enum) {
		t.Errorf("shared.FieldCodes() = %q, want the contract's FieldError.code enum %q", codes, enum)
	}
}
