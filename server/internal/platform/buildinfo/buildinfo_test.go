package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestFromSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings []debug.BuildSetting
		want     Info
	}{
		{
			name:     "no vcs settings keeps unknown defaults",
			settings: nil,
			want:     Info{Version: "1.2.3", Commit: "unknown", CommitTime: "unknown"},
		},
		{
			name: "vcs settings are applied and other settings ignored",
			settings: []debug.BuildSetting{
				{Key: "GOOS", Value: "linux"},
				{Key: "vcs.revision", Value: "abc123"},
				{Key: "vcs.time", Value: "2026-09-22T10:00:00Z"},
				{Key: "vcs.modified", Value: "true"},
			},
			want: Info{Version: "1.2.3", Commit: "abc123", CommitTime: "2026-09-22T10:00:00Z", Modified: true},
		},
		{
			name:     "clean working tree is not modified",
			settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "false"}},
			want:     Info{Version: "1.2.3", Commit: "unknown", CommitTime: "unknown"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fromSettings("1.2.3", tt.settings); got != tt.want {
				t.Errorf("fromSettings() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetReportsStampedVersion(t *testing.T) {
	if got := Get().Version; got != version {
		t.Errorf("Get().Version = %q, want %q", got, version)
	}
}
