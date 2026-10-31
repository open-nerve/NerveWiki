package domain_test

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type lintCursorWire struct {
	V int        `json:"v"`
	R string     `json:"r"`
	N uuid.UUID  `json:"n"`
	S string     `json:"s"`
	C int64      `json:"c,omitempty"`
	K string     `json:"k,omitempty"`
	H string     `json:"h,omitempty"`
	I *uuid.UUID `json:"i,omitempty"`
	T string     `json:"t,omitempty"`
	B bool       `json:"b,omitempty"`
}

func TestLintCursor(t *testing.T) {
	zero := uuid.UUID{}
	one := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	binding := domain.CursorBinding{NotebookID: zero, Mode: domain.ModeStandard}
	hash := strings.Repeat("a", 64)
	encode := func(p lintCursorWire) string {
		t.Helper()
		s, err := shared.EncodeCursor(p)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	wanted := lintCursorWire{V: 1, R: "wanted", N: zero, S: "I", C: 1, K: "YQ", H: hash}
	t.Run("inner-key-canonical", func(t *testing.T) {
		for _, key := range []string{"YR", "YQ=", "YQ\n", base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 257)), ""} {
			p := wanted
			p.K = key
			if _, err := domain.DecodeLintCursor(encode(p)); err == nil {
				t.Fatalf("noncanonical/overlong inner key accepted: bytes%d %q", len(key), key[:min(len(key), 12)])
			}
		}
		c, err := domain.DecodeLintCursor(encode(wanted))
		if err != nil {
			t.Fatal(err)
		}
		p, err := c.PositionFor(binding, domain.Wanted)
		if err != nil || string(p.Prefix) != "a" || p.Pages != 1 {
			t.Fatalf("canonical YQ = %+v,%v", p, err)
		}
	})
	t.Run("zero-UUID-and-start", func(t *testing.T) {
		for _, rule := range domain.ApplicableRules(domain.ModeLLM) {
			s, err := domain.EncodeLintCursor(binding, rule, domain.Position{Begin: true})
			if err != nil {
				t.Fatal(err)
			}
			var wire lintCursorWire
			if err = shared.DecodeCursor(s, &wire); err != nil || !wire.B || wire.I != nil || wire.C != 0 || wire.K != "" || wire.H != "" || wire.T != "" {
				t.Fatalf("start cursor fields = %+v,%v", wire, err)
			}
			c, err := domain.DecodeLintCursor(s)
			if err != nil {
				t.Fatal(err)
			}
			p, err := c.PositionFor(binding, rule)
			if err != nil || !p.Begin || p.ID != nil {
				t.Fatalf("start = %+v %v", p, err)
			}
		}
		s, err := domain.EncodeLintCursor(binding, domain.Orphans, domain.Position{ID: &zero})
		if err != nil {
			t.Fatal(err)
		}
		c, err := domain.DecodeLintCursor(s)
		if err != nil {
			t.Fatal(err)
		}
		p, err := c.PositionFor(binding, domain.Orphans)
		if err != nil || p.Begin || p.ID == nil || *p.ID != zero {
			t.Fatalf("zero actual position = %+v %v", p, err)
		}
		if _, err = domain.EncodeLintCursor(binding, domain.Orphans, domain.Position{Begin: true, ID: &zero}); err == nil {
			t.Fatal("start plus actual UUID accepted")
		}
	})
	t.Run("rule-field-whitelists", func(t *testing.T) {
		for _, p := range []lintCursorWire{
			{V: 2, R: "orphans", S: "I", I: &zero}, {V: 1, R: "bad", S: "I", I: &zero},
			{V: 1, R: "wanted", S: "I", C: 0, K: "YQ", H: hash}, {V: 1, R: "wanted", S: "I", C: -1, K: "YQ", H: hash},
			{V: 1, R: "wanted", S: "I", C: 1, K: "YQ", H: strings.Repeat("A", 64)}, {V: 1, R: "wanted", S: "I", C: 1, K: "YQ", H: hash, I: &zero},
			{V: 1, R: "orphans", S: "I"}, {V: 1, R: "orphans", S: "I", I: &zero, H: hash},
			{V: 1, R: "ambiguous", S: "I", I: &zero}, {V: 1, R: "invalid_sources", S: "I", I: &zero, H: hash, C: 1},
			{V: 1, R: "undigested_sources", S: "I", I: &zero}, {V: 1, R: "orphans", S: "I", B: true, C: 1},
		} {
			if _, err := domain.DecodeLintCursor(encode(p)); err == nil {
				t.Fatalf("invalid rule fields accepted: %+v", p)
			}
		}
	})
	t.Run("UTC-and-time-roundtrip", func(t *testing.T) {
		for _, stamp := range []string{"2026-10-30T00:00:00+00:00", "2026-10-30T08:00:00+08:00", "2026-10-30T00:00:00.000Z", "bad", "2026-10-30t00:00:00Z"} {
			p := lintCursorWire{V: 1, R: "undigested_sources", S: "I", I: &zero, T: stamp}
			if _, err := domain.DecodeLintCursor(encode(p)); err == nil {
				t.Fatalf("noncanonical UTC accepted: %s", stamp)
			}
		}
		p := lintCursorWire{V: 1, R: "undigested_sources", S: "I", I: &zero, T: "0001-01-01T00:00:00Z"}
		c, err := domain.DecodeLintCursor(encode(p))
		if err != nil {
			t.Fatal(err)
		}
		pos, err := c.PositionFor(binding, domain.UndigestedSources)
		if err != nil || !pos.CreatedAt.IsZero() {
			t.Fatalf("valid earliest time = %+v %v", pos, err)
		}
		s, err := domain.EncodeLintCursor(binding, domain.UndigestedSources, domain.Position{ID: &zero, CreatedAt: time.Unix(1, 123).In(time.FixedZone("east", 28800))})
		if err != nil {
			t.Fatal(err)
		}
		var wire lintCursorWire
		if err = shared.DecodeCursor(s, &wire); err != nil || wire.T != "1970-01-01T00:00:01.000000123Z" {
			t.Fatalf("encoded UTC=%+v %v", wire, err)
		}
	})
	t.Run("scope-binding-and-owned-position", func(t *testing.T) {
		c, err := domain.DecodeLintCursor(encode(wanted))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = c.PositionFor(domain.CursorBinding{NotebookID: zero, Mode: domain.ModeLLM, AreaRootID: &one}, domain.Wanted); err != nil {
			t.Fatal("implicit cursor bound old mode/root")
		}
		for _, b := range []domain.CursorBinding{{NotebookID: one}, {NotebookID: zero, Explicit: true, Mode: domain.ModeLLM, AreaRootID: &one}} {
			if _, err = c.PositionFor(b, domain.Wanted); err == nil {
				t.Fatal("foreign scope accepted")
			}
		}
		if _, err = c.PositionFor(binding, domain.Orphans); err == nil {
			t.Fatal("foreign rule accepted")
		}
		p, _ := c.PositionFor(binding, domain.Wanted)
		p.Prefix[0] = 'z'
		again, _ := c.PositionFor(binding, domain.Wanted)
		if string(again.Prefix) != "a" {
			t.Fatal("decoded cursor exposes mutable bytes")
		}
		explicit := domain.CursorBinding{NotebookID: zero, Explicit: true, Mode: domain.ModeLLM, AreaRootID: &zero}
		s, err := domain.EncodeLintCursor(explicit, domain.Orphans, domain.Position{ID: &zero})
		if err != nil {
			t.Fatal(err)
		}
		c, err = domain.DecodeLintCursor(s)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = c.PositionFor(explicit, domain.Orphans); err != nil {
			t.Fatal(err)
		}
		explicit.AreaRootID = &one
		if _, err = c.PositionFor(explicit, domain.Orphans); err == nil {
			t.Fatal("explicit root not bound")
		}
		for _, scope := range []string{"E:llm_wiki:", "E:bad:", "E:standard:bad", "I:standard", "E:LLM_WIKI:00000000-0000-0000-0000-000000000000"} {
			p := wanted
			p.S = scope
			if _, err := domain.DecodeLintCursor(encode(p)); err == nil {
				t.Fatalf("bad scope accepted %s", scope)
			}
		}
	})
	t.Run("outer-unique-and-length", func(t *testing.T) {
		valid := encode(wanted)
		raw, _ := base64.RawURLEncoding.DecodeString(valid)
		for _, bad := range []string{valid + "=", valid + "\n", base64.RawURLEncoding.EncodeToString(append([]byte(" "), raw...)), base64.RawURLEncoding.EncodeToString(bytes.Replace(raw, []byte(`"v":1,"p"`), []byte(`"v":1,"v":1,"p"`), 1)), strings.Repeat("a", 2049)} {
			if _, err := domain.DecodeLintCursor(bad); err == nil {
				t.Fatal("noncanonical/oversized outer cursor accepted")
			}
		}
	})
}
