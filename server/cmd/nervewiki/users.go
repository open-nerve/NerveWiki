package main

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/open-nerve/NerveWiki/server/internal/bootstrap"
)

// newUsersCommand is `nervewiki users`, the server administrator's account
// commands (M1/P4 design 3.7). Each names the account with --email; the
// ones that set a password read it from stdin, through tty when stdin is a
// terminal.
func newUsersCommand(load configLoader, stdin io.Reader, tty terminal) *cobra.Command {
	users := &cobra.Command{
		Use:   "users",
		Short: "Manage accounts as the server's administrator",
		// A runnable parent makes cobra validate its args, so a mistyped
		// subcommand fails instead of printing help and exiting 0.
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	withPassword := func(command func(email, password string) bootstrap.UserCommand) func(*cobra.Command, string) (bootstrap.UserCommand, error) {
		return func(cmd *cobra.Command, email string) (bootstrap.UserCommand, error) {
			password, err := readPassword(cmd.Context(), stdin, cmd.ErrOrStderr(), tty)
			return command(email, password), err
		}
	}
	withoutPassword := func(command func(email string) bootstrap.UserCommand) func(*cobra.Command, string) (bootstrap.UserCommand, error) {
		return func(_ *cobra.Command, email string) (bootstrap.UserCommand, error) { return command(email), nil }
	}
	users.AddCommand(
		userCommand(load, "create", "Create an account without signing it in; works while sign-up is disabled",
			withPassword(bootstrap.CreateUser)),
		userCommand(load, "reset-password", "Set an account's password and revoke all its sessions and API tokens",
			withPassword(bootstrap.ResetPassword)),
		setEmailCommand(load),
		userCommand(load, "deactivate", "Deactivate an account and revoke its sessions; its API tokens stop working until it is activated",
			withoutPassword(bootstrap.DeactivateUser)),
		userCommand(load, "activate", "Activate an account; its unexpired API tokens work again, "+
			"so run reset-password too if it may be compromised",
			withoutPassword(bootstrap.ActivateUser)),
	)
	return users
}

// userCommand builds one `nervewiki users` subcommand: it requires --email,
// loads the configuration, then has prepare build the command, which may
// read the password, so that a configuration error comes before the prompt.
func userCommand(load configLoader, use, short string,
	prepare func(cmd *cobra.Command, email string) (bootstrap.UserCommand, error)) *cobra.Command {
	var email string
	cmd := &cobra.Command{
		Use:   use + " --email <address>",
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			command, err := prepare(cmd, email)
			if err != nil {
				return err
			}
			return bootstrap.Users(cmd.Context(), cfg, cmd.ErrOrStderr(), cmd.OutOrStdout(), command)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "the account's e-mail address")
	_ = cmd.MarkFlagRequired("email") // the flag exists
	return cmd
}

// setEmailCommand is `nervewiki users set-email`.
func setEmailCommand(load configLoader) *cobra.Command {
	var newEmail string
	cmd := userCommand(load, "set-email",
		"Change an account's e-mail address and revoke its sessions; its API tokens stay, "+
			"so run reset-password too if the change is about a compromise",
		func(_ *cobra.Command, email string) (bootstrap.UserCommand, error) {
			return bootstrap.SetEmail(email, newEmail), nil
		})
	cmd.Use += " --new-email <address>"
	cmd.Flags().StringVar(&newEmail, "new-email", "", "the new e-mail address")
	_ = cmd.MarkFlagRequired("new-email") // the flag exists
	return cmd
}
