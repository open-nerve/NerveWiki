package app_test

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The fake keeps the same conservative marker as RecordRevision's SQL:
// any ordinary write makes the changeset's version ordinary.
func TestRevisionsKeepOnlyAllRewriteWrites(t *testing.T) {
	for _, tt := range []struct {
		first, next, want bool
	}{{true, true, true}, {true, false, false}, {false, true, false}, {false, false, false}} {
		t.Run(fmt.Sprintf("%t/%t", tt.first, tt.next), func(t *testing.T) {
			f := newFixture()
			first := app.Revision{ID: uuid.NewV7(), ChangesetID: uuid.NewV7(), NodeID: uuid.NewV7(), Base: new(1), Revision: 2,
				Content: "first", Hash: hashOf("first"), ByteSize: 5, Rewrite: tt.first, At: now()}
			if err := f.store.RecordRevision(context.Background(), first); err != nil {
				t.Fatal(err)
			}
			if got := f.store.revisions[0].Rewrite; got != tt.first {
				t.Errorf("first write marker = %t, want %t", got, tt.first)
			}
			next := first
			next.ID, next.Base, next.Revision = uuid.NewV7(), new(2), 3
			next.Content, next.Hash, next.ByteSize, next.Rewrite, next.At = "末", hashOf("末"), 3, tt.next, now().Add(time.Second)
			if err := f.store.RecordRevision(context.Background(), next); err != nil {
				t.Fatal(err)
			}
			got := f.store.revisions[0]
			if len(f.store.revisions) != 1 || got.Rewrite != tt.want || got.ID != first.ID || *got.Base != 1 ||
				got.Revision != 3 || got.Content != "末" || got.ByteSize != 3 || !bytes.Equal(got.Hash, hashOf("末")) || !got.At.Equal(next.At) {
				t.Errorf("merged revision = %+v, want original id/base, final body and rewrite=%t", got, tt.want)
			}
			next.Base, next.Revision = new(4), 5
			if err := f.store.RecordRevision(context.Background(), next); err == nil || f.store.revisions[0].Revision != 3 {
				t.Errorf("non-contiguous write = %v, revisions %+v; want refusal with the old row kept", err, f.store.revisions)
			}
		})
	}
}

// Appenders mark their writes, but cannot turn a version that also holds
// an ordinary write into a system rewrite.
func TestParticipantsKeepOnlyTheirOwnVersionsAsRewrites(t *testing.T) {
	for _, ordinary := range []bool{false, true} {
		t.Run(fmt.Sprintf("ordinary=%t", ordinary), func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRename)
			n := f.page("Target", nil, 0)
			source := f.page("Source", nil, 1)
			p := &participant{recorder: f.rec}
			f.partakers = []app.Participant{p}
			_, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
				if ordinary {
					if _, err := u.WriteContent(ctx, app.ContentWrite{NodeID: source.ID, Content: "ordinary", Base: 1}); err != nil {
						return err
					}
				}
				for i, text := range []string{"first rewrite", "last rewrite"} {
					p.write = &app.ContentWrite{NodeID: source.ID, Content: text, Base: f.store.contents[source.ID].Revision}
					if _, err := u.Rename(ctx, n.ID, fmt.Sprintf("Renamed %d", i)); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(f.store.revisions) != 1 {
				t.Fatalf("versions = %+v, want one merged version", f.store.revisions)
			}
			r := f.store.revisions[0]
			wantRevision := 3
			if ordinary {
				wantRevision = 4
			}
			if r.Rewrite != !ordinary || *r.Base != 1 || r.Revision != wantRevision || r.Content != "last rewrite" {
				t.Errorf("version = %+v, want from 1 to %d, rewrite=%t", r, wantRevision, !ordinary)
			}
		})
	}
}

// The marker of the version being undone and the bytes of its base must
// survive both compaction and actual writes, including the next undo.
func TestSourceRevertsCarryRewriteAcrossBatches(t *testing.T) {
	t.Run("restore continuation", sourceRewriteContinuation)
	for _, batch := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("batch=%d", batch), func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRevert)
			a, b := f.page("System", nil, 0), f.page("Ordinary", nil, 1)
			ids := []uuid.UUID{a.ID, b.ID}
			before := map[uuid.UUID]string{a.ID: "原", b.ID: ""}
			after := map[uuid.UUID]string{a.ID: "rewritten", b.ID: "user wrote"}
			for _, id := range ids {
				f.content(id, before[id], 1)
				f.content(id, after[id], 2)
			}
			va := f.log.version(a.ID, 1, 2, after[a.ID])
			va.Rewrite = true
			c := f.log.changeset(domain.ChangesetEdit, f.alice, now(), nil, va, f.log.version(b.ID, 1, 2, after[b.ID]))
			for round := range 2 {
				want := before
				if round == 1 {
					want = after
				}
				contents, err := f.log.ContentsNow(context.Background(), ids)
				if err != nil {
					t.Fatal(err)
				}
				for _, id := range ids {
					if contents[id].Bytes != len(f.store.contents[id].Content) {
						t.Errorf("current bytes = %d, want %d", contents[id].Bytes, len(f.store.contents[id].Content))
					}
				}
				var undone uuid.UUID
				if batch == 0 {
					got, err := f.revert(c.ID, nil, 0)
					if err != nil || !got.Changed {
						t.Fatalf("undo round %d = %+v, %v", round, got, err)
					}
					undone = got.ChangesetID
				} else {
					w, spec := f.jobs(now(), 0, 0), f.jobSpec()
					plan, err := w.PlanRevert(f.asJob(), spec, c.ID)
					if err != nil {
						t.Fatal(err)
					}
					steps, _ := plan.RevertSteps()
					if len(steps) != 2 {
						t.Fatalf("steps = %+v, want two content writes", steps)
					}
					for i, s := range steps {
						if s.Kind != domain.RevertContent || s.Rewrite != (s.Node == a.ID) || s.Bytes != len(want[s.Node]) ||
							!bytes.Equal(s.Hash, hashOf(want[s.Node])) || plan.Steps()[i].Bytes != int64(len(want[s.Node])) {
							t.Errorf("round %d compact step = %+v, weight %+v; want base bytes and the reverted marker", round, s, plan.Steps()[i])
						}
					}
					if done, err := f.runJob(w, spec, plan, batch); err != nil || len(done) != (2+batch-1)/batch {
						t.Fatalf("run round %d = %+v, %v", round, done, err)
					}
					undone = spec.Changeset
				}
				for _, id := range ids {
					current := f.store.contents[id]
					if current.Content != want[id] || current.Revision != 3+round {
						t.Errorf("round %d content = %+v, want %q at %d", round, current, want[id], 3+round)
					}
					found := 0
					for _, r := range f.store.revisions {
						if r.NodeID == id && r.ChangesetID == undone {
							found++
							if r.Rewrite != (id == a.ID) || r.ByteSize != len(want[id]) || *r.Base != 2+round {
								t.Errorf("round %d stored version = %+v, want rewrite=%t and target bytes", round, r, id == a.ID)
							}
						}
					}
					if found != 1 {
						t.Errorf("round %d has %d versions for %s, want 1", round, found, id)
					}
				}
				c = f.recorded(undone, c.ID)
			}
		})
	}
}

