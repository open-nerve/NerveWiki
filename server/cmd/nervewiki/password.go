package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// terminal is what readPassword uses of a terminal; the tests replace it.
type terminal interface {
	IsTerminal(fd int) bool
	GetState(fd int) (*term.State, error)
	ReadPassword(fd int) ([]byte, error)
	Restore(fd int, state *term.State) error
}

// realTerminal is the process's terminal, through golang.org/x/term.
type realTerminal struct{}

func (realTerminal) IsTerminal(fd int) bool                  { return term.IsTerminal(fd) }
func (realTerminal) GetState(fd int) (*term.State, error)    { return term.GetState(fd) }
func (realTerminal) ReadPassword(fd int) ([]byte, error)     { return term.ReadPassword(fd) }
func (realTerminal) Restore(fd int, state *term.State) error { return term.Restore(fd, state) }

// readPassword reads the new password (M1/P4 design 3.7), never from an
// argument or the environment, which ps and /proc show. On a terminal it
// asks twice without echo, prompting on prompt, and the two must match;
// otherwise it reads one line, for scripts and tests. Only the line ending,
// \n or \r\n, is removed: spaces and a lone \r are part of the password.
func readPassword(ctx context.Context, in io.Reader, prompt io.Writer, tty terminal) (string, error) {
	if f, ok := in.(*os.File); ok && tty.IsTerminal(int(f.Fd())) {
		return promptTwice(ctx, int(f.Fd()), prompt, tty)
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", fmt.Errorf("read the password from standard input: %w", err)
	}
	if l, ok := strings.CutSuffix(line, "\n"); ok {
		line = strings.TrimSuffix(l, "\r")
	}
	return line, nil
}

func promptTwice(ctx context.Context, fd int, prompt io.Writer, tty terminal) (string, error) {
	state, err := tty.GetState(fd)
	if err != nil {
		return "", fmt.Errorf("read the terminal's state: %w", err)
	}
	first, err := promptPassword(ctx, fd, state, prompt, tty, "Password: ")
	if err != nil {
		return "", err
	}
	again, err := promptPassword(ctx, fd, state, prompt, tty, "Password again: ")
	if err != nil {
		return "", err
	}
	if !bytes.Equal(first, again) {
		return "", errors.New("the passwords do not match")
	}
	return string(first), nil
}

// promptPassword asks for the password once. The echo is off while
// ReadPassword waits, and the first SIGINT cancels ctx rather than the
// process (signalContext), so ReadPassword waits on: the read runs aside,
// and when ctx is done the terminal is restored from state before the
// command returns, as the second SIGINT, which kills the process, could not.
// The abandoned read ends with the process.
func promptPassword(ctx context.Context, fd int, state *term.State, prompt io.Writer, tty terminal, label string) ([]byte, error) {
	type read struct {
		password []byte
		err      error
	}
	done := make(chan read, 1)
	_, _ = io.WriteString(prompt, label)
	go func() {
		password, err := tty.ReadPassword(fd)
		done <- read{password, err}
	}()
	var r read
	select {
	case r = <-done:
	case <-ctx.Done():
		r.err = errors.Join(context.Cause(ctx), tty.Restore(fd, state))
	}
	_, _ = io.WriteString(prompt, "\n")
	if r.err != nil {
		return nil, fmt.Errorf("read the password: %w", r.err)
	}
	return r.password, nil
}
