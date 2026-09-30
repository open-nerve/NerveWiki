package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

func TestNewJSON(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, config.LogConfig{Level: "info", Format: "json"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.Debug("hidden")
	logger.Info("shown", "key", "value")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("want exactly one JSON line, got %q: %v", buf.String(), err)
	}
	if entry["msg"] != "shown" || entry["level"] != "INFO" || entry["key"] != "value" {
		t.Errorf("entry = %v", entry)
	}
}

func TestNewText(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, config.LogConfig{Level: "debug", Format: "text"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.Debug("shown", "key", "value")

	if out := buf.String(); !strings.Contains(out, "level=DEBUG msg=shown key=value") {
		t.Errorf("output = %q", out)
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	for _, cfg := range []config.LogConfig{
		{Level: "verbose", Format: "json"},
		{Level: "info", Format: "xml"},
	} {
		if _, err := New(&bytes.Buffer{}, cfg); err == nil {
			t.Errorf("New(%+v) error = nil, want an error", cfg)
		}
	}
}
