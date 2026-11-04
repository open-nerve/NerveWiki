package app

import (
	"context"
	"fmt"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// resolve resolves links, of notebookID, anew (M6/P3 design 3.4, step 7),
// and returns those that resolve otherwise, each with its new resolution.
// Under the notebook's lock, a link or an alias of a page the notebook
// does not have is a defect: an error.
func (x Index) resolve(ctx context.Context, notebookID uuid.UUID, links []Link) ([]Link, error) {
	rs, missing, err := resolutions(ctx, x.Store, x.Pages, notebookID, links, nil, nil)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("linking: the pages %v of links or aliases are not pages of %s", missing, notebookID)
	}
	var changed []Link
	for i, l := range links {
		if rs[i] != l.Resolution {
			l.Resolution = rs[i]
			changed = append(changed, l)
		}
	}
	return changed, nil
}

// checkEvery is how many links resolutions resolves between two looks at
// its context.
const checkEvery = 4096

// resolutions is where links, of notebookID, resolve (M6/P3 design 3.4,
// step 7), in links' order: it reads the candidates of all of them at
// once, the pages with their last keys, the pages with an alias of them,
// and the paths of those and of the links' pages; none for links of which
// no target is a path. A link whose page is not one of the notebook's
// resolves to none, and an alias whose page is not is none: missing has
// those pages, each once. A target that is no path resolves to none. The
// links of a page to one target, as it parses (written in any case or
// form), resolve once, so that a page that writes one link many times
// costs one resolution against the pages of its name (P3B review M1). A
// target looks only at the pages whose path ends as it does, so that a page
// of distinct paths to pages of one name costs about as many as it has
// (M6 closeout FA-I1); and a context ended stops it. known are pages whose
// paths the caller read, a page in the trash's (M8/P2 design 3.7): their
// links resolve from them, which Paths, of the pages alive, would not
// find; the candidates and the aliases are still the pages alive alone.
// gone are pages deleted whose rows the index still has: their aliases
// are none (M9/P3 design 3.7.2, a batch's deferred rewrite and its plan).
func resolutions(ctx context.Context, store Store, pages Pages, notebookID uuid.UUID, links []Link, known []domain.Node,
	gone map[uuid.UUID]bool,
) (
	[]domain.Resolution, []uuid.UUID, error,
) {
	if len(links) == 0 {
		return nil, nil, nil
	}
	targets := newParsedTargetRows(len(links))
	var keys []string
	reach := make(map[string]int)
	ids := make([]uuid.UUID, 0)
	read := make(map[uuid.UUID]bool, len(known))
	for _, n := range known {
		read[n.ID] = true
	}
	orderedSources := true
	for i, l := range links {
		sourceChanged := i == 0 || links[i-1].SourceID != l.SourceID
		if sourceChanged && i > 0 && links[i-1].SourceID.Compare(l.SourceID) > 0 {
			orderedSources = false
		}
		t, parsed := targets.add(i, l.Target)
		if parsed {
			for _, key := range t.LastKeys() {
				if _, seen := reach[key]; !seen {
					keys = append(keys, key)
					reach[key] = 0
				}
			}
		}
		if sourceChanged && !read[l.SourceID] {
			ids = append(ids, l.SourceID)
			read[l.SourceID] = true
		}
	}
	targets.cache = nil // Later passes need values and row references, not raw keys.
	slices.Sort(keys)
	out := make([]domain.Resolution, len(links))
	if len(keys) == 0 {
		return out, nil, nil
	}
	candidates, err := pages.ByKeys(ctx, notebookID, keys)
	if err != nil {
		return nil, nil, err
	}
	aliases, err := store.Aliases(ctx, notebookID, keys)
	if err != nil {
		return nil, nil, err
	}
	aliasedBy := make(map[string][]uuid.UUID)
	for _, a := range aliases {
		if gone[a.PageID] {
			continue
		}
		aliasedBy[a.Key] = append(aliasedBy[a.Key], a.PageID)
		ids = append(ids, a.PageID)
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	nodes, err := pages.Paths(ctx, notebookID, slices.Compact(ids))
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[uuid.UUID]domain.Node, len(nodes)+len(known))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	for _, n := range known {
		byID[n.ID] = n
	}
	overflow := 0
	var reachFrom domain.Node
	var reachExists bool
	for i, l := range links {
		t, parsed := targets.at(i, &overflow)
		if i == 0 || links[i-1].SourceID != l.SourceID {
			reachFrom, reachExists = byID[l.SourceID]
		}
		if reachExists && parsed {
			for _, key := range t.LastKeys() {
				reach[key] = max(reach[key], t.Reach(reachFrom.Path))
			}
		}
	}
	// Read once by the ends of their paths, as deep as the targets ending
	// with each key reach, for each target to find those that end as it
	// does (M6 closeout FA-I1, FA2-M1, FA3-N2).
	suffixes := domain.NewSuffixes(candidates, reach)
	var missing []uuid.UUID
	// Share fresh values across an ordered run only when complete source
	// folders are equal; overflow still has the original per-source cache.
	// Unordered sources keep the original whole-call source cache.
	resolved := newResolutionMemo(orderedSources)
	overflow = 0
	// The pages of each alias key, read once a key, as a target of it comes.
	aliased := make(map[string][]domain.Node)
	var from domain.Node
	var exists bool
	for i, l := range links {
		if i%checkEvery == 0 {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
		}
		if i == 0 || links[i-1].SourceID != l.SourceID {
			from, exists = byID[l.SourceID]
			if orderedSources {
				resolved.begin(from.Path, exists)
			}
		}
		t, parsed := targets.at(i, &overflow)
		if !parsed {
			continue
		}
		if !exists {
			missing = append(missing, l.SourceID)
			continue
		}
		key := t.Key()
		if r, ok := resolved.get(l.SourceID, key); ok {
			out[i] = r
			continue
		}
		for _, key := range t.LastKeys() {
			if _, ok := aliased[key]; ok {
				continue
			}
			var named []domain.Node
			for _, id := range aliasedBy[key] {
				if n, ok := byID[id]; ok {
					named = append(named, n)
				} else {
					missing = append(missing, id)
				}
			}
			aliased[key] = named
		}
		out[i] = suffixes.Resolve(t, from.Path, aliased)
		resolved.put(l.SourceID, key, out[i])
	}
	slices.SortFunc(missing, uuid.UUID.Compare)
	return out, slices.Compact(missing), nil
}
