package httpadapter_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
)

// Every problem code the module's operations declare must be answered by a
// test here (v0.1 design 13.1, item 8).
func TestMain(m *testing.M) { apitest.Main(m) }
