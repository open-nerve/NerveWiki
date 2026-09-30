package httpadapter_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// The platform reads the parameter out of oapi-codegen's binding errors by
// their shape (httpserver.APIErrors.BadRequest), and its own tests use
// lookalikes. The generated types are checked here, where they are declared,
// so that an oapi-codegen upgrade that changes them fails before an
// operation with parameters depends on them.
func TestGeneratedBindingErrorsNameTheParameter(t *testing.T) {
	cause := errors.New("cause")
	tests := []struct {
		err  error
		code string
	}{
		{&gen.UnescapedCookieParamError{ParamName: "p", Err: cause}, "invalid_format"},
		{&gen.UnmarshalingParamError{ParamName: "p", Err: cause}, "invalid_format"},
		{&gen.RequiredParamError{ParamName: "p"}, "required"},
		{&gen.RequiredHeaderError{ParamName: "p", Err: cause}, "required"},
		{&gen.InvalidParamFormatError{ParamName: "p", Err: cause}, "invalid_format"},
		{&gen.TooManyValuesForParamError{ParamName: "p", Count: 2}, "invalid_format"},
	}
	errs := httpserver.NewAPIErrors(slog.New(slog.DiscardHandler))
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%T", tt.err), func(t *testing.T) {
			rec := httptest.NewRecorder()

			errs.BadRequest(rec, httptest.NewRequest(http.MethodGet, "/api/v0/instance", nil), tt.err)

			var p httpserver.Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || rec.Code != http.StatusBadRequest ||
				len(p.Errors) != 1 || p.Errors[0].Field != "p" || p.Errors[0].Code != tt.code {
				t.Errorf("BadRequest = %d %s, want 400 naming p with %s", rec.Code, rec.Body, tt.code)
			}
		})
	}
}
