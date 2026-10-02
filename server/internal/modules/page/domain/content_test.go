package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestCheckContent(t *testing.T) {
	for _, tt := range []struct {
		name, content, code string
	}{
		{"empty", "", ""},
		{"CRLF, a byte order mark, NFD and spaces", "\ufeff# Cafe\u0301 \r\n\r\t\u00a0\u2028", ""},
		{"exactly 5 MB", strings.Repeat("a", domain.MaxContentBytes), ""},
		{"5 MB of three-byte characters", strings.Repeat("名", domain.MaxContentBytes/3), ""},
		{"one byte more", strings.Repeat("a", domain.MaxContentBytes+1), shared.FieldTooLong},
		{"a NUL", "a\x00b", shared.FieldInvalidFormat},
		{"a lone continuation byte", "a\x80b", shared.FieldInvalidFormat},
		{"a surrogate encoded in UTF-8", "a\xed\xa0\x80b", shared.FieldInvalidFormat},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.CheckContent("content", tt.content)
			if tt.code == "" {
				if err != nil {
					t.Errorf("CheckContent() = %v, want nil", err)
				}
				return
			}
			var e *shared.Error
			if !errors.As(err, &e) || e.Code != shared.CodeValidationFailed || len(e.Fields) != 1 ||
				e.Fields[0].Field != "content" || e.Fields[0].Code != tt.code {
				t.Errorf("CheckContent() = %v, want 422 on content, %s", err, tt.code)
			}
		})
	}
}
