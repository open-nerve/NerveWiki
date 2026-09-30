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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// After the first signal starts the graceful shutdown, restore the
	// default handling so a second signal stops the process at once.
	context.AfterFunc(ctx, stop)
	code := run(ctx, os.Args[1:], os.Environ(), os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run executes one command line and returns the process exit code. Results
// go to stdout; logs and errors go to stderr.
func run(ctx context.Context, args, environ []string, stdout, stderr io.Writer) int {
	root := newRootCommand(environ)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "nervewiki: %v\n", err)
		return 1
	}
	return 0
}
