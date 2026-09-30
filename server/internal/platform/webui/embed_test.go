package webui

import (
	"io/fs"
	"testing"
)

func TestFSIsTheDistDirectory(t *testing.T) {
	if _, err := fs.Stat(FS(), ".gitkeep"); err != nil {
		t.Errorf("FS() lacks .gitkeep, so it is not dist/: %v", err)
	}
}
