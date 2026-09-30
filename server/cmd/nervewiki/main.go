// Command nervewiki runs the Nerve Wiki server and its operational commands.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signalContext()
	code := run(ctx, os.Args[1:], os.Environ(), os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// signalContext is cancelled by the first SIGINT or SIGTERM, which starts
// the graceful shutdown. The default handling comes back before the context
// is cancelled, so a second signal, however soon, stops the process at once.
func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-signals:
		case <-ctx.Done():
		}
		signal.Stop(signals)
		cancel()
	}()
	return ctx, cancel
}

// run executes one command line and returns the process exit code. A
// password comes from stdin; results go to stdout; logs, prompts and errors
// go to stderr.
func run(ctx context.Context, args, environ []string, stdin io.Reader, stdout, stderr io.Writer) int {
	root := newRootCommand(environ, stdin)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "nervewiki: %v\n", err)
		return 1
	}
	return 0
}
