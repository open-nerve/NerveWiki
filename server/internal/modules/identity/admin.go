package identity

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// AdminDeps are what bootstrap builds for the server administrator's
// commands: the pool and the settings they need, nothing of HTTP.
type AdminDeps struct {
	Pool     *pgxpool.Pool
	Tx       shared.TxManager
	Clock    app.Clock
	Logger   *slog.Logger
	Password PasswordHashing
	// The deactivation's registrants, the same as Deps's: a deactivation
	// from the command line goes through them too (M1 design 8).
	DeactivationVetoers     []DeactivationVetoer
	DeactivationSubscribers []DeactivationSubscriber
}

// The results of the administrator's commands.
type (
	// CreatedUser is the account the administrator created.
	CreatedUser = app.CreatedUser
	// PasswordReset is the account whose password was reset and what the
	// reset revoked.
	PasswordReset = app.ResetPasswordResult
	// EmailChange is the account's new address and the sessions revoked.
	EmailChange = app.SetEmailResult
	// Deactivated is the account deactivated and the sessions revoked, or
	// that it was inactive already.
	Deactivated = app.DeactivateAccountResult
	// Activated is the account activated and its tokens usable again, or
	// that it was active already.
	Activated = app.ActivateResult
)

// Admin is the server administrator's account commands (M1/P4 design 3.6):
// nervewiki users. Each names the account by its address.
type Admin struct {
	create     *app.CreateUser
	reset      *app.ResetPassword
	setEmail   *app.SetEmail
	deactivate *app.DeactivateAccount
	activate   *app.Activate
}

// NewAdmin wires the commands from the pool alone: no signing key, rate
// limit or sign-up switch. It is not a method of Module, so that the
// command line need not build the HTTP side first (M1/P4 design 3.8).
func NewAdmin(d AdminDeps) *Admin {
	p := newParts(d.Pool, d.Password, d.Logger, d.DeactivationVetoers, d.DeactivationSubscribers)
	store := p.store
	return &Admin{
		create: app.NewCreateUser(app.CreateUserDeps{Rules: p.rules, Hasher: p.hasher, Users: store, Clock: d.Clock, Logger: d.Logger}),
		reset: app.NewResetPassword(app.ResetPasswordDeps{
			Accounts: store, Passwords: store, Sessions: store, APITokens: store, Hasher: p.hasher, Rules: p.rules,
			Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		setEmail: app.NewSetEmail(app.SetEmailDeps{
			Accounts: store, Emails: store, Sessions: store, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		deactivate: app.NewDeactivateAccount(app.DeactivateAccountDeps{
			Accounts: store, Steps: p.steps, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		activate: app.NewActivate(app.ActivateDeps{
			Accounts: store, Users: store, APITokens: store, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
	}
}

// CreateUser creates the account of email with password; it signs nobody
// in and does not ask whether sign-up is open.
func (a *Admin) CreateUser(ctx context.Context, email, password string) (CreatedUser, error) {
	return a.create.Execute(ctx, email, password)
}

// ResetPassword sets the password of the account of email and revokes every
// session and personal access token of it.
func (a *Admin) ResetPassword(ctx context.Context, email, password string) (PasswordReset, error) {
	return a.reset.Execute(ctx, email, password)
}

// SetEmail gives the account of email the address newEmail and revokes
// every session of it.
func (a *Admin) SetEmail(ctx context.Context, email, newEmail string) (EmailChange, error) {
	return a.setEmail.Execute(ctx, email, newEmail)
}

// Deactivate deactivates the account of email, through the deactivation's
// registrants.
func (a *Admin) Deactivate(ctx context.Context, email string) (Deactivated, error) {
	return a.deactivate.Execute(ctx, email)
}

// Activate activates the account of email again.
func (a *Admin) Activate(ctx context.Context, email string) (Activated, error) {
	return a.activate.Execute(ctx, email)
}
