package domain

import (
	"slices"
	"testing"
	"uuid"
)

// Each literal result states the preference contract independently of the
// selection loop. Every input order must keep the same winner and ambiguity,
// including duplicate IDs and a preferred subtree discovered after a tie.
func TestPreferredCandidatesKeepOrderIndependentMeaning(t *testing.T) {
	id := func(n byte) uuid.UUID { return uuid.UUID{15: n} }
	folder := []Step{{ID: id(40), Name: "long-folder", Key: "long-folder"}}
	node := func(n, parent byte, name string) Node {
		return Node{ID: id(n), Path: []Step{{ID: id(parent), Name: name, Key: name}, {ID: id(n), Name: "dup", Key: "dup"}}}
	}
	insideLong := node(9, 40, "long-folder")
	insideShort := Node{ID: id(40), Path: slices.Clone(folder)}
	outsideLow := node(1, 50, "x")
	outsideHigh := node(2, 51, "y")
	outsideLong := node(3, 52, "long")
	unicodeShort := node(5, 53, "éé")
	unicodeEqual := node(4, 54, "😀")
	for _, tt := range []struct {
		name   string
		nodes  []Node
		folder []Step
		want   Resolution
	}{
		{"root-shortest-and-id-tie", []Node{outsideLong, outsideHigh, outsideLow}, nil, Resolution{ID: id(1), Ambiguous: true}},
		{"shorter-candidate-clears-an-earlier-tie", []Node{outsideLong, node(6, 55, "wide"), outsideLow}, nil, Resolution{ID: id(1)}},
		{"subtree-replaces-earlier-global-tie", []Node{outsideLow, outsideHigh, insideLong}, folder, Resolution{ID: id(9)}},
		{"folder-own-page-is-inside", []Node{insideLong, outsideLow, insideShort}, folder, Resolution{ID: id(40)}},
		{"no-subtree-keeps-global-choice", []Node{outsideLong, outsideLow, outsideHigh}, folder, Resolution{ID: id(1), Ambiguous: true}},
		{"utf16-equal-length", []Node{unicodeShort, unicodeEqual, outsideLong}, nil, Resolution{ID: id(4), Ambiguous: true}},
		{"duplicate-id-still-ambiguous", []Node{insideLong, insideLong, outsideLow}, folder, Resolution{ID: id(9), Ambiguous: true}},
		{"zero-id-wins-equal-length", []Node{node(0, 60, "x"), outsideLow}, nil, Resolution{Ambiguous: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			permutations := 0
			var visit func(int)
			visit = func(at int) {
				if at == len(tt.nodes) {
					permutations++
					if got := preferred(tt.nodes, tt.folder); got != tt.want {
						t.Fatalf("permutation %d: preferred=%+v, want %+v", permutations, got, tt.want)
					}
					return
				}
				for i := at; i < len(tt.nodes); i++ {
					tt.nodes[at], tt.nodes[i] = tt.nodes[i], tt.nodes[at]
					visit(at + 1)
					tt.nodes[at], tt.nodes[i] = tt.nodes[i], tt.nodes[at]
				}
			}
			visit(0)
			want := 1
			for i := 2; i <= len(tt.nodes); i++ {
				want *= i
			}
			if permutations != want {
				t.Fatalf("visited %d input orders, want %d", permutations, want)
			}
		})
	}
}
