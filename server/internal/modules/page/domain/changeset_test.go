package domain_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A log's cursor is written in UTC, the one spelling DecodeCursor takes.
func TestChangesetCursor(t *testing.T) {
	at := time.Date(2026, 10, 1, 20, 0, 0, 123456000, time.FixedZone("CST", 8*3600))
	c := domain.ChangesetCursor{At: at, ID: uuid.NewV7()}
	s, err := shared.EncodeCursor(c)
	if err != nil {
		t.Fatal(err)
	}
	var got domain.ChangesetCursor
	if err := shared.DecodeCursor(s, &got); err != nil || !got.At.Equal(at) || got.At.Location() != time.UTC || got.ID != c.ID {
		t.Errorf("DecodeCursor(EncodeCursor(%+v)) = %+v, %v; want it back in UTC", c, got, err)
	}
	spelled := func(at time.Time) string {
		return base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"p":{"at":"` + at.Format(time.RFC3339Nano) + `","id":"` +
			c.ID.String() + `"}}`))
	}
	if err := shared.DecodeCursor(spelled(at), &got); err == nil {
		t.Error("DecodeCursor(the time at an offset) = nil, want the cursor refused")
	}
	if from := domain.ChangesetFrom(); from.ID != uuid.Max() || from.At.Year() != 9999 {
		t.Errorf("ChangesetFrom() = %+v, want above every changeset", from)
	}
}

// The client filter is a client the changesets table takes, or mcp for
// every MCP client; anything else is 422 on client, bytes no UTF-8 too.
func TestParseClientFilter(t *testing.T) {
	for _, tt := range []struct {
		in     string
		client domain.Client
		all    bool
	}{
		{"web", domain.ClientWeb, false},
		{"api", domain.ClientAPI, false},
		{"cli", domain.ClientCLI, false},
		{"mcp", "", true},
		{"mcp:claude-code", "mcp:claude-code", false},
		{"mcp:" + strings.Repeat("名", 128), domain.Client("mcp:" + strings.Repeat("名", 128)), false},
		// Format characters other than the bidirectional controls are a
		// name's: a zero-width space, a soft hyphen, a byte order mark.
		{"mcp:a\u200bb", "mcp:a\u200bb", false},
		{"mcp:a\u00adb", "mcp:a\u00adb", false},
		{"mcp:\ufeffa", "mcp:\ufeffa", false},
	} {
		client, all, err := domain.ParseClientFilter(tt.in)
		if err != nil || client != tt.client || all != tt.all {
			t.Errorf("ParseClientFilter(%q) = %q %v %v; want %q %v", tt.in, client, all, err, tt.client, tt.all)
		}
	}
	for _, in := range []string{"", "WEB", "mobile", "mcp:", "mcp:a\nb", "mcp:" + strings.Repeat("名", 129), "mcp:\xff", "\xff",
		"mcp:a\x00", "mcp:a\u2028b", "mcp:\u202ea", "mcp:\u061c", "mcp:a\u200e"} {
		_, _, err := domain.ParseClientFilter(in)
		var e *shared.Error
		if !errors.As(err, &e) || e.Code != shared.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Field != "client" {
			t.Errorf("ParseClientFilter(%q) = %v, want 422 on client", in, err)
		}
		// Bytes that are no UTF-8 are told so first, whatever they say.
		if !utf8.ValidString(in) && e.Fields[0].Message != "is not UTF-8" {
			t.Errorf("ParseClientFilter(%q) tells %q, want it is not UTF-8", in, e.Fields[0].Message)
		}
	}
}

// A message is 1–4,096 bytes of UTF-8 without NUL.
func TestCheckMessage(t *testing.T) {
	for _, ok := range []string{"a", strings.Repeat("a", 4096), strings.Repeat("名", 1365), "line\nline"} {
		if err := domain.CheckMessage(ok); err != nil {
			t.Errorf("CheckMessage(%d bytes) = %v, want nil", len(ok), err)
		}
	}
	for in, code := range map[string]string{
		"": shared.FieldTooShort, strings.Repeat("a", 4097): shared.FieldTooLong, "a\xff": shared.FieldInvalidFormat,
		"a\x00": shared.FieldInvalidFormat,
	} {
		var e *shared.Error
		if err := domain.CheckMessage(in); !errors.As(err, &e) || len(e.Fields) != 1 || e.Fields[0].Field != "message" ||
			e.Fields[0].Code != code {
			t.Errorf("CheckMessage(%d bytes) = %v, want %s on message", len(in), err, code)
		}
	}
}

// A change is what its item tells, content without one.
func TestChangeKind(t *testing.T) {
	a, b := uuid.NewV7(), uuid.NewV7()
	place := func(parent *uuid.UUID, name string, order float64) *domain.ChangePlace {
		return &domain.ChangePlace{ParentID: parent, Name: name, SortOrder: order}
	}
	for _, tt := range []struct {
		name string
		row  domain.ChangeRow
		want domain.ChangeKind
	}{
		{"content", domain.ChangeRow{Content: &domain.ChangeVersion{Revision: 2}}, domain.ChangeContent},
		{"created", domain.ChangeRow{After: place(nil, "A", 1)}, domain.ChangeCreated},
		{"restored", domain.ChangeRow{After: place(nil, "A", 1), Restored: true}, domain.ChangeRestored},
		{"deleted", domain.ChangeRow{Before: place(&a, "A", 1)}, domain.ChangeDeleted},
		{"moved", domain.ChangeRow{Before: place(&a, "A", 1), After: place(&b, "A", 1)}, domain.ChangeMoved},
		{"moved to the root and renamed", domain.ChangeRow{Before: place(&a, "A", 1), After: place(nil, "B", 1)}, domain.ChangeMoved},
		{"renamed", domain.ChangeRow{Before: place(&a, "A", 1), After: place(&a, "B", 1)}, domain.ChangeRenamed},
		{"renamed and reordered", domain.ChangeRow{Before: place(nil, "A", 1), After: place(nil, "a", 2)}, domain.ChangeRenamed},
		{"reordered", domain.ChangeRow{Before: place(&a, "A", 1), After: place(&a, "A", 2)}, domain.ChangeReordered},
	} {
		if got := tt.row.Change(); got != tt.want {
			t.Errorf("%s: Change() = %s, want %s", tt.name, got, tt.want)
		}
	}
}
