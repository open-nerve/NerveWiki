package wiki

import "github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"

type (
	VirtualFiles        = app.VirtualFiles
	VirtualFileDeps     = app.VirtualFileDeps
	VirtualScope        = app.VirtualScope
	VirtualExportScope  = app.VirtualExportScope
	ExportPath          = app.ExportPath
	FileName            = app.FileName
	FileProfile         = app.FileProfile
	FileIndexHead       = app.FileIndexHead
	FileIndexFacts      = app.FileIndexFacts
	IndexFileReads      = app.IndexFileReads
	VirtualTreeNode     = app.VirtualTreeNode
	VirtualTreeReads    = app.VirtualTreeReads
	VirtualSourceNode   = app.VirtualSourceNode
	VirtualSourceReads  = app.VirtualSourceReads
	VirtualLogChange    = app.VirtualLogChange
	VirtualLogSource    = app.VirtualLogSource
	VirtualLogChangeset = app.VirtualLogChangeset
	VirtualLogReads     = app.VirtualLogReads
)

const (
	IndexFile  = app.IndexFile
	LogFile    = app.LogFile
	ReadFile   = app.ReadFile
	ExportFile = app.ExportFile
)

func NewVirtualFiles(d VirtualFileDeps) *VirtualFiles { return app.NewVirtualFiles(d) }
