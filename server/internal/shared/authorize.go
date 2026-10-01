package shared

import (
	"context"
	"uuid"
)

// WorkspaceRole is a member's role in a workspace (v0.1 design 3.2), as the
// API and the database spell it. Rules compare roles by set, never by order:
// any other value is allowed nothing.
type WorkspaceRole string

// The three workspace roles.
const (
	WorkspaceAdmin  WorkspaceRole = "admin"
	WorkspaceMember WorkspaceRole = "member"
	WorkspaceGuest  WorkspaceRole = "guest"
)

// WorkspaceRoles are the three roles.
func WorkspaceRoles() []WorkspaceRole {
	return []WorkspaceRole{WorkspaceAdmin, WorkspaceMember, WorkspaceGuest}
}

// Action names an operation the Authorizer decides on, e.g.
// "workspace.read". Each module declares its own, with an Actions() that
// lists them; the access module's rule table is keyed by these names
// (M2/P1 design 3.4).
type Action string

// Target is what an action is on: a workspace.
type Target struct {
	WorkspaceID uuid.UUID
}

// Grant is what a decision read: the caller's role in the target's
// workspace. Use cases make the checks that compare two people with it
// instead of reading the role again.
type Grant struct {
	WorkspaceRole WorkspaceRole
}

// Authorizer decides whether actor may do action on t. It reads the facts
// on every call, in the transaction ctx carries: a write calls it after it
// has locked the target's row (v0.1 design 13.1, item 5). A caller who
// cannot see the target gets ErrNotVisible, which the use case turns into
// its own resource's 404; one who can see it but whose role the rule does
// not allow gets Forbidden. An action without a rule is an internal error:
// nothing is allowed.
type Authorizer interface {
	Authorize(ctx context.Context, actor Actor, action Action, t Target) (Grant, error)
}

// ErrNotVisible is the Authorizer's answer when the caller cannot see the
// target. It has no code: the use case recognizes it with errors.Is and
// answers the 404 code of what the caller named, e.g. workspace.not_found,
// which only the use case knows.
var ErrNotVisible = &Error{Kind: KindNotFound, Detail: "The target is not visible to the caller."}
