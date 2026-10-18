package app

import (
	"container/list"
	"context"
	"errors"
	"strings"
	"sync"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Instructions are a notebook's instructions for MCP (M9 design 4.6): its
// root page AGENTS's content without its frontmatter, cut at
// MaxInstructionsBytes as measure counts (mcpserver.TextBytes, which the
// module root passes); ok is false when it has none (no such node, an
// attachment holding the key, a content empty once cut), which the entry
// answers with the built-in ones. It decides nothing: the entry's Gate or
// open_notebook decided the notebook.
type Instructions struct {
	store   AgentsStore
	nodes   Nodes
	md      Markdown
	measure func(string) int
	cache   *instructionsCache
}

// NewInstructions returns the use case and its cache: the module builds
// one, which the notebook entry and open_notebook share.
func NewInstructions(store AgentsStore, nodes Nodes, md Markdown, measure func(string) int) *Instructions {
	return &Instructions{store: store, nodes: nodes, md: md, measure: measure, cache: newInstructionsCache(maxCachedInstructions)}
}

// maxCachedInstructions is how many AGENTS texts the cache holds, each a
// copy of at most MaxInstructionsBytes: 8 MiB at most.
const maxCachedInstructions = 256

// Execute reads the notebook's instructions, unlocked: the AGENTS page by
// one point lookup, with its title now; its text from the cache when the
// page's node and revision are those cached, else from its content, cached
// under the revision read with it (M9/P1 design 3.15.1). The note names
// the title now: a rename that changes only the case keeps the key.
func (i *Instructions) Execute(ctx context.Context, notebookID uuid.UUID) (a domain.Agents, ok bool, err error) {
	node, kind, title, revision, found, err := i.store.AgentsPage(ctx, notebookID, shared.TitleKey(domain.AgentsTitle))
	if err != nil || !found || kind != domain.KindPage {
		return domain.Agents{}, false, err
	}
	text, hit := i.cache.get(instructionsKey{notebook: notebookID, node: node, revision: revision})
	if !hit {
		pc, err := i.nodes.PageContent(ctx, node)
		switch {
		case errors.Is(err, ErrNotFound): // deleted since the lookup
			return domain.Agents{}, false, nil
		case err != nil:
			return domain.Agents{}, false, err
		}
		text.body, text.cut = domain.AgentsBody(pc.Content, i.md.FrontmatterEnd(pc.Content), i.measure)
		// The body is a part of the content: cached as a copy, so that an
		// entry holds its 32 KiB, not the content's 5 MiB. Written since
		// the lookup, the content read is of a later revision: it is
		// cached under its own.
		text.body = strings.Clone(text.body)
		i.cache.put(instructionsKey{notebook: notebookID, node: node, revision: pc.Revision}, text)
	}
	if text.body == "" {
		return domain.Agents{}, false, nil
	}
	return domain.Agents{Body: text.body, Cut: text.cut, Title: title}, true, nil
}

// instructionsKey is what an AGENTS text is cached by (G16): replacing the
// page, renaming one in or out and deleting it change the node or leave
// none; a restore from the trash brings back the node, its revision and
// its content; a version restored and a revert write a revision.
type instructionsKey struct {
	notebook, node uuid.UUID
	revision       int
}

// cachedText is AgentsBody's answer, without the note, which names the
// title of each read.
type cachedText struct {
	body string
	cut  bool
}

// instructionsCache holds the texts last used, at most max: the least
// recently used goes first.
type instructionsCache struct {
	mu      sync.Mutex
	max     int
	order   *list.List // of *cachedEntry, the most recent first
	entries map[instructionsKey]*list.Element
}

type cachedEntry struct {
	key  instructionsKey
	text cachedText
}

func newInstructionsCache(maxEntries int) *instructionsCache {
	return &instructionsCache{max: maxEntries, order: list.New(), entries: map[instructionsKey]*list.Element{}}
}

func (c *instructionsCache) get(k instructionsKey) (cachedText, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[k]
	if !ok {
		return cachedText{}, false
	}
	c.order.MoveToFront(e)
	return e.Value.(*cachedEntry).text, true
}

func (c *instructionsCache) put(k instructionsKey, text cachedText) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[k]; ok {
		e.Value.(*cachedEntry).text = text
		c.order.MoveToFront(e)
		return
	}
	c.entries[k] = c.order.PushFront(&cachedEntry{key: k, text: text})
	if c.order.Len() > c.max {
		last := c.order.Back()
		c.order.Remove(last)
		delete(c.entries, last.Value.(*cachedEntry).key)
	}
}
