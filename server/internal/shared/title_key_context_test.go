package shared_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The hand-written keys guard full folding and both normalization passes;
// the repeated forms put composition across the reader's fixed chunks.
func TestLintTitleKeyContext(t *testing.T) {
	for _, tt := range []struct{ name, raw, want string }{
		{"full-fold", "Straße/İ/Σςσ", "strasse/i\u0307/σσσ"},
		{"NFC", "A\u030a/E\u0301/\u212b", "å/é/å"},
		{"chunk-combining", strings.Repeat("a", 4095) + "E\u0301", strings.Repeat("a", 4095) + "é"},
		{"chunk-hangul", strings.Repeat("a", 4093) + "\u1100\u1161\u11a8", strings.Repeat("a", 4093) + "각"},
		{"chunk-fold", strings.Repeat("a", 4095) + "Straße", strings.Repeat("a", 4095) + "strasse"},
		{"first-NFC-before-fold", strings.Repeat("a", 4096) + "\u0345\u0301", strings.Repeat("a", 4095) + "áι"},
		{"last-NFC-after-fold", strings.Repeat("a", 4095) + "\u0390", strings.Repeat("a", 4095) + "ΐ"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shared.TitleKeyContext(t.Context(), tt.raw)
			if err != nil || got != tt.want {
				t.Fatalf("context title key = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
	t.Run("whole-key-differential", func(t *testing.T) {
		parts := []string{"A\u030a", "Straße", "İ", "\u0345\u0301", "\u1100\u1161\u11a8", "Σς", "\ufb03", "\u0130\u0323", "\u1f88", "\uff21", "\u0301", "\ufffd"}
		for offset := range 32 {
			for _, a := range parts {
				for _, b := range parts {
					raw := strings.Repeat("A", 4080+offset) + strings.Repeat(a+b, 16)
					got, err := shared.TitleKeyContext(t.Context(), raw)
					if err != nil || got != shared.TitleKey(raw) {
						t.Fatalf("whole-key mismatch offset=%d a=%q b=%q err=%v", offset, a, b, err)
					}
				}
			}
		}
	})
	t.Run("cancel-before", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		got, err := shared.TitleKeyContext(ctx, "small")
		if got != "" || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled title key = %q, %v", got, err)
		}
	})
	t.Run("cancel-during-stream", func(t *testing.T) {
		base, cancel := context.WithCancel(t.Context())
		ctx := &titleCancelContext{Context: base, cancel: cancel, left: 4}
		got, err := shared.TitleKeyContext(ctx, strings.Repeat("Straße/E\u0301", 10000))
		if got != "" || !errors.Is(err, context.Canceled) || ctx.left != 0 {
			t.Fatalf("stream cancellation = length%d, %v; remaining polls%d", len(got), err, ctx.left)
		}
	})
}

// Cancel a real context at a deterministic poll, without sleeps or goroutines.
type titleCancelContext struct {
	context.Context
	cancel context.CancelFunc
	left   int
}

func (c *titleCancelContext) Err() error {
	if c.left > 0 {
		c.left--
		if c.left == 0 {
			c.cancel()
		}
	}
	return c.Context.Err()
}