// The root commits first, leaving each child a trash root. Each later
// batch continues the original group and keeps the restored page joined
// to its write back, including the ordinary page next to the rewrite.
func sourceRewriteContinuation(t *testing.T) {
	s := newJobScene()
	root := s.node("F", nil, false)
	a, b := s.node("A", &root, false), s.node("B", &root, false)
	s.content(a, "原")
	s.content(b, "")
	e := s.begin()
	e.write(a)
	e.write(b)
	e.del(root)
	f, c := s.load()
	for i := range c.versions {
		c.versions[i].Rewrite = c.versions[i].NodeID == a
	}
	w, spec, plan := f.revertJob(t, c.ID, nil, now())
	steps, _ := plan.RevertSteps()
	if got := s.describe(steps, plan.Steps()); !slices.Equal(got, []string{"restore F", "restore A+", "content A", "restore B+", "content B"}) {
		t.Fatalf("steps = %v, want the root followed by each child's joined restore/write", got)
	}
	for _, i := range []int{2, 4} {
		step := steps[i]
		wantBytes := 0
		if step.Node == a {
			wantBytes = 3
		}
		if step.Rewrite != (step.Node == a) || step.Bytes != wantBytes || plan.Steps()[i].Bytes != int64(wantBytes) {
			t.Errorf("continuation write = %+v, weight %+v; want rewrite=%t and base bytes %d", step, plan.Steps()[i], step.Node == a, wantBytes)
		}
	}
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 1); err != nil {
		t.Fatal(err)
	}
	if _, alive := f.store.nodes[root]; !alive || !f.store.trash[a].TrashRoot || !f.store.trash[b].TrashRoot || len(f.store.revisions) != 0 {
		t.Fatalf("after the root: root alive=%t, A=%+v B=%+v versions=%+v; want the children left as trash roots", alive,
			f.store.trash[a], f.store.trash[b], f.store.revisions)
	}
	for i, id := range []uuid.UUID{a, b} {
		from := 1 + 2*i
		if _, err := w.RunBatch(f.asJob(), spec, plan, from, from+2); err != nil {
			t.Fatalf("continue child %d: %v", i, err)
		}
		n := f.store.nodes[id]
		if n.ParentID == nil || *n.ParentID != root || f.store.contents[id].Revision != 3 {
			t.Errorf("continued child = %+v, content %+v; want under F at revision 3", n, f.store.contents[id])
		}
		if i == 0 && !f.store.trash[b].TrashRoot {
			t.Error("B no longer waits in the trash after A's continuation")
		}
	}
	want := map[uuid.UUID]string{a: "原", b: ""}
	check := func(changeset uuid.UUID, base int, texts map[uuid.UUID]string) {
		t.Helper()
		seen := 0
		for _, r := range f.store.revisions {
			if r.ChangesetID != changeset {
				continue
			}
			seen++
			text, ok := texts[r.NodeID]
			if !ok || r.Rewrite != (r.NodeID == a) || r.Base == nil || *r.Base != base || r.Revision != base+1 ||
				r.Content != text || r.ByteSize != len(text) || !bytes.Equal(r.Hash, hashOf(text)) {
				t.Errorf("continuation's version = %+v, want target bytes and rewrite=%t", r, r.NodeID == a)
			}
		}
		if seen != 2 {
			t.Errorf("continuation wrote %d versions, want both pages", seen)
		}
	}
	check(spec.Changeset, 2, want)
	undone := f.recorded(spec.Changeset, c.ID)
	again, err := f.revert(undone.ID, nil, 0)
	if err != nil || !again.Changed {
		t.Fatalf("undo of the continuation = %+v, %v", again, err)
	}
	check(again.ChangesetID, 3, map[uuid.UUID]string{
		a: s.p.versions[domain.VersionRef{NodeID: a, Revision: 2}],
		b: s.p.versions[domain.VersionRef{NodeID: b, Revision: 2}],
	})
}
