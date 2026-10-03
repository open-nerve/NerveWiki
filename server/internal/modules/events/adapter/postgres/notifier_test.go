package postgresadapter_test

import (
	"context"
	"errors"
	"testing"
	"time"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/events/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The domain's payload limit is NOTIFY's.
func TestThePayloadLimitIsNotifys(t *testing.T) {
	if domain.MaxPayload != postgres.MaxNotifyPayload {
		t.Errorf("domain.MaxPayload = %d, postgres.MaxNotifyPayload = %d; want the same", domain.MaxPayload, postgres.MaxNotifyPayload)
	}
}

// The notifier sends on Channel when its transaction commits, and refuses
// outside one.
func TestTheNotifierSendsOnTheChannel(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewEmptyDatabase(t), MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+postgresadapter.Channel); err != nil {
		t.Fatal(err)
	}

	if err := (postgresadapter.Notifier{}).Notify(ctx, "out"); !errors.Is(err, postgres.ErrNotifyOutsideTx) {
		t.Errorf("Notify() outside a transaction = %v, want ErrNotifyOutsideTx", err)
	}
	err = postgres.NewTxManager(pool, 2*time.Second).WithinTx(ctx, func(ctx context.Context) error {
		return postgresadapter.Notifier{}.Notify(ctx, "in")
	})
	wait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	n, werr := conn.Conn().WaitForNotification(wait)
	if err != nil || werr != nil || n.Channel != "nwiki_events" || n.Payload != "in" {
		t.Errorf("Notify() = %v; heard %+v, %v; want in on nwiki_events", err, n, werr)
	}
}
