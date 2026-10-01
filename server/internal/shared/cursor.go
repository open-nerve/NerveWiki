package shared

import (
	"encoding/base64"
	"encoding/json"
)

// cursorVersion is the version of the cursor envelope, v.
const cursorVersion = 1

// cursorEnvelope is what a page cursor encodes (v0.1 design 6.1): the
// envelope's version and the list's own payload.
type cursorEnvelope struct {
	V int             `json:"v"`
	P json.RawMessage `json:"p"`
}

// EncodeCursor returns the page cursor of payload: the JSON {"v":1,"p":…}
// in unpadded base64url. Each list defines its payload by its own sort key,
// ending in the id, so pages neither repeat nor skip a row.
func EncodeCursor(payload any) (string, error) {
	p, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(cursorEnvelope{V: cursorVersion, P: p})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(out), nil
}

// DecodeCursor decodes into payload a cursor that EncodeCursor wrote. It
// accepts only that spelling: the decoded payload must encode back to
// cursor byte for byte. Another version, another member, a second value,
// or another spelling of the same JSON or base64 (blanks, a name's case, a
// repeated member, \r or \n, unused bits set) is InvalidCursor, as is a
// cursor that does not decode, or whose payload is missing, null or not
// the list's. A cursor is not signed: one edited to another payload of the
// list's shape, spelled as EncodeCursor writes it, decodes like any other.
func DecodeCursor(cursor string, payload any) error {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return InvalidCursor()
	}
	var env cursorEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return InvalidCursor()
	}
	// A null payload is refused here: into a pointer, slice or map it
	// decodes to nil, which encodes back to null and would pass the
	// comparison below.
	if len(env.P) == 0 || string(env.P) == "null" {
		return InvalidCursor()
	}
	if err := json.Unmarshal(env.P, payload); err != nil {
		return InvalidCursor()
	}
	if again, err := EncodeCursor(payload); err != nil || again != cursor {
		return InvalidCursor()
	}
	return nil
}

// InvalidCursor reports a cursor that DecodeCursor refuses: 400
// bad_request on the cursor parameter, the request's structure.
func InvalidCursor() *Error {
	return &Error{
		Kind: KindBadRequest, Code: CodeBadRequest, Detail: "The cursor is not one this list can read.",
		Fields: []FieldError{{Field: "cursor", Code: FieldInvalidFormat, Message: "is not a cursor this list can read"}},
	}
}

// Page sizes of a list (api/common.yaml Limit).
const (
	DefaultPageSize = 50
	MaxPageSize     = 100
)

// PageSize returns the page size that limit asks for: DefaultPageSize when
// it is nil. Outside 1–MaxPageSize it is 422 validation_failed on limit; a
// limit that is no integer never gets here, the parameter binding answers
// 400.
func PageSize(limit *int) (int, error) {
	switch {
	case limit == nil:
		return DefaultPageSize, nil
	case *limit < 1 || *limit > MaxPageSize:
		return 0, Invalid(FieldError{Field: "limit", Code: FieldOutOfRange, Message: "must be between 1 and 100"})
	}
	return *limit, nil
}
