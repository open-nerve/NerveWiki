package mcpadapter

import (
	"context"
	"encoding/hex"
	"errors"
	"math"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// readIn is read's arguments (M9/P1 design 3.11.3).
type readIn struct {
	Path   string `json:"path" jsonschema:"The path from the notebook's root: \"A/B.md\" for a page, \"A/diagram.png\" for an attachment."`
	Offset *int   `json:"offset,omitempty" jsonschema:"The first line, from 1. Default 1."`
	Column *int   `json:"column,omitempty" jsonschema:"The byte of the first line to start from, from 0: next_column of the previous answer. Default 0."`
	Limit  *int   `json:"limit,omitempty" jsonschema:"At most this many lines, from 1. Default 2000."`
}

// pageMeta is a page's first block (M9/P1 design 3.11.1): next_offset and
// next_column null when nothing follows, or when the next part starts a
// line.
type pageMeta struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Version    string `json:"version"`
	Offset     int    `json:"offset"`
	Column     int    `json:"column"`
	Lines      int    `json:"lines"`
	TotalLines int    `json:"total_lines"`
	Bytes      int    `json:"byte_size"`
	NextOffset *int   `json:"next_offset"`
	NextColumn *int   `json:"next_column"`
}

// attachmentMeta is an attachment's first block (M9/P1 design 3.11.2):
// the sides read decided on, null when unknown; link null without an
// extension; url and expires_at null without server.public_url, a note
// then saying so when no image follows either.
type attachmentMeta struct {
	Path    string     `json:"path"`
	Kind    string     `json:"kind"`
	MIME    string     `json:"mime"`
	Bytes   int64      `json:"byte_size"`
	SHA256  string     `json:"sha256"`
	Width   *int       `json:"width"`
	Height  *int       `json:"height"`
	Link    *string    `json:"link"`
	URL     *string    `json:"url"`
	Expires *time.Time `json:"expires_at"`
	Note    string     `json:"note,omitempty"`
}

// The notes of an attachment read gives no second block of.
const (
	noPublicURL = "server.public_url is not set, so there is no address to download it; ask the server's admin."
	noFile      = "The attachment's file is missing from the server's storage."
)

// widest is the widest first block a page at path may have: each integer
// at twenty characters, next_column set (M9/P1 design 3.11.1). The real
// one is never wider, so a slice within the room it leaves keeps the
// answer within its bound.
func widest(path string) pageMeta {
	most := math.MinInt64
	version := domain.VersionToken{Node: uuid.UUID{}, Revision: most}.String()
	return pageMeta{Path: path, Kind: "page", Version: version, Offset: most, Column: most, Lines: most,
		TotalLines: most, Bytes: most, NextOffset: &most, NextColumn: &most}
}

// measure is what the answer leaves a page's slice: mcpserver counts
// the text, the room is the bound less the widest first block.
func measure() app.TextMeasure {
	return measureOf(func(path string) any { return widest(path) })
}

// measureOf is what the answer leaves a slice whose first block, at path,
// is never wider than first(path): read's page's, versions' version's.
func measureOf(first func(path string) any) app.TextMeasure {
	return app.TextMeasure{Bytes: mcpserver.TextBytes, Room: func(path string) int {
		meta, err := line(first(path))
		if err != nil {
			return 0 // Cut answers ErrNoRoom: the tool fails
		}
		return mcpserver.MaxAnswerBytes - mcpserver.TextBytes(meta)
	}}
}

