package domain

import (
	"errors"
	"uuid"
)

// Refused is a guard's or a precheck's refusal of a step, marked with the
// node it is about (M10 design 4.6): a batch answers it at its operation
// rather than read it as a tree that changed, and a background job stops
// with it as refused, its code and node in the report. It unwraps to the
// refusal, so every errors.Is and errors.As of today sees through it.
type Refused struct {
	Err    error
	NodeID uuid.UUID
}

func (r *Refused) Error() string { return r.Err.Error() }
func (r *Refused) Unwrap() error { return r.Err }

// Refuse marks err as a refusal of the node id; an err marked already is
// as it was.
func Refuse(err error, id uuid.UUID) error {
	if r := (*Refused)(nil); errors.As(err, &r) {
		return err
	}
	return &Refused{Err: err, NodeID: id}
}
