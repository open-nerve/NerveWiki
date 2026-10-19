package domain

import (
	"slices"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A batch of MCP's writes (M9 design 4.11; M9/P3 design 3.3): its ops as
// the agent gave them, each checked by its tool's own rules at ops[i].

// BatchArgs is an op of a batch as the agent gave it: Op names the tool,
// the rest are the arguments, nil when left out.
type BatchArgs struct {
	Op                                      string
	Path, Content, Base, Old, New, From, To *string
	All, UpdateLinks, Recursive             *bool
}

// OpKind is the tool an op of a batch is (M9/P3 design 3.3.2). Its values
// are OpKind*, since the write guards' Operation has OpMove and OpDelete.
type OpKind string

// The tools a batch's op is one of.
const (
	OpKindWrite  OpKind = "write"
	OpKindEdit   OpKind = "edit"
	OpKindMove   OpKind = "move"
	OpKindDelete OpKind = "delete"
)

// BatchStep is an op checked: its kind, where its arguments are, and
// them parsed; Base nil without base_version.
type BatchStep struct {
	Kind                        OpKind
	Args                        Args
	Path, From, To              Path
	Content, Old, New           string
	Base                        *VersionToken
	All, UpdateLinks, Recursive bool
}

const (
	// MaxBatchOps is the most ops a batch takes (M9 design 4.11). Its
	// largest body, 100 edits of five arguments, is some 715 JSON values,
	// some 1,215 with every member of the flat op set: an eighth of
	// mcpserver.MaxBodyValues, which the edge counts before any decode
	// (M9/P2 design 3.2.6); a tool with lists stays far below it too.
	MaxBatchOps = 100
	// MaxBatchStringBytes bounds a batch's strings together, paths and
	// messages included; its contents and new_strings together are within
	// MaxContentBytes, as one write's content is (M9 design 4.11, 4.14).
	MaxBatchStringBytes = 6 << 20
)

// member is an argument of a batch's op: its name, and whether the op
// gave it.
type member struct {
	name  string
	given func(BatchArgs) bool
}

// members are the arguments of a batch's op besides op, in the flat op's
// order, which a refusal of an argument the tool does not take follows.
func members() []member {
	return []member{
		{"path", func(o BatchArgs) bool { return o.Path != nil }},
		{"content", func(o BatchArgs) bool { return o.Content != nil }},
		{"base_version", func(o BatchArgs) bool { return o.Base != nil }},
		{"old_string", func(o BatchArgs) bool { return o.Old != nil }},
		{"new_string", func(o BatchArgs) bool { return o.New != nil }},
		{"replace_all", func(o BatchArgs) bool { return o.All != nil }},
		{"from", func(o BatchArgs) bool { return o.From != nil }},
		{"to", func(o BatchArgs) bool { return o.To != nil }},
		{"update_links", func(o BatchArgs) bool { return o.UpdateLinks != nil }},
		{"recursive", func(o BatchArgs) bool { return o.Recursive != nil }},
	}
}

// toolArgs is the arguments the tool kind takes in a batch, as it takes
// them on its own: the required ones, in the order its use case checks
// them, and the optional ones; false for no tool.
func toolArgs(kind OpKind) (required, optional []string, ok bool) {
	switch kind {
	case OpKindWrite:
		return []string{"path", "content"}, []string{"base_version"}, true
	case OpKindEdit:
		return []string{"path", "old_string", "new_string"}, []string{"replace_all", "base_version"}, true
	case OpKindMove:
		return []string{"from", "to"}, []string{"update_links"}, true
	case OpKindDelete:
		return []string{"path"}, []string{"recursive"}, true
	}
	return nil, nil, false
}

// CheckBatch checks a batch's values, before any read (M9 design 4.15,
// the value 422s): its number of ops, none validation_failed on ops,
// past MaxBatchOps page.batch_too_large; its strings together and its
// contents and new_strings together, page.batch_too_large; its message
// (CheckMessage); then each op in their order: its tool, its members as
// the tool takes them, required and not_allowed at ops[i], and their
// values by the tool's own rules and order (ParsePath, PagePath,
// CheckContent, CheckEdit, ParseVersionToken, MoveKinds). The first that
// fails is the answer, an *OpFailed for an op's.
func CheckBatch(ops []BatchArgs, message string) ([]BatchStep, error) {
	switch {
	case len(ops) == 0:
		return nil, shared.Invalid(shared.FieldError{Field: "ops", Code: shared.FieldTooShort, Message: "must hold at least one op"})
	case len(ops) > MaxBatchOps:
		return nil, BatchTooLarge("ops", len(ops), MaxBatchOps, "split the batch")
	}
	strs, contents := len(message), 0
	for _, o := range ops {
		strs += len(o.Op)
		for _, s := range []*string{o.Path, o.Content, o.Base, o.Old, o.New, o.From, o.To} {
			if s != nil {
				strs += len(*s)
			}
		}
		for _, s := range []*string{o.Content, o.New} {
			if s != nil {
				contents += len(*s)
			}
		}
	}
	switch {
	case strs > MaxBatchStringBytes:
		return nil, BatchTooLarge("bytes of strings, paths and message included", strs, MaxBatchStringBytes, "split the batch")
	case contents > MaxContentBytes:
		return nil, BatchTooLarge("bytes of contents and new_strings", contents, MaxContentBytes, "split the batch")
	}
	if err := CheckMessage(message); err != nil {
		return nil, err
	}
	steps := make([]BatchStep, len(ops))
	for i, o := range ops {
		s, err := checkOp(OpArgs(i), o)
		if err != nil {
			return nil, &OpFailed{Op: i, Tool: s.Kind, Err: err}
		}
		steps[i] = s
	}
	return steps, nil
}

// checkOp checks the op o at a: its tool, its members, then their values
// as its tool's use case checks them. The step has its kind even when a
// later check fails.
func checkOp(a Args, o BatchArgs) (BatchStep, error) {
	kind := OpKind(o.Op)
	required, optional, ok := toolArgs(kind)
	if !ok {
		return BatchStep{}, NotAllowed(a.Field("op"), "is one of write, edit, move and delete")
	}
	s := BatchStep{Kind: kind, Args: a, UpdateLinks: true}
	if err := checkMembers(a, o, kind, required, optional); err != nil {
		return s, err
	}
	var err error
	switch kind {
	case OpKindWrite, OpKindEdit:
		if s.Path, err = ParsePath(a.Field("path"), *o.Path, false); err != nil {
			return s, err
		}
		if err := PagePath(a, s.Path, string(kind)); err != nil {
			return s, err
		}
		if kind == OpKindWrite {
			s.Content = *o.Content
			err = CheckContent(a.Field("content"), s.Content)
		} else {
			s.Old, s.New, s.All = *o.Old, *o.New, o.All != nil && *o.All
			err = CheckEdit(a, s.Old, s.New)
		}
		if err != nil {
			return s, err
		}
		if o.Base != nil {
			base, err := ParseVersionToken(a.Field("base_version"), *o.Base)
			if err != nil {
				return s, err
			}
			s.Base = &base
		}
	case OpKindMove:
		if s.From, err = ParsePath(a.Field("from"), *o.From, false); err != nil {
			return s, err
		}
		if s.To, err = ParsePath(a.Field("to"), *o.To, false); err != nil {
			return s, err
		}
		if err := MoveKinds(a, s.From, s.To); err != nil {
			return s, err
		}
		s.UpdateLinks = o.UpdateLinks == nil || *o.UpdateLinks
	case OpKindDelete:
		if s.Path, err = ParsePath(a.Field("path"), *o.Path, false); err != nil {
			return s, err
		}
		s.Recursive = o.Recursive != nil && *o.Recursive
	}
	return s, nil
}

// checkMembers is 422 at a's member: one the tool kind requires that o
// lacks, required, before one o gives that kind takes not, not_allowed.
func checkMembers(a Args, o BatchArgs, kind OpKind, required, optional []string) error {
	all := members()
	given := func(name string) bool {
		i := slices.IndexFunc(all, func(m member) bool { return m.name == name })
		return all[i].given(o)
	}
	for _, name := range required {
		if !given(name) {
			return shared.Invalid(shared.FieldError{Field: a.Field(name), Code: shared.FieldRequired,
				Message: "is required for " + string(kind)})
		}
	}
	for _, m := range all {
		if m.given(o) && !slices.Contains(required, m.name) && !slices.Contains(optional, m.name) {
			return NotAllowed(a.Field(m.name), "is no argument of "+string(kind))
		}
	}
	return nil
}
