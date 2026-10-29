package mcpadapter

import (
	"context"
	"errors"
	"maps"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// batchDescription is batch's description, as api/mcp/tools.json has it
// (M9/P3 design 3.12.1).
const batchDescription = `Apply up to 100 writes as one change, all or none, with a message the history shows. Each op ` +
	`is a write, edit, move or delete: op names the tool and its other members are that tool's arguments; it acts on ` +
	`the notebook as the ops before it left it, their paths and contents. base_version is a page's version from before ` +
	`the batch; a page the batch creates takes none. A refused op writes nothing at all: the error has its op, from 0, ` +
	`and that tool's code. A delete of a page whose descendants an earlier op deleted takes them in one deletion, the ` +
	`earlier op answering merged_into; a batch may not delete what it creates, renames or moves. Unlike separate calls, ` +
	`links follow the batch's renames and moves, merged node by node, from where they led before the batch, or, in a ` +
	`page the batch writes, at its latest write before them (on the tree then, with the aliases from before the batch): ` +
	`what other ops did in between to where they lead (creating, renaming, moving or deleting a page) is not seen, nor ` +
	`any alias the batch changed. A page the batch writes or deletes that is being edited in the web editor refuses at ` +
	`the first op that writes or deletes it, as page.locked; one that only the links' rewrite of its renames and moves ` +
	`reaches is checked after those, as linking.pages_locked at its first rename or move. The answer has the ` +
	`changeset's id and time and each op's result, as many as fit, results_count ` +
	`all of them: changeset reads the rest. What the batch writes reaches the search some seconds after it: building ` +
	`says it is on its way. Each op's changed is what its tool would answer at that step; the batch's tells whether it ` +
	`left a change. ` +
	`In an LLM Wiki, one move of an existing subtree from outside into the source area may carry at most ` +
	`10,000 nodes, including its root, empty pages and attachments; above that it is page.source_too_large. ` +
	`Known invalid destination, cycle, title, protection and reserved-name errors come first; the bound precedes ` +
	`depth errors in descendants not yet read. Ordinary moves, moves within or out of sources, and moves of ` +
	`the source root or an ancestor that keep source membership are unchanged. Move smaller subtrees into sources separately.` +
	` Writes to each page are merged into its final body before the write unit runs. Source duplicate checks at creation and source entry use those merged bytes, so a batch can differ from the same tools called separately or from their apparent intermediate bodies. A refusal keeps the entire batch unchanged.`

// batchIn is batch's arguments (M9 design 4.11; M9/P3 design 3.3): ops,
// each one flat object, op naming the tool and its other members that
// tool's arguments. The schema has no oneOf, so an op's members are the
// use case's: a member its tool takes not given is 422 required at
// ops[i], one it does not take 422 not_allowed.
type batchIn struct {
	Ops     []batchOp `json:"ops" jsonschema:"The writes, applied in their order as one change: 1 to 100."`
	Message string    `json:"message" jsonschema:"Why the batch is made, shown with its change in the history: 1 to 4,096 bytes."`
}

// batchOp is one op of batch: op and the arguments of its tool.
type batchOp struct {
	Op          string  `json:"op" jsonschema:"The tool this op is: write, edit, move or delete. Its other members are that tool's arguments."`
	Path        *string `json:"path,omitempty" jsonschema:"write, edit and delete: the path, from the notebook's root, as that tool takes it."`
	Content     *string `json:"content,omitempty" jsonschema:"write: the page's whole content, as it will be stored."`
	BaseVersion *string `json:"base_version,omitempty" jsonschema:"write and edit: the page's version as read before the batch. None for a page the batch creates."`
	OldString   *string `json:"old_string,omitempty" jsonschema:"edit: the text to replace, byte for byte, which must occur exactly once unless replace_all is set."`
	NewString   *string `json:"new_string,omitempty" jsonschema:"edit: the text to put in its place."`
	ReplaceAll  *bool   `json:"replace_all,omitempty" jsonschema:"edit: replace every occurrence of old_string. Default false."`
	From        *string `json:"from,omitempty" jsonschema:"move: the path of the page or attachment that moves."`
	To          *string `json:"to,omitempty" jsonschema:"move: the full path it will have, never a page to move it into."`
	UpdateLinks *bool   `json:"update_links,omitempty" jsonschema:"move: rewrite the links to what moves so that they follow it. Default true."`
	Recursive   *bool   `json:"recursive,omitempty" jsonschema:"delete: delete a page with its children. Default false."`
}

// batchAnswer is batch's answer (M9 design 4.7, 4.11; M9/P3 design
// 3.9.1): the changeset's id and time, null when nothing changed; building,
// whether it queued nodes for the search, in bulk, which the search finds
// once its index has taken them (3.10.1); the ops' results in their order,
// as many as fit, with the nodes each write created; results_count all of
// them, the rest read with changeset.
type batchAnswer struct {
	ChangesetID  *string    `json:"changeset_id"`
	At           *time.Time `json:"at"`
	Changed      bool       `json:"changed"`
	Building     bool       `json:"building"`
	Results      []opResult `json:"results"`
	ResultsCount int        `json:"results_count"`
}

// opResult is an op's result, its tool's answer without the changeset
// (the batch's): path, the node's after the batch, a delete's when
// deleted, and at the op for a page a later op deleted; a content op's
// version, its page's as the batch left it, or as the op's write left it
// for a page a later op deleted; changed, what the op's tool would answer
// at its step; created, the nodes a write created, root first, at their
// paths after the batch, created_count all of them; a delete merged into
// a later one, merged_into, with no pages or attachments of its own.
type opResult struct {
	Op           string        `json:"op"`
	Path         string        `json:"path"`
	NodeID       string        `json:"node_id"`
	Changed      bool          `json:"changed"`
	Version      *string       `json:"version,omitempty"`
	Created      []createdNode `json:"created,omitempty"`
	CreatedCount *int          `json:"created_count,omitempty"`
	Replacements *int          `json:"replacements,omitempty"`
	Pages        *int          `json:"pages,omitempty"`
	Attachments  *int          `json:"attachments,omitempty"`
	MergedInto   *int          `json:"merged_into,omitempty"`
}

// batch applies the ops as one change. The answer holds the ops and the
// nodes their writes created, in one column, as many as fit: an op cut
// has fewer created than created_count, results fewer than results_count.
func (t tools) batch(ctx context.Context, c mcpserver.Call, in batchIn) (mcpserver.Result, error) {
	ops := make([]domain.BatchArgs, len(in.Ops))
	for i, o := range in.Ops {
		ops[i] = domain.BatchArgs{Op: o.Op, Path: o.Path, Content: o.Content, Base: o.BaseVersion, Old: o.OldString, New: o.NewString,
			All: o.ReplaceAll, From: o.From, To: o.To, UpdateLinks: o.UpdateLinks, Recursive: o.Recursive}
	}
	b, err := t.uc.Batch.Execute(ctx, c.Notebook, app.PathBatch{Ops: ops, Message: in.Message}, domain.MCPClient(c.Client))
	if err != nil {
		return mcpserver.Result{}, t.duplicateErr(ctx, c, batchErr(err, c, in.Ops))
	}
	results := make([]opResult, len(b.Ops))
	// column is the answer's items in their order: each op, then the
	// nodes it created, by their op and their place (-1 for the op).
	type item struct{ op, created int }
	var column []item
	for i, d := range b.Ops {
		results[i] = resultOf(d)
		column = append(column, item{i, -1})
		for j := range d.Created {
			column = append(column, item{i, j})
		}
	}
	changeset, at := changesetOf(b.Outcome)
	res, _, err := mcpserver.Fit(len(column), func(k int) any {
		out := []opResult{}
		for _, it := range column[:k] {
			if it.created < 0 {
				r := results[it.op]
				r.Created = nil
				out = append(out, r)
				continue
			}
			last := &out[len(out)-1]
			last.Created = append(last.Created, results[it.op].Created[it.created])
		}
		return batchAnswer{ChangesetID: changeset, At: at, Changed: changeset != nil, Building: b.Outcome.Building, Results: out,
			ResultsCount: len(results)}
	})
	return res, err
}

// resultOf is an op's result with all it created: each tool's members, as
// its own answer has them.
func resultOf(d app.OpDone) opResult {
	r := opResult{Op: string(d.Kind), Path: d.Path, NodeID: d.Node.ID.String(), Changed: d.Changed}
	switch d.Kind {
	case domain.OpKindWrite:
		r.Version, r.CreatedCount = new(d.Version.String()), new(len(d.Created))
		for _, n := range d.Created {
			r.Created = append(r.Created, createdNode{Path: n.Path, NodeID: n.Node.ID.String()})
		}
	case domain.OpKindEdit:
		r.Version, r.Replacements = new(d.Version.String()), new(d.Replacements)
	case domain.OpKindDelete:
		r.Pages, r.Attachments = new(d.Pages), new(d.Assets)
		if d.MergedInto >= 0 {
			r.MergedInto = new(d.MergedInto)
		}
	}
	return r
}

// batchErr is batch's err (M9/P3 design 3.9.2): an op's in its tool's
// words, from the op's own path (writeErr, toolErr, moveErr), the batch's
// deferred rewrite's refusal of edit locks in the batch's, then placed at
// the op (atOp); any other as toolErr has it.
func batchErr(err error, c mcpserver.Call, ops []batchOp) error {
	var f *domain.OpFailed
	if !errors.As(err, &f) {
		return toolErr(err, c, "")
	}
	var o batchOp
	if f.Op >= 0 && f.Op < len(ops) {
		o = ops[f.Op]
	}
	path := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	var locks *domain.LockPaths
	inner := f.Err
	switch {
	case errors.As(f.Err, &locks) && errors.Is(f.Err, errPagesLocked):
		inner = lockErrOf(f.Err, locks, c, "", "The batch's renames and moves")
	case f.Tool == domain.OpKindWrite:
		inner = writeErr(f.Err, c, path(o.Path))
	case f.Tool == domain.OpKindMove:
		inner = moveErr(f.Err, c, path(o.From))
	case f.Tool != "":
		inner = toolErr(f.Err, c, path(o.Path))
	}
	return atOp(inner, f)
}

// errPagesLocked is linking.pages_locked as the page module meets it, a
// rewrite's refusal: only a batch's deferred rewrite answers it at an op.
var errPagesLocked = shared.NewError(shared.KindConflict, "linking.pages_locked", "")

// atOp is err, the op f's in its tool's words, at the op (M9/P3 design
// 3.9.2): mcpserver writes the place "ops[i] (tool)" after the code in
// whatever words answer (ToolError.At), and op, from 0, is a member of
// the error, in each of Fit's builds.
func atOp(err error, f *domain.OpFailed) error {
	var te *mcpserver.ToolError
	if !errors.As(err, &te) {
		return &mcpserver.ToolError{Err: err, At: f.Place(), Extra: map[string]any{"op": f.Op}}
	}
	out := *te
	out.At = f.Place()
	if te.Fit != nil {
		build := te.Fit.Build
		out.Fit = &mcpserver.ErrorFit{N: te.Fit.N, Build: func(k int) (string, map[string]any) {
			text, extra := build(k)
			return text, withOp(extra, f.Op)
		}}
		return &out
	}
	out.Extra = withOp(te.Extra, f.Op)
	return &out
}

// withOp is extra with op set, extra itself left as it is.
func withOp(extra map[string]any, op int) map[string]any {
	out := maps.Clone(extra)
	if out == nil {
		out = map[string]any{}
	}
	out["op"] = op
	return out
}
