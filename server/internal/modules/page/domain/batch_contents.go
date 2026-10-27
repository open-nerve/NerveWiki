package domain

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Contents is the plan's contents' pass, of the ops before its failed one
// (M9 design 4.11), on the pages before the batch, read unlocked, by id:
// texts those whose first content op is an edit, or a write the renames and
// moves before it reach (ReadWhole), heads the other writes'. A
// base_version against the page's revision then: past,
// page.revision_mismatch, unless the op is a write of the bytes the page
// holds at that op (a page read by its head alone compares the head's size
// and hash, the unit's sha256), which is unchanged, as write's own is; a
// write's content (a write without base_version of the bytes the page holds
// is still the page's content op, writing nothing); an edit's Replace on
// the page's content as the ops before it leave it, unchanged when it
// leaves the bytes as they were, as edit's own is (M9/P3 review A-M3). A
// write or an edit sees
// the page's content rewritten first by rw where a rename or a move between
// its content ops reaches its links, as separate calls would leave it
// (M9/P3 review A-I1: a write's bytes too); a creation's content is
// compared with a page there by its head's size and hash. Each page's
// content at its last content op is the one the unit writes. A failure of
// an op before Failed's replaces it, so that the first op that fails
// decides; its error is a problem's, rw's too (its parses' bounds and
// budget). Any other error of rw's is Contents' own. A plan that failed for
// good, past its items or its reads, is not run.
func (p *BatchPlan) Contents(texts map[uuid.UUID]PageText, heads map[uuid.UUID]PageHead, rw LinkRewriter) error {
	if p.final {
		return nil
	}
	limit := len(p.steps)
	if p.Failed != nil {
		limit = p.Failed.Op
	}
	p.multi = len(p.order) > 1
	for j := range limit {
		s := p.steps[j]
		if s.Kind != OpKindWrite && s.Kind != OpKindEdit {
			continue
		}
		p.at = j
		err := p.content(j, texts, heads, rw)
		var problem *shared.Error
		switch {
		case err == nil:
			continue
		case !errors.As(err, &problem):
			return err
		}
		p.Failed = &OpFailed{Op: j, Tool: s.Kind, Err: err}
		return nil
	}
	return nil
}

// content is the content op j on its page.
func (p *BatchPlan) content(j int, texts map[uuid.UUID]PageText, heads map[uuid.UUID]PageHead, rw LinkRewriter) error {
	s, ps := p.steps[j], &p.Steps[j]
	pg := p.pages[ps.Node]
	if pg.first == j {
		if err := pg.load(s.Kind, texts, heads); err != nil {
			return err
		}
	}
	var beforeBytes int
	var changed bool
	switch {
	case s.Kind == OpKindWrite && len(ps.Created) > 0:
		pg.content, pg.known, changed = s.Content, true, s.Content != ""
	case s.Kind == OpKindWrite:
		same, size, err := p.holdsAt(pg, j, rw, s.Content)
		switch {
		case err != nil:
			return err
		case s.Base == nil && !same && !p.firstSourceBody(j, pg, size):
			return &PathTaken{Path: s.Path.Written}
		case s.Base != nil && s.Base.Revision != pg.revision && !same:
			return &VersionMismatch{Path: s.Path.Written, Current: VersionToken{Node: pg.id, Revision: pg.revision}, Why: Changed}
		}
		pg.content, pg.known, ps.Changed = s.Content, true, !same
		beforeBytes, changed = size, !same
	default:
		if s.Base != nil && s.Base.Revision != pg.revision {
			return &VersionMismatch{Path: s.Path.Written, Current: VersionToken{Node: pg.id, Revision: pg.revision}, Why: Changed}
		}
		content := pg.content
		if moves := p.MovesBetween(ps.Previous, j); len(moves) > 0 {
			ps.Moved = true
			rewritten, err := p.rewrite(pg, ps.Previous, j, rw)
			if err != nil {
				return err
			}
			content = rewritten
		}
		r, err := Replace(s.Args, content, s.Old, s.New, s.All)
		if err != nil {
			return err
		}
		pg.content, ps.Replacements, ps.Changed = r.Content, r.Count, r.Content != content
		beforeBytes, changed = len(content), ps.Changed
	}
	p.contentFacts(j, pg, beforeBytes, changed)
	if changed {
		// The item is a single actual change. Keep the accumulated Merged
		// in the stored facts and final body for the unit guard's fallback;
		// it must not turn this item's empty first write into a second write.
		fact := ps.Facts[len(ps.Facts)-1]
		content := *fact.Content
		content.Merged, fact.Content = 1, &content
		if v := CheckContract(p.notebook, fact); v.Err != nil {
			return Refuse(v.Err, v.NodeID)
		}
	}
	return nil
}

