package domain_test

import (
	"reflect"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The keys are taken anew from the names: the nodes whose key changes come
// back with the new one; siblings that would share one change nothing,
// each group of them told, and nodes under different parents may share.
func TestRekeyTakesTheKeysAnew(t *testing.T) {
	a, b := uuid.NewV7(), uuid.NewV7()
	node := func(parent *uuid.UUID, name, key string) domain.Node {
		return domain.Node{ID: uuid.NewV7(), ParentID: parent, Name: name, NameKey: key}
	}
	stale := node(nil, "Straße", "straße")
	fresh := node(&a, "Straße", "strasse")
	other := node(&b, "STRASSE", "old")
	changed, clashes := domain.Rekey([]domain.Node{stale, fresh, other})
	stale.NameKey, other.NameKey = "strasse", "strasse"
	if !reflect.DeepEqual(changed, []domain.Node{stale, other}) || clashes != nil {
		t.Errorf("Rekey = %+v, %+v; want the stale nodes rekeyed", changed, clashes)
	}

	x, y, z := node(&a, "Straße", "s1"), node(&a, "STRASSE", "s2"), node(&a, "Other", "other")
	changed, clashes = domain.Rekey([]domain.Node{x, z, y})
	if changed != nil || !reflect.DeepEqual(clashes, [][]domain.Node{{x, y}}) {
		t.Errorf("Rekey of a clash = %+v, %+v; want nothing changed and the clash", changed, clashes)
	}
}
