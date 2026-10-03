package domain

// Flip is content with its task item at offset ticked, or cleared (M5/P6
// design 3.3): the character between the item's brackets made 'x', or a
// space; every other byte as it was. offset is a task item's, in content.
func Flip(content string, offset int, checked bool) string {
	mark := " "
	if checked {
		mark = "x"
	}
	return content[:offset] + mark + content[offset+1:]
}
