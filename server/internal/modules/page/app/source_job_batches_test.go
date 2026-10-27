package app_test

import (
	"context"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestSourceBackgroundCallbacksAreRequiredAndChecked(t *testing.T) {
	for _, route := range []string{"check", "plan"} {
		for _, mode := range []domain.Mode{domain.ModeStandard, domain.ModeLLMWiki} {
			t.Run(route+"/"+string(mode), func(t *testing.T) {
				f := newRevertFixture()
				f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: mode, SourceRootID: uuid.NewV7()}}
				run := func(b app.JobBatches) error {
					w := f.jobs(now(), 0, 0)
					if route == "check" {
						_, err := w.CheckRevert(f.asAlice(), f.eng, f.c.ID, nil, nil, b)
						return err
					}
					_, err := w.PlanRevert(f.asJob(), f.jobSpec(), f.c.ID, b)
					return err
				}
				if err := run(nil); err == nil || !strings.Contains(err.Error(), "batch") {
					t.Fatalf("missing callback = %v, want internal batch error", err)
				}
				calls := 0
				if err := run(func(s []domain.JobStep) [][2]int { calls++; return allJobBatches(s) }); err != nil {
					t.Fatal(err)
				}
				want := 0
				if mode == domain.ModeLLMWiki {
					want = 1
				}
				if calls != want {
					t.Fatalf("callback calls=%d, want %d", calls, want)
				}
				if mode == domain.ModeStandard {
					for _, c := range f.rec.calls {
						if strings.HasPrefix(c, "TrashLines ") || strings.Contains(c, "page.manage_source") {
							t.Fatalf("ordinary added %s", c)
						}
					}
					return
				}
				for _, cuts := range [][][2]int{{{1, 3}}, {{0, 0}}, {{0, 2}, {1, 3}}, {{0, 999}}} {
					if err := run(func([]domain.JobStep) [][2]int { return cuts }); err == nil || !strings.Contains(err.Error(), "batch") {
						t.Fatalf("invalid cuts %v = %v", cuts, err)
					}
				}
			})
		}
	}
}

func TestSourceContentOnlyRevertReadsOnlyRelevantLines(t *testing.T) {
	for _, route := range []string{"sync", "check", "plan"} {
		for _, mode := range []domain.Mode{domain.ModeStandard, domain.ModeLLMWiki} {
			t.Run(route+"/"+string(mode), func(t *testing.T) {
				f := newFixture()
				f.grant(domain.ActionRevert)
				raw := f.page("raw", nil, 0)
				p := f.page("P", &raw.ID, 0)
				// A persisted system rewrite is allowed back through the now
				// active source contract; a legacy user write is intentionally not.
				c := f.writing(p.ID, "updated")
				c.versions[0].Rewrite = true
				f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: mode, SourceRootID: raw.ID}}
				f.rec.calls = nil
				var err error
				switch route {
				case "sync":
					_, err = f.revert(c.ID, nil, 0)
				case "check":
					_, err = f.jobs(now(), 0, 0).CheckRevert(f.asAlice(), f.eng, c.ID, nil, nil, allJobBatches)
				case "plan":
					_, err = f.jobs(now(), 0, 0).PlanRevert(f.asJob(), f.jobSpec(), c.ID, allJobBatches)
				}
				if err != nil {
					t.Fatal(err)
				}
				reads := 0
				for _, call := range f.rec.calls {
					if strings.HasPrefix(call, "ListNodes") || strings.Contains(call, "page.manage_source") {
						t.Fatalf("content-only added %s", call)
					}
					if strings.HasPrefix(call, "TrashLines ") {
						reads++
						if !strings.HasPrefix(call, "TrashLines 1") {
							t.Fatalf("unexpected line batch: %s", call)
						}
					}
				}
				want := 0
				if mode == domain.ModeLLMWiki {
					want = 1
					if route == "sync" {
						want = 2
					}
				}
				if reads != want {
					t.Fatalf("ancestor batches=%d want %d; calls=%v", reads, want, f.rec.calls)
				}
			})
		}
	}
}

func TestSourceBackgroundPortsKeepJoinedAndEmptyPlans(t *testing.T) {
	for _, route := range []string{"check", "plan"} {
		t.Run(route+"/joined", func(t *testing.T) {
			s := newJobScene()
			root := s.node("F", nil, false)
			a := s.node("A", &root, false)
			s.content(a, "original")
			e := s.begin()
			e.write(a)
			e.del(root)
			f, c := s.load()
			f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: uuid.NewV7()}}
			cuts := func(steps []domain.JobStep) [][2]int {
				if len(steps) != 3 || !steps[1].Joined {
					t.Fatalf("steps=%v, want restore F, restore A+, content A", steps)
				}
				return [][2]int{{0, 2}, {2, 3}}
			}
			w := f.jobs(now(), 0, 0)
			var err error
			if route == "check" {
				_, err = w.CheckRevert(f.asAlice(), f.eng, c.ID, nil, nil, cuts)
			} else {
				_, err = w.PlanRevert(f.asJob(), f.jobSpec(), c.ID, cuts)
			}
			if err == nil || !strings.Contains(err.Error(), "batch cuts") {
				t.Fatalf("Joined cut = %v", err)
			}
		})
		t.Run(route+"/empty", func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRevert, domain.ActionWrite, domain.ActionRename)
			p := f.page("P", nil, 0)
			if _, err := f.put(p.ID, app.ContentPut{Content: "same", Base: 1}); err != nil {
				t.Fatal(err)
			}
			f.log.versions[domain.VersionRef{NodeID: p.ID, Revision: 2}] = "same"
			out, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
				if _, err := u.WriteContent(ctx, app.ContentWrite{NodeID: p.ID, Content: "other", Base: 2}); err != nil {
					return err
				}
				_, err := u.WriteContent(ctx, app.ContentWrite{NodeID: p.ID, Content: "same", Base: 3})
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			c := f.recorded(out.ChangesetID, uuid.Nil())
			c.Kind = domain.ChangesetEdit
			if len(c.versions) != 1 || c.versions[0].Base == nil || *c.versions[0].Base != 2 || c.versions[0].Revision != 4 {
				t.Fatalf("merged history=%+v", c.versions)
			}
			f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: uuid.NewV7()}}
			calls := 0
			cuts := func(steps []domain.JobStep) [][2]int {
				calls++
				if len(steps) != 0 {
					t.Fatalf("no-op steps=%v", steps)
				}
				return nil
			}
			w := f.jobs(now(), 0, 0)
			if route == "check" {
				_, err = w.CheckRevert(f.asAlice(), f.eng, c.ID, nil, nil, cuts)
			} else {
				var plan app.JobPlan
				plan, err = w.PlanRevert(f.asJob(), f.jobSpec(), c.ID, cuts)
				if len(plan.Steps()) != 0 || plan.Total() != 0 {
					t.Fatalf("no-op plan=%v total=%d", plan.Steps(), plan.Total())
				}
			}
			if err != nil || calls != 1 {
				t.Fatalf("empty plan err=%v callback=%d", err, calls)
			}
		})
	}
}
