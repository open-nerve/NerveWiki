package bootstrap

import (
	"context"
	"net/http"
	"net/netip"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// Behind a trusted proxy, a sign-in's session records the client the proxy
// forwarded, not the proxy (M1/P1 design 3.5 through to M1/P2's sessions).
func TestASignInBehindATrustedProxyRecordsTheClient(t *testing.T) {
	contract := apitest.Load(t)
	dbURL := pgtest.NewDatabase(t)
	cfg := testConfig(t, dbURL, true)
	cfg.Server.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}
	base := startApp(t, cfg, migrations.FS())
	registerAccount(t, contract, base, "alice@example.com")

	req := newRequest(t, http.MethodPost, base+"/api/v0/auth/login", "", []byte(`{"email":"alice@example.com","password":"Tr0ub4dor&3"}`))
	req.Header.Set("X-Forwarded-For", "198.51.100.23")
	res, body := sendRequest(t, req)
	contract.CheckResponse(t, req, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login = %d %s, want 200", res.StatusCode, body)
	}

	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	var ips []string
	rows, _ := conn.Query(context.Background(), `SELECT host(ip) FROM auth_sessions ORDER BY created_at, id`)
	ips, err = pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	// The registration came without X-Forwarded-For: the proxy itself.
	if len(ips) != 2 || ips[0] != "127.0.0.1" || ips[1] != "198.51.100.23" {
		t.Errorf("session IPs = %q, want [127.0.0.1 198.51.100.23]", ips)
	}
}
