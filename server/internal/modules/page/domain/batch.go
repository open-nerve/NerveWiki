package domain

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
