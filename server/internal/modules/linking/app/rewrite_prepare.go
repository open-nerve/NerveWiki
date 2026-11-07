package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func rewriteOf(content string, facts domain.Facts, reached map[int]rewritten, from []domain.Step, tree domain.Tree, recased domain.Recased) domain.Rewriting {
	var links []domain.Resolved
	for _, l := range facts.Links {
		if x, ok := reached[l.Start]; ok {
			links = append(links, domain.Resolved{Link: l, Before: x.before, After: x.after})
		}
	}
	return domain.Rewrite(content, from, links, tree, recased)
}

type rewriteReadBack struct {
	content    string
	now        Parsed
	left       domain.Unwritten
	kept, read bool
	large      int
}

// Both serial and reserved writes use the original Written callback. Only
// its parser's already-owned budget differs; all actual candidates and
// their fit/error precedence remain here once.
func (r Rewrite) readBack(ctx context.Context, content string, facts domain.Facts, from []domain.Step, tree domain.Tree,
	rewriting domain.Rewriting, parse func(context.Context, string) (Parsed, error), shares *keptShares,
) (out rewriteReadBack, err error) {
	var now Parsed
	defer func() {
		if !out.kept {
			now.release()
		}
	}()
	writing, left, kept, err := rewriting.Written(content, facts, from, tree, func(writing string) (domain.Facts, error) {
		now.release()
		now = Parsed{}
		if len(writing) > r.MaxContent {
			out.large = len(writing)
			return domain.Facts{}, domain.ErrTooLarge
		}
		if err := shares.fit(len(writing)); err != nil {
			return domain.Facts{}, err
		}
		var err error
		now, err = parse(ctx, writing)
		out.read = true
		return now.Facts, err
	})
	if err != nil {
		return out, err
	}
	out.content, out.left, out.kept = writing, left, kept
	if kept {
		out.now = now
	}
	return out, nil
}

// Called only by the coordinator. A worker never logs future-page facts.
func (r Rewrite) logReadBack(ctx context.Context, id uuid.UUID, originalBytes int, out rewriteReadBack) bool {
	if !out.kept {
		level, msg := slog.LevelError, "the links of a page are not rewritten: no writing reads back as its links and aliases"
		if !out.read {
			level, msg = slog.LevelWarn, "the links of a page are not rewritten: it would hold more than a page may"
		}
		attrs := []slog.Attr{slog.String("page_id", id.String())}
		if out.large > 0 {
			attrs = append(attrs, slog.Int("bytes", originalBytes), slog.Int("written", out.large))
		}
		r.Logger.LogAttrs(ctx, level, msg, attrs...)
		return false
	}
	var sizes []slog.Attr
	if out.left.TooLarge > 0 {
		sizes = []slog.Attr{slog.Int("bytes", originalBytes), slog.Int("written", out.left.TooLarge)}
	}
	for _, l := range out.left.Links {
		r.left(ctx, "no writing of the page with it was kept", id, l, sizes...)
	}
	return true
}
