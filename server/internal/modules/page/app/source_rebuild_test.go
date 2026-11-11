package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// rebuildFake is a source area's rebuild's store and attachments: the
// area's nodes, the heads and files it knows, each call traced in order;
// a write changes each row it is given unless short.
type rebuildFake struct {
	nodes  []app.SourceNode
	heads  map[uuid.UUID]app.SourceValue
	files  map[uuid.UUID]app.AttachmentInfo
	err    map[string]error // a call's error, by its name
	short  bool             // the first write changes one row fewer
	calls  []string
	writes []app.SourceValue
}

func (f *rebuildFake) call(name string, n int) error {
	f.calls = append(f.calls, fmt.Sprint(name, " ", n))
	return f.err[name]
}

func (f *rebuildFake) ForgetSources(context.Context, uuid.UUID) error { return f.call("Forget", 0) }

func (f *rebuildFake) SourceNodes(context.Context, uuid.UUID, uuid.UUID) ([]app.SourceNode, error) {
	return f.nodes, f.call("Nodes", 0)
}

func (f *rebuildFake) SourceRebuildHeads(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]app.SourceValue, error) {
	if !slices.IsSortedFunc(ids, uuid.UUID.Compare) || len(ids) > domain.MaxUnitNodes {
		return nil, errors.New("heads not by id, or past the bound")
	}
	out := map[uuid.UUID]app.SourceValue{}
	for _, id := range ids {
		if v, ok := f.heads[id]; ok {
			out[id] = v
		}
	}
	return out, f.call("Heads", len(ids))
}

func (f *rebuildFake) Of(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]app.AttachmentInfo, error) {
	if !slices.IsSortedFunc(ids, uuid.UUID.Compare) {
		return nil, errors.New("attachments not by id")
	}
	out := map[uuid.UUID]app.AttachmentInfo{}
	for _, id := range ids {
		if v, ok := f.files[id]; ok {
			out[id] = v
		}
	}
	return out, f.call("Attachments", len(ids))
}

func (f *rebuildFake) SetSourceHashes(_ context.Context, _ uuid.UUID, values []app.SourceValue) (map[uuid.UUID]app.SourceValue, error) {
	if len(values) > domain.MaxUnitNodes {
		return nil, errors.New("a write past the bound")
	}
	if err := f.call("Set", len(values)); err != nil {
		return nil, err
	}
	f.writes = append(f.writes, values...)
	out := map[uuid.UUID]app.SourceValue{}
	for _, v := range values {
		out[v.ID] = app.SourceValue{ID: v.ID, Kind: v.Kind}
	}
	if f.short {
		f.short = false
		delete(out, values[0].ID)
	}
	return out, nil
}

// rebuildArea is pages pages with content and assets attachments, an empty
// page and a zero-byte attachment among the latter, each its own hash and
// size; its ids in a shuffled order.
func rebuildArea(pages, assets int) *rebuildFake {
	f := &rebuildFake{heads: map[uuid.UUID]app.SourceValue{}, files: map[uuid.UUID]app.AttachmentInfo{}}
	for i := range pages + assets + 1 {
		id := uuid.NewV7()
		sum := sha256.Sum256(fmt.Append(nil, i))
		switch {
		case i < pages:
			f.nodes = append(f.nodes, app.SourceNode{ID: id, Kind: domain.KindPage, PageBytes: int64(i + 1)})
			f.heads[id] = app.SourceValue{ID: id, Kind: domain.KindPage, Hash: sum[:], Bytes: int64(i + 1)}
		case i == pages:
			// An empty page, a folder: its head has the empty content's hash.
			empty := sha256.Sum256(nil)
			f.nodes = append(f.nodes, app.SourceNode{ID: id, Kind: domain.KindPage})
			f.heads[id] = app.SourceValue{ID: id, Kind: domain.KindPage, Hash: empty[:]}
		default:
			f.nodes = append(f.nodes, app.SourceNode{ID: id, Kind: domain.KindAsset})
			f.files[id] = app.AttachmentInfo{MIME: "text/plain", Bytes: int64(i - pages - 1), SHA256: sum[:]}
		}
	}
	slices.Reverse(f.nodes)
	return f
}

