// Package httpserver provides nervewiki's HTTP platform: the server lifecycle,
// the fixed middleware chain, the router with the health endpoints,
// problem+json errors, what the API operations share (API: the error mapping
// and the per-route middlewares), and support for long-lived routes.
package httpserver

import (
	"encoding/json"
	"net/http"
	"uuid"
)

// ContentTypeProblem is the media type of RFC 9457 problem details.
const ContentTypeProblem = "application/problem+json"

// Codes of the problems the platform itself reports. The other platform
// codes (validation_failed, forbidden …) come from domain errors through
// ProblemError; module codes are namespaced by module, e.g. "page.locked"
// (v0.1 design 6.1).
const (
	CodeBadRequest      = "bad_request"
	CodeUnauthorized    = "unauthorized"
	CodeNotFound        = "not_found"
	CodePayloadTooLarge = "payload_too_large"
	CodeRateLimited     = "rate_limited"
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
	// Lock, Locks and EndedBy are the members only some codes carry (M5
	// design 4.5, M6/P4 design 4.1): page.locked names the lock,
	// linking.pages_locked the locks, page.edit_session_unlocked who
	// unlocked.
	Lock    *ProblemLock   `json:"lock,omitempty"`
	Locks   []ProblemLock  `json:"locks,omitempty"`
	EndedBy *ProblemPerson `json:"ended_by,omitempty"`
}

// ProblemLock is a page's edit lock: the page locked and who holds it.
type ProblemLock struct {
	PageID      uuid.UUID `json:"page_id"`
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
}

// ProblemPerson is an account a problem names.
type ProblemPerson struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
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
