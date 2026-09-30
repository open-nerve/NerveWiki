// Package logging builds nervewiki's structured logger.
package logging

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// New returns a logger that writes to w with the configured level and
// format. It never touches the slog default logger.
func New(w io.Writer, cfg config.LogConfig) (*slog.Logger, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, fmt.Errorf("log level: %w", err)
	}
	opts := &slog.HandlerOptions{Level: level}
	switch cfg.Format {
	case "json":
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	case "text":
		return slog.New(slog.NewTextHandler(w, opts)), nil
	default:
		return nil, fmt.Errorf("log format %q: must be text or json", cfg.Format)
	}
}
