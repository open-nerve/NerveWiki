package domain

import (
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// An import's entries (M7/P6 design 3.11): what the archive's directory
// lists, sorted before any byte is read into what the import reads, what
// it skips and why, and what it ignores.

// The zip methods an import reads: Store and Deflate.
const (
	MethodStore   uint16 = 0
	MethodDeflate uint16 = 8
)

// RawEntry is an entry as the archive's directory lists it: its index
// there, its name's bytes, whether it is a folder, a symbolic link or
// another file that is no regular one, encrypted, and its method.
type RawEntry struct {
	Index     int
	Name      string
	Folder    bool
	Special   bool
	Encrypted bool
	Method    uint16
}

// ImportEntry is an entry the import reads: its index in the archive, its
// path in the vault (its names, NFC, the vault's own folder dropped),
// whether it is a folder, and its name in the archive, which a problem
// tells.
type ImportEntry struct {
	Index  int
	Path   []string
	Folder bool
	Name   string
}

// Joined is the entry's path in the vault, its names joined by "/".
func (e ImportEntry) Joined() string {
	return strings.Join(e.Path, "/")
}

// Sorted is an archive's entries sorted: those the import reads, in the
// archive's order; the index of the vault's meta.json, -1 when it has
// none; and the problems of those it skips.
type Sorted struct {
	Entries []ImportEntry
	Meta    int
	Skipped []Problem
}

// Classify sorts raw, an archive's entries in its order. A name that is
// not UTF-8, or whose path leaves the root, is skipped; "\" separates
// names as "/" does, "." and empty names are dropped. Entries whose path
// holds a name that starts with a dot, __MACOSX or Thumbs.db are ignored,
// but for the vault's .nerve/meta.json. A folder that is the archive's
// only one at its top, besides those ignored, and holds .obsidian or
// .nerve, is the vault itself: the paths start below it. A file that is
// a symbolic link or no regular one, encrypted, or of a method the import
// does not read, and the second entry of a path, are skipped.
func Classify(raw []RawEntry) Sorted {
	var out Sorted
	out.Meta = -1
	var kept []candidate
	for _, r := range raw {
		shown := strings.ToValidUTF8(r.Name, "\uFFFD")
		if !utf8.ValidString(r.Name) {
			out.Skipped = append(out.Skipped, Problem{Path: shown, Code: ProblemNameNotUTF8})
			continue
		}
		path, ok := pathOf(r.Name)
		if !ok {
			out.Skipped = append(out.Skipped, Problem{Path: shown, Code: ProblemUnsafePath})
			continue
		}
		if len(path) > 0 {
			kept = append(kept, candidate{raw: r, path: path})
		}
	}
	if top, ok := vaultFolder(kept); ok {
		for i, c := range kept {
			if c.path[0] == top {
				kept[i].path = c.path[1:]
			}
		}
	}
	seen := map[string]bool{}
	for _, c := range kept {
		switch {
		case len(c.path) == 0:
			continue
		case len(c.path) == 2 && c.path[0] == ".nerve" && c.path[1] == "meta.json" && !c.raw.Folder:
			if out.Meta < 0 {
				out.Meta = c.raw.Index
			}
			continue
		case slices.ContainsFunc(c.path, ignored):
			continue
		}
		problem := ProblemCode("")
		joined := strings.Join(c.path, "/")
		switch {
		case c.raw.Folder:
			if seen[joined+"/"] {
				continue
			}
			seen[joined+"/"] = true
			out.Entries = append(out.Entries, ImportEntry{Index: c.raw.Index, Path: c.path, Folder: true, Name: c.raw.Name})
			continue
		case c.raw.Special:
			problem = ProblemSpecialFile
		case c.raw.Encrypted:
			problem = ProblemEncrypted
		case c.raw.Method != MethodStore && c.raw.Method != MethodDeflate:
			problem = ProblemUnsupportedMethod
		case seen[joined]:
			problem = ProblemDuplicate
		}
		if problem != "" {
			out.Skipped = append(out.Skipped, Problem{Path: c.raw.Name, Code: problem})
			continue
		}
		seen[joined] = true
		out.Entries = append(out.Entries, ImportEntry{Index: c.raw.Index, Path: c.path, Name: c.raw.Name})
	}
	return out
}

// pathOf is the names of an entry's path: "\" separates them as "/" does,
// "." and empty names dropped, each in NFC. false for a path that leaves
// the root: absolute, starting with a drive ("C:"), or holding "..".
func pathOf(name string) ([]string, bool) {
	name = strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(name, "/") || drive(name) {
		return nil, false
	}
	var path []string
	for part := range strings.SplitSeq(name, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			return nil, false
		}
		path = append(path, norm.NFC.String(part))
	}
	return path, true
}

// drive reports whether name starts with a Windows drive: a letter and a
// colon.
func drive(name string) bool {
	return len(name) >= 2 && name[1] == ':' && (name[0] >= 'a' && name[0] <= 'z' || name[0] >= 'A' && name[0] <= 'Z')
}

// ignored reports whether an entry under a folder, or a file, named name
// is left out of an import: a hidden one (.obsidian, .trash, .git,
// .DS_Store…), macOS's __MACOSX, Windows's Thumbs.db.
func ignored(name string) bool {
	return strings.HasPrefix(name, ".") || name == "__MACOSX" || strings.EqualFold(name, "Thumbs.db")
}

// candidate is an entry whose name is UTF-8 and whose path stays in the
// root: its path's names.
type candidate struct {
	raw  RawEntry
	path []string
}

// vaultFolder is the folder at the top of the entries that is the vault
// itself: every entry not ignored at the top is in it, or is it, and some
// entry in it is under .obsidian or .nerve.
func vaultFolder(entries []candidate) (string, bool) {
	top, marked := "", false
	for _, c := range entries {
		p := c.path
		if ignored(p[0]) {
			continue
		}
		if len(p) == 1 && !c.raw.Folder {
			return "", false
		}
		if top == "" {
			top = p[0]
		} else if p[0] != top {
			return "", false
		}
		if len(p) > 1 && (p[1] == ".obsidian" || p[1] == ".nerve") {
			marked = true
		}
	}
	return top, top != "" && marked
}
