// Package httpserver provides nervewiki's HTTP platform: the server lifecycle,
// the fixed middleware chain, the router with the health endpoints,
// problem+json errors, what the API operations share (API: the error mapping
// and the per-route middlewares), and support for long-lived routes.
package httpserver

import (
	"encoding/json"
	"net/http"
)

// ContentTypeProblem is the media type of RFC 9457 problem details.
const ContentTypeProblem = "application/problem+json"

// Codes of the problems the platform itself reports. The other platform
// codes (validation_failed, forbidden …) come from domain errors through
// ProblemError; module codes are namespaced by module, e.g. "page.locked"
// (v0.1 design 6.1).
const (
	CodeBadRequest      = "bad_request"
	CodeNotFound        = "not_found"
	CodePayloadTooLarge = "payload_too_large"
	CodeInternal        = "internal_error"
	CodeNotReady        = "not_ready"
)

// Problem is an RFC 9457 problem details body (v0.1 design 6.1). Code is the
// stable identifier clients branch on; Title is for humans.
type Problem struct {
	Status int          `json:"status"`
	Code   string       `json:"code"`
	Title  string       `json:"title"`
	Detail string       `json:"detail,omitempty"`
	Errors []FieldError `json:"errors,omitempty"`
}

// FieldError points at one invalid field of a request. Code is one of a
// closed set of field codes; clients translate it rather than show Message.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteProblem sends p with status p.Status as application/problem+json.
func WriteProblem(w http.ResponseWriter, p Problem) {
	w.Header().Set("Content-Type", ContentTypeProblem)
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p) // nothing useful to do if the client is gone
}
