package domain

// VirtualName identifies a generated file; it is never a node or version.
type VirtualName string

const (
	VirtualIndex VirtualName = "index.md"
	VirtualLog   VirtualName = "log.md"
)

// CanonicalVirtualRead accepts only a root page with one .md suffix. ParsePath
// already normalized the title keys and checked the spelling's grammar.
func CanonicalVirtualRead(path Path) (VirtualName, bool) {
	if path.Kind != KindPage || path.Dir || len(path.Keys) != 1 {
		return "", false
	}
	switch path.Keys[0] {
	case "index":
		return VirtualIndex, true
	case "log":
		return VirtualLog, true
	}
	return "", false
}

// VirtualFileError names a generated root file. Its code and explanation
// are the existing virtual-file contract, shared with destination guards.
type VirtualFileError struct{ Name string }

func (e *VirtualFileError) Error() string { return ErrVirtualFile.Error() }
func (e *VirtualFileError) Unwrap() error { return ErrVirtualFile }

// VirtualLookup refuses the four reserved root title keys before a real
// lookup. The caller supplies the state it already read, including a
// batch's simulated tree state. Nested and standard paths are unchanged.
func VirtualLookup(state NotebookState, next Lookup) Lookup {
	return func(chain []Node, key string) (Node, bool, error) {
		if state.Mode == ModeLLMWiki && len(chain) == 0 && Reserved(key) {
			return Node{}, false, &VirtualFileError{Name: key}
		}
		return next(chain, key)
	}
}
