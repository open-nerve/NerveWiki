package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteProblem(rec, Problem{
		Status: http.StatusUnprocessableEntity,
		Code:   "pages.title_taken",
		Title:  "The title is taken",
		Errors: []FieldError{{Field: "title", Code: "taken", Message: "another page has this title"}},
	})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
	if ct := rec.Result().Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	want := `{"status":422,"code":"pages.title_taken","title":"The title is taken",` +
		`"errors":[{"field":"title","code":"taken","message":"another page has this title"}]}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestWriteProblemOmitsEmptyOptionalMembers(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteProblem(rec, Problem{Status: http.StatusNotFound, Code: CodeNotFound, Title: "Not Found"})

	want := `{"status":404,"code":"not_found","title":"Not Found"}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}
