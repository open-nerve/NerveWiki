package domain

import "strings"

// An attachment's name (M7/P2 design 3.3): a title, which an export writes
// as the file's name, that never reads as a page's file, nor loses the
// extension that tells its type.

// CheckAssetName checks s as field, an attachment's name: CheckTitle's
// rules, and no ".md" at its end, in any case, which an export would read
// back as a page. A name that breaks a rule is 422 on field.
func CheckAssetName(field, s string) (Title, error) {
	title, err := CheckTitle(field, s)
	if err != nil {
		return Title{}, err
	}
	if n := title.Name; len(n) >= len(".md") && strings.EqualFold(n[len(n)-len(".md"):], ".md") {
		return Title{}, NotAllowed(field, `An attachment's name must not end with ".md", which names a page's file.`)
	}
	return title, nil
}

// CheckAssetRename checks s as field, the new name of the attachment
// named old: CheckAssetName's rules, and an extension when old has one,
// which may change.
func CheckAssetRename(field, old, s string) (Title, error) {
	title, err := CheckAssetName(field, s)
	if err != nil {
		return Title{}, err
	}
	if hasExtension(old) && !hasExtension(title.Name) {
		return Title{}, NotAllowed(field, "An attachment's new name must keep an extension: its type is told by it.")
	}
	return title, nil
}

// hasExtension reports whether name ends with an extension: a dot that
// does not start it, and a character after it.
func hasExtension(name string) bool {
	i := strings.LastIndexByte(name, '.')
	return i > 0 && i < len(name)-1
}
