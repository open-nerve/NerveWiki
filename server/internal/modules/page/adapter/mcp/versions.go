package mcpadapter

import (
	"context"
	"errors"
	"math"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// versionsIn is versions' arguments (M9/P2 design 3.16.1): with revision,
// a version's read, offset, column and limit its window; without, the
// list, limit and cursor its page.
type versionsIn struct {
	Path     string  `json:"path" jsonschema:"The page's path from the notebook's root, as \"A/B.md\"."`
	Revision *int    `json:"revision,omitempty" jsonschema:"The revision to read, from 1. Without it, the list of the page's versions."`
	Offset   *int    `json:"offset,omitempty" jsonschema:"With revision: the first line, from 1. Default 1."`
	Column   *int    `json:"column,omitempty" jsonschema:"With revision: the byte of the first line to start from, from 0: next_column of the previous answer. Default 0."`
	Limit    *int    `json:"limit,omitempty" jsonschema:"Without revision: at most this many versions, 1 to 100, default 50. With revision: at most this many lines, from 1, default 2000."`
	Cursor   *string `json:"cursor,omitempty" jsonschema:"Without revision: next_cursor of the previous answer, to go on from there."`
}

// versionsAnswer is versions' list (M9/P2 design 3.16.1): the page, its
// version now and a page of its versions, newest first; next_cursor null
// when nothing follows.
type versionsAnswer struct {
	Path           string       `json:"path"`
	NodeID         string       `json:"node_id"`
	CurrentVersion string       `json:"current_version"`
	Versions       []versionRow `json:"versions"`
	NextCursor     *string      `json:"next_cursor"`
}

// versionMeta is a version's first block, as read's pageMeta (M9/P2
// design 3.16.1): the version's own revision, maybe above the one asked
// for, and the page's version now, which write and edit take.
type versionMeta struct {
	Path           string `json:"path"`
	Kind           string `json:"kind"`
	Revision       int    `json:"revision"`
	CurrentVersion string `json:"current_version"`
	Offset         int    `json:"offset"`
	Column         int    `json:"column"`
	Lines          int    `json:"lines"`
	TotalLines     int    `json:"total_lines"`
	Bytes          int    `json:"byte_size"`
	NextOffset     *int   `json:"next_offset"`
	NextColumn     *int   `json:"next_column"`
}

// errNoVersionFits is a first version over the answer's bound, which no
// path's bound lets happen (M9/P2 design 3.16.4).
var errNoVersionFits = errors.New("page: versions' first version does not fit in an answer")

// widestVersion is the widest first block a version of the page at path
// may have, as widest is read's.
func widestVersion(path string) versionMeta {
	most := math.MinInt64
	return versionMeta{Path: path, Kind: "page", Revision: most,
		CurrentVersion: domain.VersionToken{Node: uuid.UUID{}, Revision: most}.String(), Offset: most, Column: most, Lines: most,
		TotalLines: most, Bytes: most, NextOffset: &most, NextColumn: &most}
}

// versions lists the versions of the page at path, or with revision reads
// one: the arguments of the other call are refused first, before the
// cursor is read (M9/P2 design 3.16.1).
func (t tools) versions(ctx context.Context, c mcpserver.Call, in versionsIn) (mcpserver.Result, error) {
	if err := domain.CheckVersionsArgs(in.Revision, in.Offset, in.Column, in.Cursor); err != nil {
		return mcpserver.Result{}, err
	}
	if in.Revision != nil {
		return t.version(ctx, c, in)
	}
	l, err := t.uc.Versions.AtPath(ctx, c.Notebook, in.Path, in.Limit, in.Cursor)
	if err != nil {
		return mcpserver.Result{}, toolErr(err, c, in.Path)
	}
	rows := make([]versionRow, len(l.Versions))
	for i, v := range l.Versions {
		rows[i] = versionOf(v)
	}
	n := len(rows)
	res, k, err := mcpserver.Fit(n, func(k int) any {
		return versionsAnswer{Path: l.Path, NodeID: l.Node.ID.String(), CurrentVersion: l.Current.String(), Versions: rows[:k],
			NextCursor: nextCursor(k, n, l.Cursors, l.NextCursor)}
	})
	switch {
	case err != nil:
		return mcpserver.Result{}, err
	case k == 0 && n > 0:
		return mcpserver.Result{}, errNoVersionFits
	}
	return res, nil
}

// version reads the version of the page at path holding revision, as read
// reads a page: its first block, then its lines.
func (t tools) version(ctx context.Context, c mcpserver.Call, in versionsIn) (mcpserver.Result, error) {
	v, err := t.uc.Version.AtPath(ctx, c.Notebook, in.Path, *in.Revision, in.Offset, in.Column, in.Limit,
		measureOf(func(path string) any { return widestVersion(path) }))
	if err != nil {
		return mcpserver.Result{}, toolErr(err, c, in.Path)
	}
	meta := versionMeta{Path: v.Path, Kind: "page", Revision: v.Revision, CurrentVersion: v.Current.String(),
		Offset: v.Window.Offset, Column: v.Window.Column, Lines: v.Slice.Lines, TotalLines: v.Slice.TotalLines, Bytes: v.Bytes}
	if v.Slice.NextOffset > 0 {
		meta.NextOffset = &v.Slice.NextOffset
	}
	if v.Slice.NextColumn > 0 {
		meta.NextColumn = &v.Slice.NextColumn
	}
	first, err := line(meta)
	if err != nil {
		return mcpserver.Result{}, err
	}
	return mcpserver.Result{Blocks: []mcpserver.Block{{Text: first}, {Text: v.Slice.Text}}}, nil
}
