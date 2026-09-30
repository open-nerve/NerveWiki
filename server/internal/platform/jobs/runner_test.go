package jobs

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// fakeClient records the contexts that Start and Stop get, and whether the
// context Start got was still live when Stop began. Start waits for its
// context when hang is set, as River's does on a SELECT 1 that the database
// never answers.
type fakeClient struct {
	hang bool

	mu         sync.Mutex
	started    context.Context
	stops      int
	liveAtStop bool
}

func (f *fakeClient) Start(ctx context.Context) error {
	f.mu.Lock()
	f.started = ctx
	f.mu.Unlock()
	if f.hang {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func (f *fakeClient) Stop(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	f.liveAtStop = f.started.Err() == nil
	return nil
}

func quiet() Config {
	return Config{ShutdownTimeout: time.Second, Logger: slog.New(slog.DiscardHandler)}
}

// Once started, the client runs on whatever happens to the caller's context,
// and Stop stops it with the client's Stop alone: River then cancels its own
// contexts with its stop cause, on which its services clean up. The context
// the client started with is released after the client's Stop.
func TestStopLeavesTheStartedClientToItsOwnStop(t *testing.T) {
	fake := &fakeClient{}
	r := newRunner(fake, quiet())
	ctx, cancel := context.WithCancel(context.Background())
	if err := r.Start(ctx); err != nil {
		t.Fatal(err)
	}
	cancel() // the server's context ends at shutdown, before the jobs stop

	time.Sleep(50 * time.Millisecond) // a context.AfterFunc still watching ctx would have fired by now
	if err := fake.started.Err(); err != nil {
		t.Fatalf("the client's context ended with the caller's: %v", err)
	}
	if err := r.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.stops != 1 || !fake.liveAtStop {
		t.Errorf("client stopped %d times, its context live at Stop: %v; want once, live", fake.stops, fake.liveAtStop)
	}
	if fake.started.Err() == nil {
		t.Error("the context the client started with is still live after Stop: it is never released")
	}
}

// Cancelling the caller's context while the client starts ends the start,
// so that a database that never answers cannot hold the shutdown; nothing
// started, so Stop has nothing to stop.
func TestAStartUnderWayEndsWithItsContext(t *testing.T) {
	fake := &fakeClient{hang: true}
	r := newRunner(fake, quiet())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Start(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()

	if err := receive(t, done, time.Second, "return from Start"); err == nil {
		t.Error("Start() = nil after its context ended, want the error")
	}
	if err := r.Stop(context.Background()); err != nil || fake.stops != 0 {
		t.Errorf("Stop() = %v after %d client stops, want nil and none", err, fake.stops)
	}
}
