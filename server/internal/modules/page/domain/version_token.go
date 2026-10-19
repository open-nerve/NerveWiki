package domain

import (
	"strconv"
	"strings"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// VersionToken is a page's version as MCP answers it (M9 design 4.10): its
// node and its content's revision, written "<node id>:<revision>". It is
// no Version, a row of a page's versions (M8 design 4.3).
type VersionToken struct {
	Node     uuid.UUID
	Revision int
}

// String is the token: the id in its canonical form, the revision in
// decimal without a sign or leading zeros.
func (v VersionToken) String() string { return v.Node.String() + ":" + strconv.Itoa(v.Revision) }

// maxVersionTokenBytes is the longest token String writes: an id's 36
// characters, ":" and MaxRevision's ten digits.
const maxVersionTokenBytes = 36 + 1 + 10

// ParseVersionToken is the version token s as String writes it (M9 design
// 4.10): a node's id in its canonical form, ":", and a revision of 1 to
// MaxRevision in decimal, without a sign or a leading zero. Any other
// spelling is 422 on field, invalid_format, the value never repeated: an
// agent's argument may be long.
func ParseVersionToken(field, s string) (VersionToken, error) {
	if len(s) <= maxVersionTokenBytes {
		if id, rev, ok := strings.Cut(s, ":"); ok {
			node, err := uuid.Parse(id)
			r, rerr := strconv.Atoi(rev)
			if t := (VersionToken{Node: node, Revision: r}); err == nil && rerr == nil && r >= 1 && r <= MaxRevision && t.String() == s {
				return t, nil
			}
		}
	}
	return VersionToken{}, NotAVersion(field)
}

// NotAVersion is 422 on field: not a version token as read, list and write
// give them.
func NotAVersion(field string) error {
	return shared.Invalid(shared.FieldError{Field: field, Code: shared.FieldInvalidFormat,
		Message: `is not a version, "<node id>:<revision>" as read, list and write give it`})
}
