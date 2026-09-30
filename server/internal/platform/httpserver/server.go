package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// idleTimeout closes keep-alive connections that stay idle this long, so idle
// clients cannot hold connections open indefinitely.
const idleTimeout = 2 * time.Minute

// Server is the HTTP server of a nervewiki process.
type Server struct {
	srv             *http.Server
	logger          *slog.Logger
	shutdownTimeout time.Duration
	addrFile        string
}

// NewServer serves h behind the platform middleware chain (request ID ->
// recover -> access log -> security headers). Reading and writing on a
// connection are bounded: request headers (server.read_header_timeout), the
// whole request with its body (server.read_timeout), the response
// (server.write_timeout) and idle keep-alive (idleTimeout). A route that
// holds its response open, such as an event stream, lifts them with
// LongLived; any other handler that legitimately needs longer, such as a
// file upload, extends its own deadlines with http.ResponseController
// instead of raising them for every request. A handler's own run time is not
// bounded: write_timeout fails its writes but neither stops it nor cancels
// its context, so its blocking calls need their own deadlines.
func NewServer(cfg config.ServerConfig, h http.Handler, logger *slog.Logger) *Server {
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           middleware(h, logger),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
	// Every request carries the signal that shutdown has begun, for
	// LongLived: Shutdown waits for connections to go idle, which a
	// long-lived response never does unless it is told to end.
	stopping, stop := context.WithCancel(context.Background())
	srv.BaseContext = func(net.Listener) context.Context { return withStopping(context.Background(), stopping) }
	srv.RegisterOnShutdown(stop)
	return &Server{
		srv:             srv,
		logger:          logger,
		shutdownTimeout: cfg.ShutdownTimeout,
		addrFile:        cfg.AddrFile,
	}
}

// ListenAndServe listens on server.addr, writes the address it got to
// server.addr_file when that is set (the end-to-end tests listen on port 0),
// and then behaves like Serve.
func (s *Server) ListenAndServe(ctx context.Context) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.srv.Addr, err)
	}
	if s.addrFile != "" {
		if err := writeAddrFile(s.addrFile, ln.Addr().String()); err != nil {
			_ = ln.Close()
			return err
		}
	}
	return s.Serve(ctx, ln)
}

// writeAddrFile writes addr to path through a temporary file and a rename,
// so a reader never sees a partial address. The error leaves the path out:
// like every *_file key, server.addr_file is not logged.
func writeAddrFile(path, addr string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(addr), 0o600); err != nil {
		return fmt.Errorf("server.addr_file: write: %w", errors.Unwrap(err))
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("server.addr_file: rename: %w", errors.Unwrap(err))
	}
	return nil
}

// Serve serves HTTP on ln until ctx is done. It then stops accepting
// connections, tells long-lived routes to end, and gives in-flight requests
// up to server.shutdown_timeout to finish; connections still open after that
// are closed and an error is returned. A clean shutdown returns nil.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	s.logger.InfoContext(ctx, "http server listening", slog.String("addr", ln.Addr().String()))
	served := make(chan error, 1)
	go func() { served <- s.srv.Serve(ln) }()

	select {
	case err := <-served:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	s.logger.InfoContext(ctx, "http server shutting down", slog.Duration("timeout", s.shutdownTimeout))
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.shutdownTimeout)
	defer cancel()
	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		closeErr := s.srv.Close()
		<-served
		return fmt.Errorf("http server shutdown: %w", errors.Join(err, closeErr))
	}
	<-served // http.ErrServerClosed once Shutdown has begun
	s.logger.InfoContext(ctx, "http server stopped")
	return nil
}
