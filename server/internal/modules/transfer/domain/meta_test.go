package domain_test

import (
	"encoding/json"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// meta.json lists the nodes written, in the plan's order: a page's file,
// or its folder; an attachment's file. The contributors' files and the
// root of a subtree follow.
func TestMetaJSON(t *testing.T) {
	nb := domain.Named{ID: uuid.MustParse("0199a2b4-0000-7000-8000-000000000001"), Name: "笔记"}
	a := uuid.MustParse("0199a2b4-0000-7000-8000-000000000002")
	b := uuid.MustParse("0199a2b4-0000-7000-8000-000000000003")
	x := uuid.MustParse("0199a2b4-0000-7000-8000-000000000004")
	y := uuid.MustParse("0199a2b4-0000-7000-8000-000000000005")
	nodes := []domain.Node{
		{ID: a, Name: "项目A", SortOrder: 1.5, Empty: true},
		{ID: b, ParentID: &a, Name: "需求", SortOrder: 3},
		{ID: x, ParentID: &a, Asset: true, Name: "图.png", SortOrder: 4},
		{ID: y, ParentID: &a, Asset: true, Name: "缺.pdf", SortOrder: 5},
	}
	p, err := domain.NewPlan(nb, nil, nodes, func(uuid.UUID) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Contribute("index.md"); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 9, 12, 0, 0, 500, time.FixedZone("CST", 8*3600))

	m := p.Meta(at, nb, nil, func(e domain.Entry) bool { return e.Node.ID != y })
	got, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"format":1,"exported_at":"2026-10-09T04:00:00Z","notebook":{"id":"0199a2b4-0000-7000-8000-000000000001","name":"笔记"},` +
		`"root":null,"nodes":[` +
		`{"path":"项目A/","kind":"page","id":"0199a2b4-0000-7000-8000-000000000002","sort_order":1.5},` +
		`{"path":"项目A/需求.md","kind":"page","id":"0199a2b4-0000-7000-8000-000000000003","sort_order":3},` +
		`{"path":"项目A/图.png","kind":"asset","id":"0199a2b4-0000-7000-8000-000000000004","sort_order":4}],` +
		`"contributed":["index.md"]}`
	if string(got) != want {
		t.Errorf("meta.json =\n%s\nwant\n%s", got, want)
	}

	root := domain.Named{ID: a, Name: "项目A"}
	sub, err := domain.NewPlan(nb, &root, nodes[:2], func(uuid.UUID) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	got, _ = json.Marshal(sub.Meta(at, nb, &root, func(domain.Entry) bool { return true }))
	want = `{"format":1,"exported_at":"2026-10-09T04:00:00Z","notebook":{"id":"0199a2b4-0000-7000-8000-000000000001","name":"笔记"},` +
		`"root":{"id":"0199a2b4-0000-7000-8000-000000000002","name":"项目A"},"nodes":[` +
		`{"path":"项目A/","kind":"page","id":"0199a2b4-0000-7000-8000-000000000002","sort_order":1.5},` +
		`{"path":"项目A/需求.md","kind":"page","id":"0199a2b4-0000-7000-8000-000000000003","sort_order":3}],` +
		`"contributed":[]}`
	if string(got) != want {
		t.Errorf("a subtree's meta.json =\n%s\nwant\n%s", got, want)
	}
}