// RebuildSources (M11/P3 design 3.4): it clears, reads the area, the
// pages' heads, the attachments' files, then writes, each by id in chunks
// of MaxUnitNodes; it writes each page with content its content's hash and
// size and each attachment its file's, the zero-byte one too, the empty
// page nothing, and answers how many. MaxUnitNodes + 1 pages are two
// chunks of heads. Each fault stops it there, nothing written after: a
// head missing, an attachment's head, a hash of 31 bytes, a page of 0
// bytes, an attachment's file missing, a file's hash of 31 bytes, a write
// that changed one row fewer. A store's error wrapping a deadline stays
// one (%w): the approval answers its deadline as 503.
func TestRebuildSources(t *testing.T) {
	ctx := context.Background()
	notebook, root := uuid.NewV7(), uuid.NewV7()
	f := rebuildArea(3, 3)
	n, err := app.RebuildSources(ctx, f, f, notebook, root)
	if err != nil || n != 6 {
		t.Fatalf("RebuildSources = %d, %v; want 6", n, err)
	}
	if got := strings.Join(f.calls, ", "); got != "Forget 0, Nodes 0, Heads 3, Attachments 3, Set 6" {
		t.Errorf("calls %s", got)
	}
	for _, v := range f.writes {
		node := slices.IndexFunc(f.nodes, func(s app.SourceNode) bool { return s.ID == v.ID })
		want := f.heads[v.ID].Hash
		if v.Kind == domain.KindAsset {
			want = f.files[v.ID].SHA256
		}
		if node < 0 || f.nodes[node].Kind != v.Kind || !bytes.Equal(v.Hash, want) {
			t.Errorf("wrote %+v, want its head's or file's hash", v)
		}
		if v.Kind == domain.KindPage && v.Bytes != f.heads[v.ID].Bytes || v.Kind == domain.KindAsset && v.Bytes != f.files[v.ID].Bytes {
			t.Errorf("wrote %+v, want its size", v)
		}
	}
	if !slices.IsSortedFunc(f.writes[:3], func(a, b app.SourceValue) int { return a.ID.Compare(b.ID) }) || f.writes[0].Kind != domain.KindPage ||
		f.writes[3].Kind != domain.KindAsset || !slices.IsSortedFunc(f.writes[3:], func(a, b app.SourceValue) int { return a.ID.Compare(b.ID) }) {
		t.Errorf("wrote %+v, want the pages, then the attachments, each by id", f.writes)
	}

	big := rebuildArea(domain.MaxUnitNodes+1, 1)
	if n, err := app.RebuildSources(ctx, big, big, notebook, root); err != nil || n != domain.MaxUnitNodes+2 {
		t.Fatalf("RebuildSources of %d pages = %d, %v", domain.MaxUnitNodes+1, n, err)
	}
	if got, want := strings.Join(big.calls, ", "), fmt.Sprintf("Forget 0, Nodes 0, Heads %d, Heads 1, Attachments 1, Set %d, Set 2",
		domain.MaxUnitNodes, domain.MaxUnitNodes); got != want {
		t.Errorf("calls %s, want %s", got, want)
	}

	for _, tc := range []struct {
		name  string
		fault func(f *rebuildFake)
		calls string
	}{
		{"a head missing", func(f *rebuildFake) { delete(f.heads, f.pageID(1)) }, "Forget 0, Nodes 0, Heads 3"},
		{"an attachment's head", func(f *rebuildFake) {
			v := f.heads[f.pageID(0)]
			v.Kind = domain.KindAsset
			f.heads[f.pageID(0)] = v
		}, "Forget 0, Nodes 0, Heads 3"},
		{"a hash of 31 bytes", func(f *rebuildFake) {
			v := f.heads[f.pageID(2)]
			v.Hash = v.Hash[:31]
			f.heads[f.pageID(2)] = v
		}, "Forget 0, Nodes 0, Heads 3"},
		{"a page of 0 bytes", func(f *rebuildFake) {
			v := f.heads[f.pageID(1)]
			v.Bytes = 0
			f.heads[f.pageID(1)] = v
		}, "Forget 0, Nodes 0, Heads 3"},
		{"an attachment's file missing", func(f *rebuildFake) { delete(f.files, f.assetID(0)) }, "Forget 0, Nodes 0, Heads 3, Attachments 3"},
		{"a file's hash of 31 bytes", func(f *rebuildFake) {
			v := f.files[f.assetID(2)]
			v.SHA256 = v.SHA256[:31]
			f.files[f.assetID(2)] = v
		}, "Forget 0, Nodes 0, Heads 3, Attachments 3"},
		{"a write one row short", func(f *rebuildFake) { f.short = true }, "Forget 0, Nodes 0, Heads 3, Attachments 3, Set 6"},
	} {
		f := rebuildArea(3, 3)
		tc.fault(f)
		if n, err := app.RebuildSources(ctx, f, f, notebook, root); err == nil || n != 0 {
			t.Errorf("%s: RebuildSources = %d, %v; want an error", tc.name, n, err)
		}
		if got := strings.Join(f.calls, ", "); got != tc.calls {
			t.Errorf("%s: calls %s, want %s", tc.name, got, tc.calls)
		}
		if tc.name != "a write one row short" && len(f.writes) != 0 {
			t.Errorf("%s: wrote %d", tc.name, len(f.writes))
		}
	}

	for _, name := range []string{"Forget", "Nodes", "Heads", "Attachments", "Set"} {
		f := rebuildArea(3, 3)
		f.err = map[string]error{name: fmt.Errorf("the store: %w", context.DeadlineExceeded)}
		if _, err := app.RebuildSources(ctx, f, f, notebook, root); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("%s past the deadline: RebuildSources = %v, want it to wrap the deadline", name, err)
		}
	}
}

// pageID is the id of the i-th page with content, by id; assetID the i-th
// attachment's.
func (f *rebuildFake) pageID(i int) uuid.UUID { return f.byKind(domain.KindPage, true)[i] }

func (f *rebuildFake) assetID(i int) uuid.UUID { return f.byKind(domain.KindAsset, false)[i] }

func (f *rebuildFake) byKind(kind domain.Kind, content bool) []uuid.UUID {
	var out []uuid.UUID
	for _, n := range f.nodes {
		if n.Kind == kind && (!content || n.PageBytes > 0) {
			out = append(out, n.ID)
		}
	}
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}
