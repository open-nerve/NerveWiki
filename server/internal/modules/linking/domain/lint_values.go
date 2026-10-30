package domain

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"
)

var errLintValue = errors.New("lint: invalid internal value")

// ApplicableRules returns a fresh list in report order, or nil for an invalid mode.
func ApplicableRules(mode LintMode) []Rule {
	rules := []Rule{Wanted, Orphans, Ambiguous, InvalidFrontmatter, MissingType, MissingSummary, UndigestedSources, InvalidSources}
	switch mode {
	case ModeStandard:
		return rules[:4:4]
	case ModeLLM:
		return rules
	default:
		return nil
	}
}

// CapCount reports a nonnegative accumulated count, without limiting paging.
func CapCount(n int64) Count { return Count{Value: min(n, 10000), Capped: n >= 10000} }

func lintPageRule(rule Rule) bool {
	return rule == Orphans || rule == InvalidFrontmatter || rule == MissingType || rule == MissingSummary
}

func lintRule(rule Rule) bool {
	return lintPageRule(rule) || rule == Wanted || rule == Ambiguous || rule == InvalidSources || rule == UndigestedSources
}

func lintHash(full string) string {
	hash := sha256.Sum256([]byte(full))
	return hex.EncodeToString(hash[:])
}

func lintDisplay(full string) (string, bool) {
	n := min(len(full), 256)
	for n < len(full) && n > 0 && !utf8.RuneStart(full[n]) {
		n--
	}
	return strings.Clone(full[:n]), n < len(full)
}

// PageItem constructs only a page-based rule, including a legal zero UUID.
func PageItem(rule Rule, id uuid.UUID) (Item, Position, error) {
	if !lintPageRule(rule) {
		return Item{}, Position{}, errLintValue
	}
	itemID, posID := id, id
	return Item{Rule: rule, ItemKey: "p:" + id.String(), PageID: &itemID}, Position{ID: &posID}, nil
}

// WantedItem retains the full canonical identity and exact distinct-page count.
func WantedItem(canonical, representativeRaw string, pages int64) (Item, Position, error) {
	if canonical == "" || representativeRaw == "" || pages <= 0 || !utf8.ValidString(canonical) || !utf8.ValidString(representativeRaw) {
		return Item{}, Position{}, errLintValue
	}
	hash := lintHash(canonical)
	target, cut := lintDisplay(representativeRaw)
	return Item{Rule: Wanted, ItemKey: "w:" + hash, Target: target, TargetCut: cut, Pages: pages},
		Position{Pages: pages, Prefix: []byte(canonical[:min(len(canonical), 256)]), Hash: hash}, nil
}

// AmbiguousItem identifies the full raw target, independently of its display.
func AmbiguousItem(page uuid.UUID, fullRaw string) (Item, Position, error) {
	if fullRaw == "" || !utf8.ValidString(fullRaw) {
		return Item{}, Position{}, errLintValue
	}
	hash := lintHash(fullRaw)
	target, cut := lintDisplay(fullRaw)
	itemID, posID := page, page
	return Item{Rule: Ambiguous, ItemKey: "a:" + page.String() + ":" + hash, PageID: &itemID, Target: target, TargetCut: cut}, Position{ID: &posID, Hash: hash}, nil
}

// InvalidSourceItem hashes one byte L/J, uint64 big-endian UTF-8 byte length,
// then the complete target/stable JSON. Identical displayed text may be two items.
func InvalidSourceItem(page uuid.UUID, fullText string, notLink bool) (Item, Position, error) {
	if fullText == "" || !utf8.ValidString(fullText) {
		return Item{}, Position{}, errLintValue
	}
	var header [9]byte
	header[0] = 'L'
	if notLink {
		header[0] = 'J'
	}
	binary.BigEndian.PutUint64(header[1:], uint64(len(fullText)))
	digest := sha256.New()
	_, _ = digest.Write(header[:])
	_, _ = digest.Write([]byte(fullText))
	hash := hex.EncodeToString(digest.Sum(nil))
	target, cut := lintDisplay(fullText)
	itemID, posID := page, page
	return Item{Rule: InvalidSources, ItemKey: "s:" + page.String() + ":" + hash, PageID: &itemID, Target: target, TargetCut: cut, NotLink: notLink}, Position{ID: &posID, Hash: hash}, nil
}

// PendingItem retains actual bytes and creation time, normalized to UTC.
func PendingItem(id uuid.UUID, kind NodeKind, bytes int64, at time.Time) (Item, Position, error) {
	if (kind != NodePage && kind != NodeAsset) || bytes < 0 {
		return Item{}, Position{}, errLintValue
	}
	at = at.Round(0).UTC()
	if _, err := time.Parse(time.RFC3339Nano, at.Format(time.RFC3339Nano)); err != nil {
		return Item{}, Position{}, errLintValue
	}
	itemID, posID := id, id
	return Item{Rule: UndigestedSources, ItemKey: "n:" + id.String(), NodeID: &itemID, Kind: kind, Bytes: bytes, CreatedAt: at}, Position{ID: &posID, CreatedAt: at}, nil
}

// ComparePosition compares valid positions of one rule; Begin precedes every
// actual item. Wanted prefixes compare as raw bytes, never as base64 text.
func ComparePosition(rule Rule, a, b Position) int {
	if a.Begin || b.Begin {
		if a.Begin == b.Begin {
			return 0
		}
		if a.Begin {
			return -1
		}
		return 1
	}
	if rule == Wanted {
		if c := cmp.Compare(b.Pages, a.Pages); c != 0 {
			return c
		}
		if c := bytes.Compare(a.Prefix, b.Prefix); c != 0 {
			return c
		}
		return strings.Compare(a.Hash, b.Hash)
	}
	if rule == UndigestedSources {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
	}
	if c := bytes.Compare(a.ID[:], b.ID[:]); c != 0 {
		return c
	}
	if rule == Ambiguous || rule == InvalidSources {
		return strings.Compare(a.Hash, b.Hash)
	}
	return 0
}
