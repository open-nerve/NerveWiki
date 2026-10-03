package bootstrap

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// streamFrame is one frame of an event stream: its event and data.
type streamFrame struct {
	event, data string
}

// field is the string field name of the frame's data, "" for none.
func (f streamFrame) field(t *testing.T, name string) string {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal([]byte(f.data), &fields); err != nil {
		t.Fatalf("frame %s: data %q: %v", f.event, f.data, err)
	}
	s, _ := fields[name].(string)
	return s
}

// eventStream is an event stream of the whole program, read in the
// background frame by frame, comments left out.
type eventStream struct {
	name   string
	frames chan streamFrame
	close  context.CancelFunc
}

// openStream opens name's event stream on base with token, past its hello,
// closed when the test ends. A 503 not_ready, while the server's listener
// connects, is asked again.
func openStream(t *testing.T, base, name, token string) *eventStream {
	t.Helper()
	res, cancel := streamResponse(t, base, name, token)
	s := &eventStream{name: name, frames: make(chan streamFrame, 256), close: cancel}
	go func() {
		defer close(s.frames)
		defer func() { _ = res.Body.Close() }()
		r := bufio.NewReader(res.Body)
		var f streamFrame
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			switch line = strings.TrimSuffix(line, "\n"); {
			case strings.HasPrefix(line, "event: "):
				f.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				f.data = strings.TrimPrefix(line, "data: ")
			case line == "" && f.event != "":
				s.frames <- f
				f = streamFrame{}
			}
		}
	}()
	if f := s.next(t); f.event != "hello" {
		t.Fatalf("%s's stream began with %s %s, want hello", name, f.event, f.data)
	}
	return s
}

// streamResponse is the 200 of name's stream on base, asked again while it
// is 503 not_ready, interleavingWait at most, and what closes it, which the
// test's end does too.
func streamResponse(t *testing.T, base, name, token string) (*http.Response, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	late := time.AfterFunc(interleavingWait, cancel)
	defer late.Stop()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v0/events", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s's stream: %v", name, err)
		}
		if res.StatusCode == http.StatusOK {
			return res, cancel
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("%s's stream = %d", name, res.StatusCode)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// settle waits until the notifications of the writes committed so far have
// reached the hub, so that a stream opened after it gets none of them: one
// arriving late would be taken for the frame a test awaits. PostgreSQL
// delivers notifications in commit order, so once alice's stream gets the
// frame of a write of the marker of nb made now, every earlier one has
// been dispatched. A stream of alice that an earlier write resets is
// opened again.
func (tm acmeTeam) settle(t *testing.T, nb, marker string) {
	t.Helper()
	for {
		s := openStream(t, tm.base, "alice", tm.tokens["alice"])
		rev := tm.content(t, "alice", marker).Revision
		tm.send(t, contentWrite("alice", marker, fmt.Sprintf("# Marker %d", rev+1), rev, ""), http.StatusOK)
		want := fmt.Sprint(map[string]int{marker: rev + 1})
		for f := s.next(t); f.event != "EOF"; f = s.next(t) {
			if f.event != "pages" {
				continue
			}
			var p pagesData
			if err := json.Unmarshal([]byte(f.data), &p); err != nil {
				t.Fatal(err)
			}
			if p.NotebookID == nb && fmt.Sprint(p.written()) == want {
				s.close()
				return
			}
		}
		s.close()
	}
}

// next returns the stream's next frame; an ended stream gives "EOF".
func (s *eventStream) next(t *testing.T) streamFrame {
	t.Helper()
	select {
	case f, ok := <-s.frames:
		if !ok {
			return streamFrame{event: "EOF"}
		}
		return f
	case <-time.After(interleavingWait):
		t.Fatalf("%s's stream: no frame within %v", s.name, interleavingWait)
		return streamFrame{}
	}
}

// expect fails t unless the next frame is event, and returns it.
func (s *eventStream) expect(t *testing.T, event string) streamFrame {
	t.Helper()
	f := s.next(t)
	if f.event != event {
		t.Fatalf("%s's stream: %s %s, want %s", s.name, f.event, f.data, event)
	}
	return f
}

// reset fails t unless the next frame is a reset for reason, then the end.
func (s *eventStream) reset(t *testing.T, reason string) {
	t.Helper()
	if f := s.expect(t, "reset"); f.field(t, "reason") != reason {
		t.Fatalf("%s's stream: reset %s, want %s", s.name, f.data, reason)
	}
	if f := s.next(t); f.event != "EOF" {
		t.Fatalf("%s's stream: %s %s after the reset, want its end", s.name, f.event, f.data)
	}
}

// The listener's reconnection (M5 design 4.11): its connection cut, the
// listener listens again and resets every open stream, whose client reads
// again what it may have missed; a stream opened after it receives the
// events.
func TestAReconnectedListenerResetsTheStreams(t *testing.T) {
	tm, nb, marker, bob := eventsTeam(t)
	alice := openStream(t, tm.base, "alice", tm.tokens["alice"])
	if n := pgtest.TerminateListeners(t, tm.pool, events.Channel, interleavingWait); n != 1 {
		t.Fatalf("%d listeners cut, want the app's one", n)
	}
	bob.reset(t, "reconnected")
	alice.reset(t, "reconnected")
	tm.quiet(t, nb, marker, openStream(t, tm.base, "bob", tm.tokens["bob"]))
}
