package identity_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock/clocktest"
)

// A token's client name is the last one recorded: "" before any, and for
// no such token; written when it differs alone, so that one client's calls
// write it once; updated_at stays, as a use changes nothing of the token
// (M9/P1 design 3.16.3).
func TestTheClientNamesRecordTheLastToldOnTheToken(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	register(newServerOn(t, pool, clocktest.At(testStart())), "alice@corp.com")
	_, id := insertToken(t, pool, nil)
	names := identity.NewClientNames(pool)
	updated := func() time.Time {
		t.Helper()
		var at time.Time
		if err := pool.QueryRow(ctx, `SELECT updated_at FROM api_tokens WHERE id = $1`, id).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at
	}
	before := updated()
	if name, err := names.Recorded(ctx, id); name != "" || err != nil {
		t.Errorf("Recorded before any = %q, %v; want none", name, err)
	}
	for _, tt := range []struct {
		name    string
		written bool
	}{{"claude-code", true}, {"claude-code", false}, {"codex", true}, {"名前", true}} {
		written, err := names.Record(ctx, id, tt.name)
		if err != nil || written != tt.written {
			t.Errorf("Record(%q) = %v, %v; want written %v", tt.name, written, err, tt.written)
		}
		if got, err := names.Recorded(ctx, id); got != tt.name || err != nil {
			t.Errorf("Recorded after %q = %q, %v", tt.name, got, err)
		}
	}
	if !updated().Equal(before) {
		t.Errorf("updated_at moved from %v to %v", before, updated())
	}
	gone := uuid.NewV7()
	if written, err := names.Record(ctx, gone, "codex"); written || err != nil {
		t.Errorf("Record of no token = %v, %v; want nothing written", written, err)
	}
	if name, err := names.Recorded(ctx, gone); name != "" || err != nil {
		t.Errorf("Recorded of no token = %q, %v", name, err)
	}
}

// A record skips a row that another transaction holds, a revocation, and
// says it wrote nothing; once the row is free, the next record writes.
func TestTheClientNamesSkipARowInUse(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	register(newServerOn(t, pool, clocktest.At(testStart())), "alice@corp.com")
	_, id := insertToken(t, pool, nil)
	names := identity.NewClientNames(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT 1 FROM api_tokens WHERE id = $1 FOR UPDATE", id); err != nil {
		t.Fatal(err)
	}
	wait, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if written, err := names.Record(wait, id, "claude-code"); written || err != nil {
		t.Fatalf("Record while the row is held = %v, %v; want nothing written, no wait", written, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if written, err := names.Record(ctx, id, "claude-code"); !written || err != nil {
		t.Errorf("Record once the row is free = %v, %v; want written", written, err)
	}
}
