package mcpadapter

import (
	"context"
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// duplicateErr adds only a currently visible path. The original refusal remains
// in the error chain on any lookup failure; batch placement and members survive.
func (t tools) duplicateErr(ctx context.Context, c mcpserver.Call, err error) error {
	var p *shared.Error
	if !errors.As(err, &p) || p.Code != "page.source_duplicate" {
		return err
	}
	text := "page.source_duplicate: " + p.Detail
	if p.DuplicateOf == nil {
		text = "page.source_duplicate: This operation contains sources with identical bytes."
	} else if t.uc.TargetPath != nil {
		if path, lookupErr := t.uc.TargetPath.Execute(ctx, c.Notebook, *p.DuplicateOf); lookupErr == nil && path != "" {
			text = "page.source_duplicate: Identical source content already exists at " + domain.Quoted(path) + "."
		}
	}
	var te *mcpserver.ToolError
	if errors.As(err, &te) {
		out := *te
		out.Text = text
		return &out
	}
	return &mcpserver.ToolError{Err: err, Text: text}
}
