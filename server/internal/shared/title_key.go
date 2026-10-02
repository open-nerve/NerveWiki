package shared

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// TitleKey is the key titles compare by (v0.1 design 3.5): NFC, full
// Unicode case folding, and NFC again, as folding need not give NFC. Two
// pages under one parent may not share it, and M6's links resolve by it.
// The folding is language-independent: Turkish İ keys as i with a dot
// above, not as i; Obsidian lowercases, so Straße and STRASSE, apart in
// Obsidian, are one title here. A new Unicode version may change keys,
// which nervewiki reindex recomputes (M6).
func TitleKey(s string) string {
	// A Caser keeps state: one per call, never shared.
	return norm.NFC.String(cases.Fold().String(norm.NFC.String(s)))
}