// read reads the page or the attachment at the path.
func (t tools) read(ctx context.Context, c mcpserver.Call, in readIn) (mcpserver.Result, error) {
	r, err := t.uc.Read.Execute(ctx, c.Notebook, in.Path, in.Offset, in.Column, in.Limit, measure())
	if err != nil {
		return mcpserver.Result{}, toolErr(err, c, in.Path)
	}
	if r.Page == nil {
		return t.readAttachment(ctx, c, in.Path, r)
	}
	p := r.Page
	meta := pageMeta{Path: r.Path, Kind: "page", Version: p.Version.String(), Offset: p.Window.Offset,
		Column: p.Window.Column, Lines: p.Slice.Lines, TotalLines: p.Slice.TotalLines, Bytes: p.Bytes}
	if p.Slice.NextOffset > 0 {
		meta.NextOffset = &p.Slice.NextOffset
	}
	if p.Slice.NextColumn > 0 {
		meta.NextColumn = &p.Slice.NextColumn
	}
	first, err := line(meta)
	if err != nil {
		return mcpserver.Result{}, err
	}
	return mcpserver.Result{Blocks: []mcpserver.Block{{Text: first}, {Text: p.Slice.Text}}}, nil
}

// readAttachment is the attachment r found at path, as written: its
// metadata, then the image itself or a link to download it (M9/P1 design
// 3.11.2). An image needs no address: without server.public_url it is
// given all the same, and what would be a link is a note. An attachment
// gone since, or its access, before its metadata is read or its bytes, is
// the path not found.
func (t tools) readAttachment(ctx context.Context, c mcpserver.Call, path string, r app.Read) (mcpserver.Result, error) {
	a, err := t.attachments.Read(ctx, r.Node.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return mcpserver.Result{}, toolErr(&domain.PathNotFound{Path: path}, c, path)
	case err != nil:
		return mcpserver.Result{}, err
	}
	meta := attachmentMeta{Path: r.Path, Kind: "attachment", MIME: a.MIME, Bytes: a.Bytes, SHA256: hex.EncodeToString(a.SHA256)}
	if a.Link != "" {
		meta.Link = &a.Link
	}
	width, height := a.Width, a.Height
	var second *mcpserver.Block
	if mayInline(a) {
		data, w, h, err := t.attachments.Content(ctx, a, domain.MaxInlineImageBytes)
		switch {
		case errors.Is(err, app.ErrNoAttachmentFile):
			meta.Note = noFile
		case errors.Is(err, domain.ErrNotFound):
			return mcpserver.Result{}, toolErr(&domain.PathNotFound{Path: path}, c, path)
		case err != nil:
			return mcpserver.Result{}, err
		case domain.InlineImage(a.MIME, a.Bytes, w, h):
			width, height = w, h
			second = &mcpserver.Block{Image: &mcpserver.Image{Data: data, MIME: a.MIME}}
		default:
			width, height = w, h
		}
	}
	switch {
	case second != nil || meta.Note != "":
	case a.URL == "":
		meta.Note = noPublicURL
	default:
		second = linkOf(a)
	}
	if width > 0 && height > 0 {
		meta.Width, meta.Height = &width, &height
	}
	if a.URL != "" {
		expires := a.Expires.UTC()
		meta.URL, meta.Expires = &a.URL, &expires
	}
	first, err := line(meta)
	if err != nil {
		return mcpserver.Result{}, err
	}
	blocks := []mcpserver.Block{{Text: first}}
	if second != nil {
		blocks = append(blocks, *second)
	}
	return mcpserver.Result{Blocks: blocks}, nil
}

// mayInline reports whether read reads a's bytes to give it as an image:
// one of the four types, within the size, its row's sides unknown or
// within the bound (M9/P1 design 3.11.2, the inline order).
func mayInline(a app.Attachment) bool {
	return domain.InlineType(a.MIME) && a.Bytes <= domain.MaxInlineImageBytes &&
		a.Width <= domain.MaxInlineImageSide && a.Height <= domain.MaxInlineImageSide
}

// linkOf is a's resource link: its download address, name, type and size.
func linkOf(a app.Attachment) *mcpserver.Block {
	return &mcpserver.Block{Link: &mcpserver.Link{URI: a.URL, Name: a.Name, MIME: a.MIME, Size: a.Bytes}}
}
