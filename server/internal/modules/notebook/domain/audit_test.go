package domain

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The audit list's cursor goes through the envelope and back.
func TestAuditCursorRoundTrip(t *testing.T) {
	want := AuditCursor{CreatedAt: time.Date(2026, 10, 2, 10, 0, 0, 123456000, time.UTC), ID: uuid.NewV7()}
	cursor, err := shared.EncodeCursor(want)
	if err != nil {
		t.Fatal(err)
	}

	var got AuditCursor
	if err := shared.DecodeCursor(cursor, &got); err != nil || !got.CreatedAt.Equal(want.CreatedAt) || got.ID != want.ID {
		t.Errorf("DecodeCursor(%q) = %+v, %v; want %+v", cursor, got, err, want)
	}
}

// Another shape, or the same position spelled otherwise, is no cursor of
// the list.
func TestAuditCursorRejects(t *testing.T) {
	id := "01a0f8f3-ae45-7b22-8a24-7c0e34d0ee78"
	for _, p := range []string{
		`["2026-10-02T10:00:00.123456Z"]`,
		`["2026-10-02T10:00:00.123456Z","` + id + `","x"]`,
		`{"created_at":"2026-10-02T10:00:00.123456Z","id":"` + id + `"}`,
		`["yesterday","` + id + `"]`,
		`["2026-10-02T10:00:00.123456Z","not a uuid"]`,
		`["2026-10-02T12:00:00.123456+02:00","` + id + `"]`, // the same instant, at an offset
		`["2026-10-02T10:00:00.123456Z","01A0F8F3-AE45-7B22-8A24-7C0E34D0EE78"]`,
	} {
		cursor := base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"p":` + p + `}`))
		var got AuditCursor
		if err := shared.DecodeCursor(cursor, &got); !errors.Is(err, shared.InvalidCursor()) {
			t.Errorf("DecodeCursor(%s) = %v, want InvalidCursor", p, err)
		}
	}
}
