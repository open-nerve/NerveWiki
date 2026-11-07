package app

import (
	"reflect"
	"slices"
	"testing"
	"uuid"
)

func TestDroppedTargetsWitnessPreservesPublishedUnion(t *testing.T) {
	ids := make([]uuid.UUID, 22)
	for i := range ids {
		ids[i] = uuid.UUID{15: byte(i + 1)}
	}
	for _, tc := range []struct {
		name           string
		pages, witness [][]uuid.UUID
		want           []uuid.UUID
		none           bool
	}{
		{"single20", [][]uuid.UUID{ids[:20]}, nil, ids[:20], false},
		{"single21", [][]uuid.UUID{ids[:21]}, nil, nil, false},
		{"single22_witness", [][]uuid.UUID{ids}, [][]uuid.UUID{ids[:21]}, nil, false},
		{"union20", [][]uuid.UUID{ids[:10], ids[10:20]}, nil, ids[:20], false},
		{"union21", [][]uuid.UUID{ids[:10], ids[10:21]}, nil, nil, false},
		{"overlap20", [][]uuid.UUID{ids[:12], ids[8:20]}, nil, ids[:20], false},
		{"overlap21", [][]uuid.UUID{ids[:12], ids[8:21]}, nil, nil, false},
		{"duplicate_and_zero20", [][]uuid.UUID{ids[:20], {{}, ids[0], ids[19]}}, nil, ids[:20], false},
		{"overflow_plus_other_page", [][]uuid.UUID{ids, {{}, ids[0]}}, [][]uuid.UUID{ids[:21], {{}, ids[0]}}, nil, false},
		{"only_zero", [][]uuid.UUID{{{}}, {{}}}, nil, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			witness := tc.witness
			if witness == nil {
				witness = tc.pages
			}
			for _, groups := range [][][]uuid.UUID{tc.pages, witness} {
				var dropped []uuid.UUID
				for _, page := range groups {
					dropped = append(dropped, page...)
				}
				original := slices.Clone(dropped)
				pub := &positionEventPublisher{}
				x := Index{Publisher: pub}
				event := PagesChanged{WorkspaceID: uuid.UUID{15: 40}, NotebookID: uuid.UUID{15: 41}}
				if err := x.publish(t.Context(), event, nil, nil, nil, dropped, false); err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(dropped, original) {
					t.Fatal("publisher changed its caller's target input")
				}
				if tc.none {
					if len(pub.events) != 0 {
						t.Fatalf("zero-only event=%+v", pub.events)
					}
					continue
				}
				want := []LinksChanged{{WorkspaceID: event.WorkspaceID, NotebookID: event.NotebookID, Pages: []uuid.UUID{}, Targets: tc.want}}
				if !reflect.DeepEqual(pub.events, want) {
					t.Fatalf("DROPPED_WITNESS_PUBLIC: got%+v want%+v including nil/empty", pub.events, want)
				}
			}
		})
	}
}
