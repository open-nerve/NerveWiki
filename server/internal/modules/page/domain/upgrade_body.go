package domain

import (
	"crypto/sha256"
	"strings"
)

// UpgradeBody is a page's body as an upgrade's guardrails compare it (M11
// design 4.6, decision 189): content past its frontmatter, which ends at
// end (Markdown.FrontmatterEnd: 0 without one, an invalid block with its
// closing line being one), and past a byte order mark there.
func UpgradeBody(content string, end int) string { return strings.TrimPrefix(content[end:], bom) }

// UpgradeBodyHash is UpgradeBody's SHA-256.
func UpgradeBodyHash(content string, end int) []byte {
	sum := sha256.Sum256([]byte(UpgradeBody(content, end)))
	return sum[:]
}
