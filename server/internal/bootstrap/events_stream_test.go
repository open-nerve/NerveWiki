package bootstrap

import (
	"bufio"
	"context"
	"encoding/json"
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
}

// openStream opens name's event stream on base with token, past its hello,
// closed when the test ends.
func openStream(t *testing.T, base, name, token string) *eventStream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v0/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	late := time.AfterFunc(interleavingWait, cancel)
	res, err := http.DefaultClient.Do(req)
	late.Stop()
	if err != nil {
		t.Fatalf("%s's stream: %v", name, err)
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		t.Fatalf("%s's stream = %d", name, res.StatusCode)
	}
	s := &eventStream{name: name, frames: make(chan streamFrame, 256)}
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
