package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"uuid"
)

// FilePath separates display text from a destination's identity. Missing
// history and export folders have no destination.
type FilePath struct {
	Label       string
	Destination *string
}

type IndexFileItem struct {
	ID                uuid.UUID
	Path              FilePath
	ParentPath, Title string
	TypeKey           string
	Type, Summary     *string
	Tags              []string
	ChangedAt         time.Time
	Inbound           *int
}

type RenderIndexInput struct {
	Items      []IndexFileItem
	Total      int
	SourcePath FilePath
	Sources    int64
}

type RenderLogChange struct {
	Path  FilePath
	Kind  string
	Asset bool
}

type RenderLogChangeset struct {
	ID                                   uuid.UUID
	Kind, Client, ByName                 string
	CreatedAt                            time.Time
	Message                              *string
	Changes                              []RenderLogChange
	NodeCount, ContentCount, IngestCount int
	IngestSources                        []FilePath
}

type RenderLogInput struct {
	Items []RenderLogChangeset
	Total int
}

// Display values are inline text, capped before Markdown escaping. A path's
// destination is never shortened, trimmed or repaired along with its label.
func fileLabel(s string, limit int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}
		return r
	}, strings.ToValidUTF8(s, "\ufffd"))
	if len(s) > limit {
		end := limit
		for end > 0 && !utf8.RuneStart(s[end]) {
			end--
		}
		s = s[:end] + "…"
	}
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`).Replace(s)
}

func fileLink(p FilePath) (string, error) {
	label := fileLabel(p.Label, 256)
	if p.Destination == nil {
		return label, nil
	}
	dest := *p.Destination
	if !utf8.ValidString(dest) || strings.ContainsFunc(dest, func(r rune) bool {
		return unicode.IsControl(r) || r == '\u2028' || r == '\u2029'
	}) {
		return "", errors.New("virtual file: invalid destination")
	}
	// Preserve literal percent and ampersand before Markdown/HTML interpret
	// them: %41 and A, or &amp; and &, must remain different paths.
	dest = strings.NewReplacer("%", "%25", "&", "%26", "<", "%3C", ">", "%3E").Replace(dest)
	return "[" + label + "](<" + dest + ">)", nil
}

// renderFile reserves the mandatory header/footer and adds complete encoded
// items only. A footer counts what was actually written, not the candidates.
func renderFile(header string, items int, maxBytes int, footer func(int) string, item func(int) (string, error)) ([]byte, error) {
	if len(header)+len(footer(0)) > maxBytes {
		return nil, errors.New("virtual file: mandatory sections exceed budget")
	}
	var out strings.Builder
	out.WriteString(header)
	written := 0
	for i := range items {
		line, err := item(i)
		if err != nil {
			return nil, err
		}
		if out.Len()+len(line)+len(footer(written+1)) > maxBytes {
			break
		}
		out.WriteString(line)
		written++
	}
	out.WriteString(footer(written))
	return []byte(out.String()), nil
}

// RenderIndex writes fixed English Markdown within maxBytes, including its
// source area and honest footer. It never calls a lint service.
func RenderIndex(in RenderIndexInput, maxBytes int) ([]byte, error) {
	source, err := fileLink(in.SourcePath)
	if err != nil {
		return nil, err
	}
	header := fmt.Sprintf("# Index\n\nSource area: %s — %d sources. Use pending_sources or lint for status.\n\n", source, in.Sources)
	footer := func(n int) string {
		return fmt.Sprintf("\nShowing %d of %d pages. Use index with type, tag or parent to filter and page through more.\n", n, in.Total)
	}
	lastGroup := ""
	return renderFile(header, len(in.Items), maxBytes, footer, func(i int) (string, error) {
		it := in.Items[i]
		link, err := fileLink(FilePath{Label: it.Title, Destination: it.Path.Destination})
		if err != nil {
			return "", err
		}
		group, heading := "parent:"+it.ParentPath, it.ParentPath
		if it.Type != nil {
			group, heading = "type:"+it.TypeKey, "Type: "+fileLabel(*it.Type, 256)
		} else {
			if heading == "" {
				heading = "/"
			}
			heading = "Parent: " + fileLabel(heading, 256)
		}
		line := ""
		if group != lastGroup {
			line = "## " + heading + "\n\n"
			lastGroup = group
		}
		line += "- " + link
		if it.Summary != nil {
			line += " — " + fileLabel(*it.Summary, 256)
		}
		if len(it.Tags) > 0 {
			tags := make([]string, min(20, len(it.Tags)))
			for i, tag := range it.Tags[:len(tags)] {
				tags[i] = fileLabel(tag, 256)
			}
			line += "; tags: " + strings.Join(tags, ", ")
			if len(it.Tags) > len(tags) {
				line += ", …"
			}
		}
		line += "; changed: " + it.ChangedAt.UTC().Format(time.RFC3339)
		if it.Inbound != nil {
			line += fmt.Sprintf("; inbound: %d", *it.Inbound)
		}
		return line + "\n", nil
	})
}

// RenderLog writes complete changesets, retaining the positions of missing
// changes and ingest sources. Total is the recent range read by the caller.
func RenderLog(in RenderLogInput, maxBytes int) ([]byte, error) {
	footer := func(n int) string {
		return fmt.Sprintf("\nShowing %d of the most recent %d changesets. Use log to filter and page through more.\n", n, in.Total)
	}
	return renderFile("# Log\n\n", len(in.Items), maxBytes, footer, func(i int) (string, error) {
		it := in.Items[i]
		line := fmt.Sprintf("- %s %s — %s; client: %s; by: %s; nodes: %d; contents: %d; ingests: %d",
			it.CreatedAt.UTC().Format(time.RFC3339), it.ID, fileLabel(it.Kind, 256), fileLabel(it.Client, 256),
			fileLabel(it.ByName, 256), it.NodeCount, it.ContentCount, it.IngestCount)
		if it.Message != nil {
			line += "; message: " + fileLabel(*it.Message, 500)
		}
		line += "\n"
		for _, c := range it.Changes {
			link, err := fileLink(c.Path)
			if err != nil {
				return "", err
			}
			kind := "page"
			if c.Asset {
				kind = "attachment"
			}
			line += "  - " + fileLabel(c.Kind, 256) + " " + kind + ": " + link + "\n"
		}
		for _, p := range it.IngestSources {
			link, err := fileLink(p)
			if err != nil {
				return "", err
			}
			line += "  - source: " + link + "\n"
		}
		return line, nil
	})
}
