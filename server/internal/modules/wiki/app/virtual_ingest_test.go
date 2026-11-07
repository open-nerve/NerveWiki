package app_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
)

func testVirtualIngestLog(t *testing.T) {
	for _, profile := range []struct {
		name    string
		value   app.FileProfile
		limit   int
		sources []string
	}{
		{"read", app.ReadFile, 50, []string{
			"  - source: Missing source",
			"  - source: [raw/Z%41.md](<raw/Z%2541.md>)",
			"  - source: Missing source",
			"  - source: [raw/A.png](<raw/A.png>)",
			"  - source: [raw/M.md](<raw/M.md>)",
		}},
		{"export", app.ExportFile, 1000, []string{
			"  - source: Missing source",
			"  - source: [collision%41.md](<collision%2541.md>)",
			"  - source: Missing source",
			"  - source: [images/A.png](<images/A.png>)",
			"  - source: [Moved.md](<Moved.md>)",
		}},
	} {
		t.Run(profile.name, func(t *testing.T) {
			root, second, fourth, fifth := uuid.UUID{15: 1}, uuid.UUID{15: 90}, uuid.UUID{15: 10}, uuid.UUID{15: 50}
			f := &fileReads{
				nodes: []app.VirtualTreeNode{{ID: root, Name: "raw"}, {ID: second, Name: "Z%41", ParentID: &root},
					{ID: fourth, Name: "A.png", ParentID: &root, Asset: true}, {ID: fifth, Name: "M", ParentID: &root}},
				log: []app.VirtualLogChangeset{{ID: uuid.UUID{15: 2}, Kind: "edit", IngestCount: 25,
					IngestSources: []app.VirtualLogSource{{}, {NodeID: &second, Path: new("raw/Z%41.md")}, {},
						{NodeID: &fourth, Path: new("raw/A.png"), Asset: true}, {NodeID: &fifth, Path: new("raw/M.md")}}}},
			}
			scope := app.VirtualScope{NotebookID: uuid.UUID{15: 3}, SourceRootID: root}
			if profile.value == app.ExportFile {
				// A missing display path does not erase a live source's identity.
				for i := range f.log[0].IngestSources {
					f.log[0].IngestSources[i].Path = nil
				}
				scope.Export = &app.VirtualExportScope{Paths: []app.ExportPath{{ID: root, Path: "raw/"},
					{ID: second, Path: "collision%41.md"}, {ID: fourth, Path: "images/A.png", Asset: true}, {ID: fifth, Path: "Moved.md"}}}
			}
			v := app.NewVirtualFiles(app.VirtualFileDeps{Index: f, Tree: f, Sources: fileSources{}, Log: f})
			type contextKey struct{}
			ctx := context.WithValue(context.Background(), contextKey{}, "snapshot")
			body, err := v.GenerateInSnapshot(ctx, scope, app.LogFile, profile.value)
			if err != nil {
				t.Fatal(err)
			}
			var sources []string
			for _, line := range strings.Split(string(body), "\n") {
				if strings.HasPrefix(line, "  - source: ") {
					sources = append(sources, line)
				}
			}
			if !reflect.DeepEqual(sources, profile.sources) || !strings.Contains(string(body), "; ingests: 25\n") ||
				!strings.HasSuffix(string(body), "\nShowing 1 of the most recent 1 changesets. Use log to filter and page through more.\n") {
				t.Fatalf("ordered nullable source/count/identity consumption: %q", body)
			}
			if f.logCalls != 1 || f.logLimit != profile.limit || f.logContext != ctx || f.inboundCalls != 0 ||
				(profile.value == app.ReadFile && f.treeCalls != 0) || (profile.value == app.ExportFile && f.treeCalls != 1) {
				t.Fatalf("profile changed reads: log=%d limit=%d tree=%d inbound=%d", f.logCalls, f.logLimit, f.treeCalls, f.inboundCalls)
			}
			t.Run("error", func(t *testing.T) {
				f.logError = errors.New("source preview failed")
				f.logCalls = 0
				got, err := v.GenerateInSnapshot(ctx, scope, app.LogFile, profile.value)
				if got != nil || err != f.logError || f.logCalls != 1 || f.logLimit != profile.limit || f.logContext != ctx { //nolint:errorlint // Preserve the exact source preview error object through GenerateInSnapshot.
					t.Fatalf("source error was replaced or swallowed: %q, %v", got, err)
				}
			})
		})
	}
}
