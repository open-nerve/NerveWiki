package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func (f *fakeStore) SourceHeads(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]app.SourceValue, error) {
	f.record(ctx, "SourceHeads")
	if len(ids) > domain.MaxUnitNodes {
		return nil, fmt.Errorf("source heads exceeded bound: %d", len(ids))
	}
	if err := f.errs["SourceHeads"]; err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]app.SourceValue, len(ids))
	for _, id := range ids {
		n, ok := f.nodes[id]
		c := f.contents[id]
		if !ok {
			if trash, found := f.trash[id]; found {
				n, ok, c = trash.Node, true, f.trashed[id]
			}
		}
		if ok && n.NotebookID == notebook {
			out[id] = app.SourceValue{ID: id, Kind: n.Kind, Hash: slices.Clone(c.Hash), BeforeSourceHash: slices.Clone(f.sourceHashes[id]), Bytes: int64(c.ByteSize)}
		}
	}
	return out, nil
}

func (f *fakeStore) SetSourceHashes(ctx context.Context, notebook uuid.UUID, values []app.SourceValue) (map[uuid.UUID]app.SourceValue, error) {
	f.record(ctx, "SetSourceHashes")
	if len(values) > domain.MaxUnitNodes {
		return nil, fmt.Errorf("source writes exceeded bound")
	}
	if err := f.errs["SetSourceHashes"]; err != nil {
		return nil, err
	}
	old := map[uuid.UUID]app.SourceValue{}
	for _, v := range values {
		if n, ok := f.nodes[v.ID]; ok && n.NotebookID == notebook {
			if !bytes.Equal(f.sourceHashes[v.ID], v.Hash) {
				old[v.ID] = app.SourceValue{ID: v.ID, Kind: n.Kind, BeforeSourceHash: slices.Clone(f.sourceHashes[v.ID])}
			}
			f.sourceHashes[v.ID] = slices.Clone(v.Hash)
		}
	}
	return old, nil
}
func (f *fakeStore) SourceBaselines(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]app.SourceValue, error) {
	f.record(ctx, "SourceBaselines")
	if len(ids) > domain.MaxUnitNodes {
		return nil, fmt.Errorf("unbounded baselines")
	}
	out := map[uuid.UUID]app.SourceValue{}
	for _, id := range ids {
		if n, ok := f.nodes[id]; ok && n.NotebookID == notebook {
			out[id] = app.SourceValue{ID: id, Kind: n.Kind, BeforeSourceHash: slices.Clone(f.sourceHashes[id])}
		}
	}
	return out, f.errs["SourceBaselines"]
}

