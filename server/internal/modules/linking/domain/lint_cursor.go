package domain

import (
	"encoding/base64"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// LintCursor is a shape-checked cursor. Binding to the authorized notebook,
// scope and requested rule is a separate step, preserving use-case error order.
type LintCursor struct {
	notebook uuid.UUID
	rule     Rule
	scope    string
	position Position
}

type lintCursorPayload struct {
	V int        `json:"v"`
	R Rule       `json:"r"`
	N uuid.UUID  `json:"n"`
	S string     `json:"s"`
	C int64      `json:"c,omitempty"`
	K string     `json:"k,omitempty"`
	H string     `json:"h,omitempty"`
	I *uuid.UUID `json:"i,omitempty"`
	T string     `json:"t,omitempty"`
	B bool       `json:"b,omitempty"`
}

func lintScope(b CursorBinding) (string, error) {
	if !b.Explicit {
		return "I", nil
	}
	mode := "standard"
	switch b.Mode {
	case ModeStandard:
	case ModeLLM:
		mode = "llm_wiki"
		if b.AreaRootID == nil {
			return "", errLintValue
		}
	default:
		return "", errLintValue
	}
	root := ""
	if b.AreaRootID != nil {
		root = b.AreaRootID.String()
	}
	return "E:" + mode + ":" + root, nil
}

func lintScopeShape(scope string) bool {
	if scope == "I" {
		return true
	}
	parts := strings.Split(scope, ":")
	if len(parts) != 3 || parts[0] != "E" || (parts[1] != "standard" && parts[1] != "llm_wiki") {
		return false
	}
	if parts[2] == "" {
		return parts[1] == "standard"
	}
	root, err := uuid.Parse(parts[2])
	return err == nil && root.String() == parts[2]
}

func lintCanonicalHash(hash string) bool {
	if len(hash) != 64 {
		return false
	}
	for _, c := range hash {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func (p lintCursorPayload) positionValue() (Position, error) {
	if p.V != 1 || !lintRule(p.R) || !lintScopeShape(p.S) {
		return Position{}, shared.InvalidCursor()
	}
	if p.B {
		if p.C != 0 || p.K != "" || p.H != "" || p.I != nil || p.T != "" {
			return Position{}, shared.InvalidCursor()
		}
		return Position{Begin: true}, nil
	}
	pos := Position{Pages: p.C, Hash: p.H}
	if p.R == Wanted {
		key, err := base64.RawURLEncoding.DecodeString(p.K)
		if err != nil || base64.RawURLEncoding.EncodeToString(key) != p.K || len(key) == 0 || len(key) > 256 || p.C <= 0 || !lintCanonicalHash(p.H) || p.I != nil || p.T != "" {
			return Position{}, shared.InvalidCursor()
		}
		pos.Prefix = key
		return pos, nil
	}
	if p.C != 0 || p.K != "" || p.I == nil {
		return Position{}, shared.InvalidCursor()
	}
	id := *p.I
	pos.ID = &id
	switch p.R {
	case Ambiguous, InvalidSources:
		if !lintCanonicalHash(p.H) || p.T != "" {
			return Position{}, shared.InvalidCursor()
		}
	case UndigestedSources:
		at, err := time.Parse(time.RFC3339Nano, p.T)
		if err != nil || at.UTC().Format(time.RFC3339Nano) != p.T || p.H != "" {
			return Position{}, shared.InvalidCursor()
		}
		pos.CreatedAt = at.UTC()
	default:
		if p.H != "" || p.T != "" {
			return Position{}, shared.InvalidCursor()
		}
	}
	return pos, nil
}

// DecodeLintCursor checks the bounded outer envelope, inner K and rule shape.
func DecodeLintCursor(raw string) (LintCursor, error) {
	if len(raw) > 2048 {
		return LintCursor{}, shared.InvalidCursor()
	}
	var payload lintCursorPayload
	if err := shared.DecodeCursor(raw, &payload); err != nil {
		return LintCursor{}, err
	}
	position, err := payload.positionValue()
	if err != nil {
		return LintCursor{}, err
	}
	return LintCursor{notebook: payload.N, rule: payload.R, scope: payload.S, position: position}, nil
}

// PositionFor binds a decoded cursor and returns an independently owned position.
func (c LintCursor) PositionFor(b CursorBinding, rule Rule) (Position, error) {
	scope, err := lintScope(b)
	if err != nil || !lintRule(rule) || rule != c.rule || b.NotebookID != c.notebook || scope != c.scope {
		return Position{}, shared.InvalidCursor()
	}
	p := c.position
	p.Prefix = slices.Clone(p.Prefix)
	if p.ID != nil {
		id := *p.ID
		p.ID = &id
	}
	return p, nil
}

// EncodeLintCursor encodes either an actual item position or only the shared
// fields and Begin=true. It refuses fields belonging to another rule.
func EncodeLintCursor(b CursorBinding, rule Rule, p Position) (string, error) {
	scope, err := lintScope(b)
	if err != nil {
		return "", err
	}
	payload := lintCursorPayload{V: 1, R: rule, N: b.NotebookID, S: scope, C: p.Pages, K: base64.RawURLEncoding.EncodeToString(p.Prefix), H: p.Hash, I: p.ID, B: p.Begin}
	if !p.CreatedAt.IsZero() || rule == UndigestedSources && !p.Begin {
		payload.T = p.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	if _, err := payload.positionValue(); err != nil {
		return "", errLintValue
	}
	raw, err := shared.EncodeCursor(payload)
	if err != nil || len(raw) > 2048 {
		return "", errLintValue
	}
	return raw, nil
}
