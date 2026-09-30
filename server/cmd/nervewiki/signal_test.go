//go:build unix

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// signalChildEnv makes the test binary act as a process whose graceful
// shutdown never ends: it reports the first signal, then hangs.
const signalChildEnv = "NWIKI_TEST_SIGNAL_CHILD"

func TestMain(m *testing.M) {
	if os.Getenv(signalChildEnv) == "1" {
		ctx, _ := signalContext()
		fmt.Println("ready")
		<-ctx.Done()
		fmt.Println("shutting down")
		select {} // a shutdown that hangs
	}
	os.Exit(m.Run())
}

// The first SIGINT starts the graceful shutdown; when that hangs, a second
// SIGINT stops the process at once.
func TestSecondSignalStopsTheProcess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), signalChildEnv+"=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	lines := bufio.NewScanner(stdout)
	expect := func(want string) {
		t.Helper()
		if !lines.Scan() || lines.Text() != want {
			t.Fatalf("child printed %q (%v), want %q", lines.Text(), lines.Err(), want)
		}
	}

	expect("ready")
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	expect("shutting down")
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the process still runs 5s after the second SIGINT")
	}
	status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGINT {
		t.Errorf("process ended with %v, want it killed by the second SIGINT", cmd.ProcessState)
	}
}
