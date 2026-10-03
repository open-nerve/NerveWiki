package httpadapter_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
)

// Every problem code the module's operation declares must be answered by a
// test here (v0.1 design 6.1).
func TestMain(m *testing.M) { apitest.Main(m) }