// A missing maintenance call, running it only at unit end, or running it
// after item recording breaks these observable writes and order checks.
func TestSourceHashTransitionsMatchRebuild(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	raw := f.page("raw", nil, 0)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
	var source domain.Node
	sum := sha256.Sum256([]byte("原文\r\n"))
	f.rec.before = map[string]func(){"RecordItems 1 in tx": func() {
		if !bytes.Equal(f.store.sourceHashes[source.ID], sum[:]) {
			t.Errorf("hash before RecordItems = %x, want current source SHA %x", f.store.sourceHashes[source.ID], sum)
		}
	}}
	_, err := f.run(f.asAlice(), domain.ClientAPI, func(ctx context.Context, u *app.Unit) error {
		// The caller-supplied ID lets the item observer read the just-written node.
		source.ID = uuid.NewV7()
		n, err := u.CreatePage(ctx, app.PageDraft{ID: source.ID, ParentID: &raw.ID, Title: "Source", Content: "原文\r\n"})
		if err != nil {
			return err
		}
		source = n
		if !bytes.Equal(f.store.sourceHashes[n.ID], sum[:]) {
			t.Errorf("same-unit source hash = %x, want %x", f.store.sourceHashes[n.ID], sum)
		}
		_, err = u.CreatePage(ctx, app.PageDraft{Title: "Outside", Content: "ordinary"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	checkFakeSourceRebuild(t, f, 1)
}

func TestSourceHashesFollowStructureAndContent(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate, domain.ActionRename, domain.ActionMove, domain.ActionDelete, domain.ActionRestore, domain.ActionWrite)
	raw, folder := f.page("raw", nil, 0), f.page("Folder", nil, 1)
	p := f.page("P", &folder.ID, 0)
	f.writeAs(p.ID, "body")
	a := f.asset("empty.bin", &p.ID, 0)
	empty := sha256.Sum256(nil)
	f.infos = map[uuid.UUID]app.AttachmentInfo{a.ID: {MIME: "application/octet-stream", SHA256: empty[:]}}
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
	g := &guard{recorder: f.rec}
	f.guards = []app.WriteGuard{g}
	if _, err := f.move(folder.ID, app.Destination{ParentID: &raw.ID}); err != nil {
		t.Fatal(err)
	}
	checkFakeSourceRebuild(t, f, 2)
	if v := g.steps[0].AreaHashes[a.ID]; v.Kind != domain.KindAsset || !bytes.Equal(v.Hash, empty[:]) {
		t.Errorf("guard's empty asset metadata = %+v", v)
	}
	f.rec.calls = nil
	if _, err := f.rename(p.ID, "Renamed"); err != nil {
		t.Fatal(err)
	}
	for _, call := range f.rec.calls {
		if strings.Contains(call, "SourceHash") || strings.Contains(call, "SourceHeads") || strings.HasPrefix(call, "AttachmentInfos") {
			t.Errorf("membership-preserving rename did source work: %s", call)
		}
	}
	checkFakeSourceRebuild(t, f, 2)
	f.rec.calls = nil
	if _, err := f.move(folder.ID, app.Destination{}); err != nil {
		t.Fatal(err)
	}
	for _, call := range f.rec.calls {
		if strings.Contains(call, "SourceHeads") || strings.HasPrefix(call, "AttachmentInfos") {
			t.Errorf("source-out read metadata: %s", call)
		}
	}
	checkFakeSourceRebuild(t, f, 0)
	if _, err := f.move(folder.ID, app.Destination{ParentID: &raw.ID}); err != nil {
		t.Fatal(err)
	}
	checkFakeSourceRebuild(t, f, 2)
	if err := f.remove(folder.ID); err != nil {
		t.Fatal(err)
	}
	checkFakeSourceRebuild(t, f, 0)
	if _, err := app.NewRestoreNode(f.trashReads(now().Add(time.Hour)), f.writer(), f.parser(), app.RestoreBounds{}, f.logger()).Execute(f.asAlice(), folder.ID, domain.ClientWeb); err != nil {
		t.Fatal(err)
	}
	checkFakeSourceRebuild(t, f, 2)
	page, err := f.create(app.PageDraft{ParentID: &raw.ID, Title: "First"})
	if err != nil {
		t.Fatal(err)
	}
	checkFakeSourceRebuild(t, f, 2)
	if _, err := f.put(page.Node.ID, app.ContentPut{Content: "first", Base: 1}); err != nil {
		t.Fatal(err)
	}
	checkFakeSourceRebuild(t, f, 3)
	// Appender writes do not participate recursively. Their hashes still change,
	// including legal convergence with an existing source and clearing to empty.
	for _, body := range []string{"body", ""} {
		f.partakers = []app.Participant{&participant{recorder: f.rec, write: &app.ContentWrite{NodeID: page.Node.ID, Content: body, Base: f.store.contents[page.Node.ID].Revision}}}
		if _, err := f.rename(raw.ID, "raw "+fmt.Sprint(len(body))); err != nil {
			t.Fatal(err)
		}
		want := 3
		if body == "" {
			want = 2
		}
		checkFakeSourceRebuild(t, f, want)
	}
}

func TestSourceHashesUseNewAssetFactsBeforeItsCallback(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(fmt.Sprint(direct), func(t *testing.T) {
			f := newFixture()
			f.grant(upload, domain.ActionCreate)
			raw := f.page("raw", nil, 0)
			f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
			sum := sha256.Sum256(nil)
			after := func(_ context.Context, n domain.Node) error {
				if !bytes.Equal(f.store.sourceHashes[n.ID], sum[:]) {
					t.Errorf("asset callback sees hash %x, want empty-file SHA %x", f.store.sourceHashes[n.ID], sum)
				}
				return nil
			}
			meta := app.AssetMeta{MIME: "application/octet-stream", SHA256: sum[:]}
			if direct {
				_, err := f.importAs(uuid.Nil(), func(ctx context.Context, u *app.ImportUnit) error {
					_, err := u.CreateAsset(ctx, app.ImportedAsset{ParentID: &raw.ID, Name: "empty.bin", Meta: meta}, after)
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
			} else {
				_, err := app.NewAssetWrites(f.writer(), f.store).CreateAsset(f.asAlice(), app.NewAsset{NotebookID: f.eng, ParentID: &raw.ID, Name: "empty.bin", Action: upload, Client: domain.ClientWeb, Meta: meta}, after)
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, call := range f.rec.calls {
				if strings.HasPrefix(call, "AttachmentInfos") {
					t.Errorf("new asset re-read its not-yet-written blob: %s", call)
				}
			}
		})
	}
}

func TestSourceHashesRejectMissingMetadataBeforeWriting(t *testing.T) {
	for _, size := range []int{0, 31, 33} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			f := newFixture()
			f.grant(upload)
			raw := f.page("raw", nil, 0)
			f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
			a := f.newAsset("bad.bin", &raw.ID)
			a.Meta.SHA256 = make([]byte, size)
			_, err := f.createAsset(a, nil)
			if err == nil {
				t.Errorf("source asset with %d-byte SHA succeeded", size)
			}
			if len(f.store.nodes) != 1 || len(f.store.changesets) != 0 {
				t.Errorf("invalid source metadata wrote nodes/changesets: %d/%d", len(f.store.nodes), len(f.store.changesets))
			}
		})
	}
}

// This oracle independently walks raw parent IDs and hashes actual body bytes.
// It does not use SourceFacts, AreaHashes, source SQL or the fake's writer.
func checkFakeSourceRebuild(t *testing.T, f *fixture, wantSources int) {
	t.Helper()
	count := 0
	for id, n := range f.store.nodes {
		var want []byte
		state := f.notebooks.states[n.NotebookID]
		inside := false
		if state.Mode == domain.ModeLLMWiki && id != state.SourceRootID {
			seen := map[uuid.UUID]bool{id: true}
			for parent := n.ParentID; parent != nil; {
				if seen[*parent] {
					t.Fatalf("cycle at %s", *parent)
				}
				seen[*parent] = true
				ancestor, ok := f.store.nodes[*parent]
				if !ok || ancestor.NotebookID != n.NotebookID {
					break
				}
				inside = inside || *parent == state.SourceRootID
				parent = ancestor.ParentID
			}
		}
		if inside {
			if n.Kind == domain.KindAsset {
				want = f.infos[id].SHA256
			} else if c := f.store.contents[id]; len(c.Content) > 0 {
				sum := sha256.Sum256([]byte(c.Content))
				want = sum[:]
			}
		}
		if len(want) > 0 {
			count++
		}
		if !bytes.Equal(f.store.sourceHashes[id], want) {
			t.Errorf("source hash for %s (%s) = %x, rebuilt %x", n.Name, id, f.store.sourceHashes[id], want)
		}
	}
	for id := range f.store.trash {
		if len(f.store.sourceHashes[id]) != 0 {
			t.Errorf("trash %s retains source hash", id)
		}
	}
	if count != wantSources {
		t.Errorf("rebuilt eligible sources = %d, want %d", count, wantSources)
	}
}

func sourceHashOf(body string) []byte {
	h := sha256.Sum256([]byte(body))
	return h[:]
}

func TestSourceRootAndAncestorMovesKeepHashesWithoutMetadataReads(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate, domain.ActionMove)
	folder, elsewhere := f.page("Folder", nil, 0), f.page("Elsewhere", nil, 1)
	raw := f.page("raw", &folder.ID, 0)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
	if _, err := f.create(app.PageDraft{ParentID: &raw.ID, Title: "Source", Content: "bytes"}); err != nil {
		t.Fatal(err)
	}
	checkFakeSourceRebuild(t, f, 1)
	for _, move := range []struct {
		id     uuid.UUID
		parent *uuid.UUID
	}{{folder.ID, &elsewhere.ID}, {raw.ID, nil}} {
		f.rec.calls = nil
		if _, err := f.move(move.id, app.Destination{ParentID: move.parent}); err != nil {
			t.Fatal(err)
		}
		checkFakeSourceRebuild(t, f, 1)
		for _, call := range f.rec.calls {
			if strings.Contains(call, "SourceHeads") || strings.Contains(call, "SourceHashes") || strings.HasPrefix(call, "AttachmentInfos") {
				t.Errorf("unchanged source membership did work: %s", call)
			}
		}
	}
}

// S2-1 only bounds the metadata calls; the source-entry policy is a later
// increment. An existing large tree still leaves the source area in chunks.
func TestSourceHashMaintenanceChunksLargeTrees(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionMove)
	raw := f.page("raw", nil, 0)
	// This tree was already a valid source area before this unit. Source-entry
	// now refuses this size, but moving it out must still clear in chunks.
	folder := f.page("Folder", &raw.ID, 1)
	for i := range domain.MaxUnitNodes {
		f.page(fmt.Sprintf("Empty %d", i), &folder.ID, float64(i))
	}
	page := f.page("Body", &folder.ID, domain.MaxUnitNodes)
	f.writeAs(page.ID, "body")
	asset := f.asset("empty.bin", &folder.ID, domain.MaxUnitNodes+1)
	f.infos = map[uuid.UUID]app.AttachmentInfo{asset.ID: {SHA256: sourceHashOf("")}}
	f.store.sourceHashes[page.ID], f.store.sourceHashes[asset.ID] = sourceHashOf("body"), sourceHashOf("")
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
	checkFakeSourceRebuild(t, f, 2)
	f.rec.calls = nil
	if _, err := f.move(folder.ID, app.Destination{}); err != nil {
		t.Fatal(err)
	}
	writes := 0
	for _, call := range f.rec.calls {
		if call == "SetSourceHashes in tx" {
			writes++
		}
		if strings.HasPrefix(call, "SourceHeads") || strings.HasPrefix(call, "AttachmentInfos") || strings.HasPrefix(call, "SubtreeWithin") {
			t.Errorf("large source-out read source-entry metadata: %s", call)
		}
	}
	if writes != 2 {
		t.Errorf("source-out hash writes = %d, want two bounded chunks", writes)
	}
	checkFakeSourceRebuild(t, f, 0)
}

