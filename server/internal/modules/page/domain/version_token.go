package domain

import (
	"strconv"
	"uuid"
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
