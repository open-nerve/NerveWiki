package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"golang.org/x/term"
)

// pipe is the read end of a pipe holding input: a file that is not a
// terminal.
func pipe(t *testing.T, input string) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	return r
}

// A file that is not a terminal, such as a pipe from a script, is read as
// one line, without a prompt.
func TestReadPasswordFromAPipe(t *testing.T) {
	var prompt strings.Builder

	got, err := readPassword(context.Background(), pipe(t, "Tr0ub4dor&3\n"), &prompt, realTerminal{})

	if err != nil || got != "Tr0ub4dor&3" || prompt.Len() != 0 {
		t.Errorf("readPassword(pipe) = %q, %v, prompting %q; want the line and no prompt", got, err, prompt.String())
	}
}

// fakeTerminal is a terminal on which the administrator types answers, one
// per ReadPassword; once they run out, ReadPassword calls out of answers
// and then waits for the test's end, as a terminal waits for a key.
type fakeTerminal struct {
	answers    []string
	outOfInput func()
	state      *term.State
	restored   []*term.State
	ended      chan struct{}
}

func newFakeTerminal(t *testing.T, answers ...string) *fakeTerminal {
	f := &fakeTerminal{answers: answers, outOfInput: func() {}, state: &term.State{}, ended: make(chan struct{})}
	t.Cleanup(func() { close(f.ended) })
	return f
}

func (f *fakeTerminal) IsTerminal(int) bool               { return true }
func (f *fakeTerminal) GetState(int) (*term.State, error) { return f.state, nil }

func (f *fakeTerminal) ReadPassword(int) ([]byte, error) {
	if len(f.answers) == 0 {
		f.outOfInput()
		<-f.ended
		return nil, errors.New("the test ended")
	}
	answer := f.answers[0]
	f.answers = f.answers[1:]
	return []byte(answer), nil
}

func (f *fakeTerminal) Restore(_ int, state *term.State) error {
	f.restored = append(f.restored, state)
	return nil
}

// On a terminal the password is asked twice, the prompts on stderr, and
// what was typed is the password, spaces and all.
func TestReadPasswordOnATerminal(t *testing.T) {
	tty := newFakeTerminal(t, " Tr0ub4dor&3 ", " Tr0ub4dor&3 ")
	var prompt strings.Builder

	got, err := readPassword(context.Background(), pipe(t, ""), &prompt, tty)

	if err != nil || got != " Tr0ub4dor&3 " || prompt.String() != "Password: \nPassword again: \n" || len(tty.restored) != 0 {
		t.Errorf("readPassword() = %q, %v, prompting %q, restoring %d times; want the password asked twice",
			got, err, prompt.String(), len(tty.restored))
	}
}

func TestReadPasswordOnATerminalFailsWhenTheTwoDiffer(t *testing.T) {
	tty := newFakeTerminal(t, "Tr0ub4dor&3", "Tr0ub4dor&4")

	got, err := readPassword(context.Background(), pipe(t, ""), &strings.Builder{}, tty)

	if err == nil || err.Error() != "the passwords do not match" || got != "" {
		t.Errorf("readPassword() = %q, %v; want the passwords do not match", got, err)
	}
}

// The first Ctrl-C cancels the command's context rather than the process,
// while the echo is off: the read gives up, and the terminal is restored to
// the state it had before the prompt, at the first prompt and at the second.
func TestReadPasswordOnATerminalRestoresItWhenCancelled(t *testing.T) {
	for _, typed := range [][]string{nil, {"Tr0ub4dor&3"}} {
		ctx, cancel := context.WithCancel(context.Background())
		tty := newFakeTerminal(t, typed...)
		tty.outOfInput = cancel
		var prompt strings.Builder

		got, err := readPassword(ctx, pipe(t, ""), &prompt, tty)

		if !errors.Is(err, context.Canceled) || got != "" || len(tty.restored) != 1 || tty.restored[0] != tty.state {
			t.Errorf("after %d answers, readPassword() = %q, %v, restoring %v; want context.Canceled and the terminal restored once to its state",
				len(typed), got, err, tty.restored)
		}
		if !strings.HasSuffix(prompt.String(), ": \n") {
			t.Errorf("prompting %q, want the line ended", prompt.String())
		}
	}
}