func (f *fakeStore) ProbeSources(ctx context.Context, notebook uuid.UUID, probes []app.SourceProbe) ([]app.SourceMatch, error) {
	f.record(ctx, "ProbeSources")
	if err := f.errs["ProbeSources"]; err != nil {
		return nil, err
	}
	total := 0
	var out []app.SourceMatch
	for i, p := range probes {
		total += p.Take
		if p.Take <= 0 || total > domain.MaxUnitNodes {
			return nil, fmt.Errorf("unbounded probe: %d", total)
		}
		var ids []uuid.UUID
		for id, n := range f.nodes {
			if n.NotebookID == notebook && n.Kind == p.Key.Kind && bytes.Equal(f.sourceHashes[id], p.Key.Hash[:]) && (p.After == nil || bytes.Compare(id[:], p.After[:]) > 0) {
				ids = append(ids, id)
			}
		}
		slices.SortFunc(ids, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
		for _, id := range ids[:min(len(ids), p.Take)] {
			out = append(out, app.SourceMatch{Probe: i, ID: id})
		}
	}
	return out, nil
}
func (f *fakeStore) LockSources(ctx context.Context, _ uuid.UUID) error {
	f.record(ctx, "LockSources")
	return f.errs["LockSources"]
}
func (f *fakeStore) ForgetSources(context.Context, uuid.UUID) error {
	panic("not a page unit operation")
}
