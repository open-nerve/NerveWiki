package domain

import "strconv"

// Args is where a write's arguments are in its call, as its 422s name
// them (M9/P3 design 3.2): the zero value for a tool's own call, whose
// fields are its arguments' names ("path", "to"); OpArgs(i) for the op i
// of a batch, "ops[i].path". Each rule of a step names its fields through
// it, so that a batch's op answers the rule's own 422 at its op.
type Args struct{ prefix string }

// OpArgs is the arguments of a batch's op i, from 0.
func OpArgs(i int) Args { return Args{prefix: "ops[" + strconv.Itoa(i) + "]."} }

// Field is the field of the argument name: "to", or "ops[3].to".
func (a Args) Field(name string) string { return a.prefix + name }
