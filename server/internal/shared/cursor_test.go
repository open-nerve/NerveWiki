package shared_test

import (
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// page stands for a list's payload.
type page struct {
	After string `json:"after"`
}

func b64(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func TestCursorRoundTrip(t *testing.T) {
	cursor, err := shared.EncodeCursor(page{After: "x??y"})
	if err != nil {
		t.Fatal(err)
	}
	// The envelope is the JSON {"v":1,"p":…} in unpadded base64url: its 28
	// bytes would need padding, and its "x??" encodes to "eD8_".
	if want := b64(`{"v":1,"p":{"after":"x??y"}}`); cursor != want {
		t.Errorf("EncodeCursor() = %q, want %q", cursor, want)
	}

	var got page
	if err := shared.DecodeCursor(cursor, &got); err != nil || got.After != "x??y" {
		t.Errorf("DecodeCursor() = %+v, %v; want the payload back", got, err)
	}
}

// EncodeCursor escapes <, > and & as json.Marshal does; the encoding
// DecodeCursor compares a cursor with must escape them the same way, or no
// cursor holding them would decode.
func TestCursorRoundTripsHTMLCharacters(t *testing.T) {
	want := page{After: "a<b>&c"}
	cursor, err := shared.EncodeCursor(want)
	if err != nil {
		t.Fatal(err)
	}

	var got page
	if err := shared.DecodeCursor(cursor, &got); err != nil || got != want {
		t.Errorf("DecodeCursor(%q) = %+v, %v; want %+v back", cursor, got, err, want)
	}
}

// Anything but a cursor that EncodeCursor wrote is 400 bad_request on the
// cursor parameter.
func TestDecodeCursorRejects(t *testing.T) {
	valid := `{"v":1,"p":{"after":"x"}}`
	c := b64(valid)
	// The envelope's 25 bytes leave 4 unused bits in c's last character:
	// flipping its lowest bit keeps the bytes and changes the spelling.
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	unusedBitSet := c[:len(c)-1] + string(alphabet[strings.IndexByte(alphabet, c[len(c)-1])^1])
	for _, tt := range []struct{ name, cursor string }{
		{"empty", ""},
		{"not base64", "not a cursor!"},
		{"padded", base64.URLEncoding.EncodeToString([]byte(valid + " "))},
		{"standard alphabet", base64.RawStdEncoding.EncodeToString([]byte(`{"v":1,"p":{"after":"??>"}}`))},
		{"not JSON", b64(`{"v":1,`)},
		{"not an object", b64(`[1,{"after":"x"}]`)},
		{"an unknown member", b64(`{"v":1,"p":{"after":"x"},"x":0}`)},
		{"a second value after it", b64(valid + `{}`)},
		{"another version", b64(`{"v":2,"p":{"after":"x"}}`)},
		{"version 0, the zero value", b64(`{"v":0,"p":{"after":"x"}}`)},
		{"no version", b64(`{"p":{"after":"x"}}`)},
		{"no payload", b64(`{"v":1}`)},
		{"a null payload", b64(`{"v":1,"p":null}`)},
		{"a payload of another list", b64(`{"v":1,"p":{"after":5}}`)},
		// The same bytes or the same values, spelled otherwise.
		{"a line feed inside", c[:10] + "\n" + c[10:]},
		{"a carriage return inside", c[:10] + "\r" + c[10:]},
		{"unused bits set", unusedBitSet},
		{"the version spelled 1.0", b64(`{"v":1.0,"p":{"after":"x"}}`)},
		{"an upper-case member name", b64(`{"V":1,"p":{"after":"x"}}`)},
		{"a member twice", b64(`{"v":1,"v":1,"p":{"after":"x"}}`)},
		{"blanks", b64(`{"v":1, "p":{"after":"x"}}`)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got page
			err := shared.DecodeCursor(tt.cursor, &got)

			var se *shared.Error
			want := []shared.FieldError{{Field: "cursor", Code: shared.FieldInvalidFormat, Message: "is not a cursor this list can read"}}
			if !errors.As(err, &se) || se.ProblemStatus() != 400 || se.Code != "bad_request" || !slices.Equal(se.Fields, want) {
				t.Errorf("DecodeCursor(%q) = %v, want 400 bad_request on cursor", tt.cursor, err)
			}
		})
	}
	// The same bytes with the version it expects decode: each case above
	// breaks one thing.
	var got page
	if err := shared.DecodeCursor(c, &got); err != nil {
		t.Errorf("DecodeCursor(valid) = %v", err)
	}
}

// A null payload is refused into a slice too, whose nil encodes back to
// null.
func TestDecodeCursorRejectsANullSlicePayload(t *testing.T) {
	var got []string
	if err := shared.DecodeCursor(b64(`{"v":1,"p":null}`), &got); !errors.Is(err, shared.InvalidCursor()) {
		t.Errorf("DecodeCursor(null into a slice) = %v, want InvalidCursor", err)
	}
}

func TestPageSize(t *testing.T) {
	for _, tt := range []struct {
		limit *int
		want  int
		ok    bool
	}{
		{nil, 50, true},
		{new(1), 1, true},
		{new(100), 100, true},
		{new(0), 0, false},
		{new(101), 0, false},
		{new(-1), 0, false},
	} {
		size, err := shared.PageSize(tt.limit)

		var se *shared.Error
		refused := errors.As(err, &se) && se.ProblemStatus() == 422 && len(se.Fields) == 1 &&
			se.Fields[0].Field == "limit" && se.Fields[0].Code == shared.FieldOutOfRange
		if size != tt.want || (err == nil) != tt.ok || (!tt.ok && !refused) {
			t.Errorf("PageSize(%v) = %d, %v; want %d, ok %v", tt.limit, size, err, tt.want, tt.ok)
		}
	}
}
