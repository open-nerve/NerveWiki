package domain

import (
	"context"
	"errors"
	"strings"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Presence is a notebook's complete page/Linkable-asset title-key presence.
// Emitter exclusions, current format and source membership do not narrow it.
type Presence interface {
	Has(key string, asset bool) bool
}

type lintKindPresence struct{ Presence }

func (p lintKindPresence) has(key string, asset bool) bool {
	return p.Presence != nil && p.Has(key, asset)
}

// ReadsAsAssetWith uses the same target form as resolution, without needing
// candidate paths. Presence must already exclude non-Linkable assets.
func (t Target) ReadsAsAssetWith(p Presence) bool {
	_, asset := t.form(lintKindPresence{p})
	return asset
}

// WantedTarget qualifies an unresolved indexed link and gives its complete
// canonical key. parentKeys is the emitter's parent folder, never its own
// title. Lint refuses an above-root relative target; Resolve still clamps it.
func WantedTarget(ctx context.Context, raw, kind string, parentKeys []string, llm bool, p Presence) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	if kind != "wikilink" && kind != "link" && kind != "embed" && kind != "image" {
		return "", false, errors.New("lint: invalid link kind")
	}
	t, valid, err := lintParseTarget(ctx, raw)
	if err != nil || !valid {
		return "", false, err
	}
	if t.Relative && t.Up > len(parentKeys) {
		return "", false, nil
	}
	asset := t.ReadsAsAssetWith(p)
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	if (kind == "image" || kind == "embed") && hasExtension(t.Name) || asset {
		return "", false, nil
	}
	keys := t.Keys
	if t.Relative {
		keys = make([]string, 0, len(parentKeys)-t.Up+len(t.Keys))
		keys = append(keys, parentKeys[:len(parentKeys)-t.Up]...)
		keys = append(keys, t.Keys...)
	}
	if llm && len(keys) == 1 && (keys[0] == "index" || keys[0] == "log" || keys[0] == "index.md" || keys[0] == "log.md") {
		return "", false, nil
	}
	var joined strings.Builder
	for i, key := range keys {
		if err := ctx.Err(); err != nil {
			return "", false, err
		}
		if i != 0 {
			joined.WriteByte('/')
		}
		for len(key) > 0 {
			if err := ctx.Err(); err != nil {
				return "", false, err
			}
			n := min(len(key), 4096)
			joined.WriteString(key[:n])
			key = key[n:]
		}
	}
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	return joined.String(), true, nil
}

// The same ParseTarget grammar, with cancellable scans and the shared
// context-aware TitleKey pipeline. The old parsing/write path is unchanged.
func lintParseTarget(ctx context.Context, raw string) (Target, bool, error) {
	var t Target
	rest := raw
	switch {
	case strings.HasPrefix(rest, "/"):
		t.Rooted, rest = true, rest[1:]
	case strings.HasPrefix(rest, "./"):
		t.Relative, rest = true, rest[2:]
	}
	if !t.Rooted {
		for strings.HasPrefix(rest, "../") {
			if t.Up%64 == 0 {
				if err := ctx.Err(); err != nil {
					return Target{}, false, err
				}
			}
			t.Relative, t.Up, rest = true, t.Up+1, rest[3:]
		}
	}
	var segments []string
	start := 0
	for i := range len(rest) {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return Target{}, false, err
			}
		}
		if rest[i] == '/' {
			segments = append(segments, rest[start:i])
			start = i + 1
		}
	}
	segments = append(segments, rest[start:])
	for i, segment := range segments {
		if i%64 == 0 {
			if err := ctx.Err(); err != nil {
				return Target{}, false, err
			}
		}
		if segment == "" || segment == "." || segment == ".." {
			return Target{}, false, nil
		}
	}
	last := segments[len(segments)-1]
	if n := len(last) - len(".md"); n > 0 && strings.EqualFold(last[n:], ".md") {
		var err error
		t.AltLast, err = shared.TitleKeyContext(ctx, last)
		if err != nil {
			return Target{}, false, err
		}
		segments[len(segments)-1] = last[:n]
	}
	t.Name = segments[len(segments)-1]
	t.Keys = make([]string, len(segments))
	for i, segment := range segments {
		key, err := shared.TitleKeyContext(ctx, segment)
		if err != nil {
			return Target{}, false, err
		}
		t.Keys[i] = key
	}
	return t, true, nil
}
