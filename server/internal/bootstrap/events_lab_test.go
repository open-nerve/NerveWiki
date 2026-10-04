package bootstrap

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// Who receives lab's events (M5 design 4.10): a pages event of each seeded
// notebook, published as the page module's observer publishes one, and an
// event of a type a later M adds (M5 design 8), published as another
// module will publish M6's links, reach the stream of each notebook column
// that reads the notebook (GET answers 200), and no other, through the
// whole program. An access event of every column, published last in the
// same transaction, ends each stream: what came before it is all the
// stream received.
func TestLabsEventsReachTheColumnsThatReadThem(t *testing.T) {
	ctx := context.Background()
	d := prepareMatrix(t)
	contract := apitest.Load(t)
	url := pgtest.NewDatabaseFrom(t, d.url)
	s := d.seeded.in(t)
	base := startApp(t, d.config(t, url, nil), migrations.FS())
	read := map[caller]map[uuid.UUID]bool{}
	streams := map[caller]*eventStream{}
	var users []uuid.UUID
	for _, c := range notebookColumns() {
		read[c] = map[uuid.UUID]bool{}
		for _, n := range matrixNotebooks() {
			id := s.notebook(n.name)
			switch status, answer := ask(t, contract, http.MethodGet, base+"/api/v0/notebooks/"+id.String(), d.tokens[c], ""); status {
			case http.StatusOK:
				read[c][id] = true
			case http.StatusNotFound:
			default:
				t.Fatalf("%s: GET %s = %d %s", c, n.name, status, answer)
			}
		}
		streams[c] = openStream(t, base, string(c), d.tokens[c])
		users = append(users, s.accounts[c])
	}

	publisher := events.NewPublisher()
	err := postgres.NewTxManager(connect(t, url), time.Second).WithinTx(ctx, func(ctx context.Context) error {
		for _, n := range matrixNotebooks() {
			if err := publisher.PagesWritten(ctx, events.PagesWritten{WorkspaceID: s.workspaces[n.slug], NotebookID: s.notebook(n.name),
				Changes: []events.PageChange{{PageID: uuid.New(), Tree: true, Revision: 1}}}); err != nil {
				return err
			}
			links := events.Event{Type: "links", WorkspaceID: s.workspaces[n.slug], NotebookID: s.notebook(n.name),
				Data: json.RawMessage(`{"page_id":"` + uuid.New().String() + `"}`)}
			if err := publisher.Publish(ctx, links); err != nil {
				return err
			}
		}
		return publisher.AccessChanged(ctx, events.AccessChanged{WorkspaceID: s.workspaces["lab"], UserIDs: users})
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range notebookColumns() {
		got := map[string]map[uuid.UUID]bool{"pages": {}, "links": {}}
		f := streams[c].next(t)
		for ; f.event == "pages" || f.event == "links"; f = streams[c].next(t) {
			var p struct {
				NotebookID uuid.UUID `json:"notebook_id"`
			}
			if err := json.Unmarshal([]byte(f.data), &p); err != nil {
				t.Fatalf("%s: %s %s: %v", c, f.event, f.data, err)
			}
			got[f.event][p.NotebookID] = true
		}
		if f.event != "reset" || f.field(t, "reason") != "access" {
			t.Fatalf("%s: %s %s after the pages events, want the reset of the access event", c, f.event, f.data)
		}
		for event, notebooks := range got {
			if !maps.Equal(notebooks, read[c]) {
				t.Errorf("%s: the stream received the %s events of %v, reads %v; want the same notebooks", c, event, notebooks, read[c])
			}
		}
	}
}
