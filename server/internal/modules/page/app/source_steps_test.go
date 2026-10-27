package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestSourceStepProducersKeepContentKindsAndBothLines(t *testing.T) {
	for _, direct := range []bool{false, true} {
		name := "create"
		if direct {
			name = "direct_import"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionCreate)
			raw := f.page("raw", nil, 0)
			f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
			g := &guard{recorder: f.rec}
			f.guards = []app.WriteGuard{g}
			if direct {
				_, err := f.importAs(uuid.Nil(), func(ctx context.Context, u *app.ImportUnit) error {
					if _, err := u.CreatePage(ctx, app.ImportedPage{ParentID: &raw.ID, Name: "Note", Content: "原文"}); err != nil {
						return err
					}
					_, err := u.CreateAsset(ctx, app.ImportedAsset{ParentID: &raw.ID, Name: "file.txt"}, func(context.Context, domain.Node) error { return nil })
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := f.create(app.PageDraft{ParentID: &raw.ID, Title: "Note", Content: "原文"}); err != nil {
					t.Fatal(err)
				}
			}
			s := g.steps[0]
			sum := sha256.Sum256([]byte("原文"))
			if s.Content == nil || s.Content.Bytes != 6 || s.Content.BeforeBytes != 0 || s.Content.Merged != 1 || !bytes.Equal(s.Content.Hash, sum[:]) {
				t.Fatalf("create content facts = %+v", s.Content)
			}
			if s.Kinds[s.Changes[0].NodeID] != domain.KindPage || s.Lines == nil || len(s.Lines.Before) != 0 || !slices.Equal(s.Lines.After, []domain.Ancestor{{ID: raw.ID, Name: "raw"}}) {
				t.Fatalf("create kinds/lines = %v / %+v", s.Kinds, s.Lines)
			}
			if direct {
				s = g.steps[1]
				if s.Kinds[s.Changes[0].NodeID] != domain.KindAsset || s.Content != nil || s.Lines == nil || !slices.Equal(s.Lines.After, []domain.Ancestor{{ID: raw.ID, Name: "raw"}}) {
					t.Fatalf("import asset facts = %+v", s)
				}
			}
			count := 0
			for _, call := range f.rec.calls {
				if call == "Ancestors in tx" {
					count++
				}
			}
			wantReads := 2
			if direct {
				wantReads = 1
			}
			if count != wantReads {
				t.Fatalf("creation ancestors read %d times, want %d", count, wantReads)
			}
		})
	}
	t.Run("content_and_move", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionWrite, domain.ActionMove)
		raw, other := f.page("raw", nil, 0), f.page("Other", nil, 1)
		p := f.page("P", &raw.ID, 0)
		a := f.asset("a.txt", &p.ID, 0)
		f.writeAs(p.ID, "old")
		f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
		g := &guard{recorder: f.rec}
		f.guards = []app.WriteGuard{g}
		if _, err := f.put(p.ID, app.ContentPut{Content: "更新", Base: 2}); err != nil {
			t.Fatal(err)
		}
		s := g.steps[0]
		if s.Content == nil || s.Content.BeforeBytes != 3 || s.Content.Bytes != 6 || s.Content.Merged != 1 || s.Rewrite || s.Kinds[p.ID] != domain.KindPage || s.Lines == nil || !slices.Equal(s.Lines.Before, s.Lines.After) || len(s.Lines.Before) != 1 {
			t.Fatalf("content facts = %+v", s)
		}
		if _, err := f.move(p.ID, app.Destination{ParentID: &other.ID}); err != nil {
			t.Fatal(err)
		}
		s = g.steps[1]
		if s.Kinds[p.ID] != domain.KindPage || s.Kinds[a.ID] != domain.KindAsset || s.Lines == nil || !slices.Equal(s.Lines.Before, []domain.Ancestor{{ID: raw.ID, Name: "raw"}}) || !slices.Equal(s.Lines.After, []domain.Ancestor{{ID: other.ID, Name: "Other"}}) {
			t.Fatalf("move kinds/lines = %v / %+v", s.Kinds, s.Lines)
		}
	})
}

func TestSourceStepMetadataAddsNoReadsInStandardMode(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionWrite)
	raw := f.page("raw", nil, 0)
	p := f.page("P", &raw.ID, 0)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeStandard, SourceRootID: raw.ID}}
	g := &guard{recorder: f.rec}
	f.guards = []app.WriteGuard{g, app.NewModeGuard(f.auth)}
	if _, err := f.put(p.ID, app.ContentPut{Content: "ordinary", Base: 1}); err != nil {
		t.Fatal(err)
	}
	reads := 0
	for _, call := range f.rec.calls {
		if call == "Ancestors in tx" {
			reads++
		}
		if strings.Contains(call, "page.manage_source") {
			t.Fatalf("standard added %s", call)
		}
	}
	if reads != 1 {
		t.Fatalf("standard ancestor reads=%d, want existing response read 1", reads)
	}
	if g.steps[0].Content == nil || g.steps[0].Kinds[p.ID] != domain.KindPage {
		t.Fatalf("ordinary metadata missing: %+v", g.steps[0])
	}
}

