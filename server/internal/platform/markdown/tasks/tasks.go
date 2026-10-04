// Package tasks is the extension of task items (M5 design 4.12; M5/P6
// design 3.2): GFM's task list as goldmark parses it, each checkbox with its
// byte position in the page's content, so that a reader can tick it and the
// server can change that one byte.
package tasks

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// Name is the extension's name: Document.Extracted(Name) is a []Task.
const Name = "tasks"

// Task is a task item. Offset is the byte position in the content of the
// character between its brackets: a space (or a tab, a form feed, a
// carriage return) when it is open, 'x' or 'X' when it is done.
type Task struct {
	Offset  int
	Checked bool
}

// Extension is the extension of task items. It replaces goldmark's: its
// parser recognizes what goldmark's does, its renderer writes what
// goldmark's writes and the checkbox's position in data-task, and Extract
// gives the document's tasks in the content's order.
func Extension() markdown.Extension {
	return markdown.Extension{
		Name: Name,
		// goldmark's priority: before links, which '[' also triggers.
		Parser:  []parser.Option{parser.WithInlineParsers(util.Prioritized(taskParser{}, 0))},
		Extract: extract,
		Renderer: func(any) []util.PrioritizedValue {
			return []util.PrioritizedValue{util.Prioritized(taskRenderer{}, 500)}
		},
		Markup: markdown.Markup{Elements: map[string][]string{"input": {"type", "checked", "disabled", "data-task"}}},
	}
}

// kind is the node's kind: goldmark numbers the kinds as they are made, so
// it is made once, as goldmark makes its own.
//
//nolint:gochecknoglobals // made once, read only
var kind = ast.NewNodeKind("Task")

// node is a task item's checkbox, as goldmark's TaskCheckBox is.
type node struct {
	ast.BaseInline
	Task
}

func (n *node) Kind() ast.NodeKind { return kind }

func (n *node) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Offset": strconv.Itoa(n.Offset), "Checked": strconv.FormatBool(n.Checked),
	}, nil)
}

// pattern is goldmark's (extension/tasklist.go).
var pattern = regexp.MustCompile(`^\[([\sxX])\]\s*`)

// taskParser is goldmark's task list parser with the checkbox's position.
type taskParser struct{}

func (taskParser) Trigger() []byte { return []byte{'['} }

// Parse makes a checkbox where goldmark does: at the start of the first
// block of a list item, nothing parsed before it.
func (taskParser) Parse(parent ast.Node, block text.Reader, _ parser.Context) ast.Node {
	if parent.Parent() == nil || parent.Parent().FirstChild() != parent || parent.HasChildren() {
		return nil
	}
	if _, ok := parent.Parent().(*ast.ListItem); !ok {
		return nil
	}
	line, seg := block.PeekLine()
	m := pattern.FindSubmatchIndex(line)
	if m == nil {
		return nil
	}
	// The line starts with '[', so it has no padding (padding is spaces
	// before it): line[i] is the source's seg.Start+i.
	value := line[m[2]]
	block.Advance(m[1])
	return &node{Task: Task{Offset: seg.Start + m[2], Checked: value == 'x' || value == 'X'}}
}

func (taskParser) CloseBlock(ast.Node, parser.Context) {}

// taskRenderer writes goldmark's checkbox with its position.
type taskRenderer struct{}

func (taskRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kind, func(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		t := n.(*node)
		if t.Checked {
			_, _ = w.WriteString(`<input checked="" disabled="" type="checkbox"`)
		} else {
			_, _ = w.WriteString(`<input disabled="" type="checkbox"`)
		}
		_, _ = w.WriteString(` data-task="` + strconv.Itoa(t.Offset) + `"> `)
		return ast.WalkContinue, nil
	})
}

// extract is the document's tasks in the content's order: the tree's is
// the reading view's, which puts the footnotes' last. A checkbox is a
// block's inline child, or in the inline node another extension moved it
// into (M6: what a comment hides), so the walk goes into inline nodes too.
func extract(t markdown.Tree) any {
	var tasks []Task
	_ = ast.Walk(t.Root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := n.(*node); ok && entering {
			tasks = append(tasks, t.Task)
		}
		return ast.WalkContinue, nil
	})
	slices.SortFunc(tasks, func(a, b Task) int { return cmp.Compare(a.Offset, b.Offset) })
	return tasks
}