// firstSourceBody permits a legal no-base first body for a source page this
// batch created empty. Existing pages and ordinary no-base writes retain
// their path-taken result; no client can supply a token for a new batch page.
func (p *BatchPlan) firstSourceBody(j int, pg *plannedPage, beforeBytes int) bool {
	if p.notebook.Mode != ModeLLMWiki || !pg.created || !pg.known || beforeBytes != 0 || pg.merged != 0 {
		return false
	}
	facts := p.Steps[j].Facts
	node := facts[len(facts)-1].Nodes[0]
	return node.ID != p.notebook.SourceRootID && node.After != nil && node.After.InArea
}

// contentFacts fills each creation's body and this item's own target body.
// Only actual user changes while inside the source area count as Merged;
// rewrites before the user's operation supply its before state instead.
func (p *BatchPlan) contentFacts(j int, pg *plannedPage, beforeBytes int, changed bool) {
	facts := p.Steps[j].Facts
	last := &facts[len(facts)-1]
	if node := last.Nodes[0]; changed && p.notebook.Mode == ModeLLMWiki && node.After != nil && node.After.InArea {
		pg.merged++
	}
	for i := range facts[:len(facts)-1] {
		empty := sha256.Sum256(nil)
		facts[i].Content = &ContentFacts{Hash: empty[:]}
	}
	hash := sha256.Sum256([]byte(pg.content))
	last.Content = &ContentFacts{Hash: hash[:], Bytes: len(pg.content), BeforeBytes: beforeBytes, Merged: pg.merged}
}

// load is the page's content as the plan read it, at its first content op
// of kind: a page the batch created, empty until written; an edit's, and a
// write's read whole, its text; another write's, its head, the content
// then the write's.
func (pg *plannedPage) load(kind OpKind, texts map[uuid.UUID]PageText, heads map[uuid.UUID]PageHead) error {
	switch {
	case pg.created:
		pg.known = true
	case kind == OpKindEdit || pg.whole:
		t, ok := texts[pg.id]
		if !ok {
			return fmt.Errorf("page: the plan has no text of the page %s", pg.id)
		}
		pg.content, pg.known, pg.revision = t.Content, true, t.Revision
	default:
		h, ok := heads[pg.id]
		if !ok {
			return fmt.Errorf("page: the plan has no head of the page %s", pg.id)
		}
		pg.head, pg.revision = &h, h.Revision
	}
	return nil
}

// holdsAt reports whether the page holds content's bytes at the op j: its
// content as the ops before leave it, rewritten for the renames and moves
// since its previous content op where they reach its links, as an edit's
// is (M9/P3 review A-I1); its head's size and hash for a page the plan read
// no text of, which none of them reaches.
func (p *BatchPlan) holdsAt(pg *plannedPage, j int, rw LinkRewriter, content string) (bool, int, error) {
	ps := &p.Steps[j]
	if !pg.known {
		return pg.holds(content), pg.head.Bytes, nil
	}
	if len(p.MovesBetween(ps.Previous, j)) == 0 {
		return pg.holds(content), len(pg.content), nil
	}
	ps.Moved = true
	rewritten, err := p.rewrite(pg, ps.Previous, j, rw)
	if err != nil {
		return false, 0, err
	}
	return rewritten == content, len(rewritten), nil
}

// holds reports whether the page holds content's bytes as the batch has it
// then: its content once the plan has it, its head's size and hash before.
func (pg *plannedPage) holds(content string) bool {
	if pg.known {
		return pg.content == content
	}
	sum := sha256.Sum256([]byte(content))
	return len(content) == pg.head.Bytes && bytes.Equal(sum[:], pg.head.Hash)
}

// rewrite is the page's content with its links written again for the
// renames and moves since its content op prev, up to j: where they led by
// the index before the batch when prev is none (a page whose index is
// stale is left as it is, as M6 leaves it, and one the batch created holds
// no links yet), by its content at prev on the tree after it otherwise.
func (p *BatchPlan) rewrite(pg *plannedPage, prev, j int, rw LinkRewriter) (string, error) {
	var before LinksBefore
	switch {
	case prev < 0 && pg.created:
		return pg.content, nil
	case prev < 0:
		indexed, ok, err := rw.Indexed(pg.id, pg.revision)
		if err != nil || !ok {
			return pg.content, err
		}
		before = indexed
	default:
		resolved, err := rw.Resolve(pg.id, pg.content, prev)
		if err != nil {
			return "", err
		}
		before = resolved
	}
	return rw.Rewrite(pg.id, pg.content, before, prev, j)
}