func TestSourceBatchProducersCarryTheActualMergedCount(t *testing.T) {
	for _, created := range []bool{false, true} {
		t.Run(map[bool]string{false: "write", true: "create"}[created], func(t *testing.T) {
			f := newFixture()
			raw := f.page("raw", nil, 0)
			f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
			var page domain.Node
			if !created {
				page = f.page("P", &raw.ID, 0)
			}
			g := &guard{recorder: f.rec}
			f.guards = []app.WriteGuard{g}
			first := writeOp("raw/P.md", "A")
			if !created {
				first.Base = new((domain.VersionToken{Node: page.ID, Revision: 1}).String())
			}
			_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, first, editOp("raw/P.md", "A", "BB"))
			if err != nil {
				t.Fatal(err)
			}
			if len(g.steps) != 1 || g.steps[0].Content == nil || g.steps[0].Content.Merged != 2 || g.steps[0].Content.Bytes != 2 {
				t.Fatalf("batch step facts=%+v", g.steps)
			}
		})
	}
}

func TestSourceStructuralProducersKeepAssetKindsAndPlannedLines(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename, domain.ActionRestore)
	raw := f.page("raw", nil, 0)
	p := f.page("P", &raw.ID, 0)
	a := f.asset("a.txt", &p.ID, 0)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
	g := &guard{recorder: f.rec}
	f.guards = []app.WriteGuard{g}
	_, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
		if _, err := u.CreateAsset(ctx, app.AssetDraft{ParentID: &p.ID, Name: "b.txt"}); err != nil {
			return err
		}
		if _, err := u.Rename(ctx, a.ID, "renamed.txt"); err != nil {
			return err
		}
		if _, err := u.Relocate(ctx, a.ID, app.Placement{ParentID: &p.ID, Name: "relocated.txt", Field: "to"}); err != nil {
			return err
		}
		_, err := u.Delete(ctx, p.ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.steps) != 4 {
		t.Fatalf("steps=%d", len(g.steps))
	}
	for i, s := range g.steps {
		if s.Lines == nil {
			t.Fatalf("step %d missing lines", i)
		}
		if i < 3 && s.Kinds[s.Changes[0].NodeID] != domain.KindAsset {
			t.Fatalf("step %d kind=%v", i, s.Kinds)
		}
		if i == 0 && len(s.Lines.After) != 2 || i > 0 && i < 3 && (len(s.Lines.Before) != 2 || !slices.Equal(s.Lines.Before, s.Lines.After)) {
			t.Fatalf("step %d lines=%+v", i, s.Lines)
		}
		if i == 3 && (s.Kinds[p.ID] != domain.KindPage || s.Kinds[a.ID] != domain.KindAsset || len(s.Lines.Before) != 1 || len(s.Lines.After) != 0) {
			t.Fatalf("delete facts=%+v", s)
		}
	}
	g.steps = nil
	f.rec.calls = nil
	if _, err := app.NewRestoreNode(f.trashReads(now()), f.writer(), f.parser(), app.RestoreBounds{}, f.logger()).Execute(f.asAlice(), p.ID, domain.ClientWeb); err != nil {
		t.Fatal(err)
	}
	if len(g.steps) != 1 {
		t.Fatalf("restore steps=%d", len(g.steps))
	}
	for _, call := range f.rec.calls {
		if strings.HasPrefix(call, "Ancestors") {
			t.Fatalf("restore reread known ancestor line: %s", call)
		}
	}
	s := g.steps[0]
	if s.Kinds[p.ID] != domain.KindPage || s.Kinds[a.ID] != domain.KindAsset || s.Lines == nil || len(s.Lines.Before) != 0 || !slices.Equal(s.Lines.After, []domain.Ancestor{{ID: raw.ID, Name: "raw"}}) {
		t.Fatalf("restore facts=%+v", s)
	}
}

func TestSourceAppenderStepCarriesRewriteAndNoSession(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	target := f.page("Target", nil, 0)
	raw := f.page("raw", nil, 1)
	source := f.page("S", &raw.ID, 0)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
	g := &guard{recorder: f.rec}
	f.guards = []app.WriteGuard{g}
	p := &participant{recorder: f.rec, write: &app.ContentWrite{NodeID: source.ID, Content: "rewritten", Base: 1, EditSession: uuid.NewV7()}}
	f.partakers = []app.Participant{p}
	if _, err := f.rename(target.ID, "Other"); err != nil {
		t.Fatal(err)
	}
	if len(g.steps) != 2 {
		t.Fatalf("steps=%d", len(g.steps))
	}
	s := g.steps[1]
	if !s.Rewrite || s.EditSessionID != uuid.Nil() || s.Content == nil || s.Content.Bytes != 9 || s.Content.BeforeBytes != 0 || s.Kinds[source.ID] != domain.KindPage || s.Lines == nil || len(s.Lines.Before) != 1 {
		t.Fatalf("appender facts=%+v", s)
	}
}
