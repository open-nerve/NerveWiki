package main

import (
	"github.com/spf13/cobra"

	"github.com/open-nerve/NerveWiki/server/internal/bootstrap"
)

// newWorkspacesCommand is `nervewiki workspaces`, the server
// administrator's workspace commands (M2/P4 design 3.3). Every flag is
// required.
func newWorkspacesCommand(load configLoader) *cobra.Command {
	workspaces := &cobra.Command{
		Use:   "workspaces",
		Short: "Manage workspaces as the server's administrator",
		// A runnable parent makes cobra validate its args, so a mistyped
		// subcommand fails instead of printing help and exiting 0.
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	var slug, name, admin string
	create := workspaceCommand(load, "create --slug <slug> --name <name> --admin <email>",
		"Create a workspace with an account as its admin; works while workspace creation is disabled",
		func() bootstrap.WorkspaceCommand { return bootstrap.CreateWorkspace(slug, name, admin) })
	requiredFlag(create, &slug, "slug", "the workspace's slug, its address")
	requiredFlag(create, &name, "name", "the workspace's name")
	requiredFlag(create, &admin, "admin", "the e-mail address of the account that becomes its admin")
	var workspace, email string
	reactivate := workspaceCommand(load, "reactivate-member --workspace <slug> --email <address>",
		"Make an account's ended membership of a workspace active again, with its role; activate the account first",
		func() bootstrap.WorkspaceCommand { return bootstrap.ReactivateMember(workspace, email) })
	requiredFlag(reactivate, &workspace, "workspace", "the workspace's slug")
	requiredFlag(reactivate, &email, "email", "the account's e-mail address")
	workspaces.AddCommand(create, reactivate)
	return workspaces
}

// workspaceCommand builds one `nervewiki workspaces` subcommand: it loads
// the configuration, then runs the command build makes from its flags.
func workspaceCommand(load configLoader, use, short string, build func() bootstrap.WorkspaceCommand) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			return bootstrap.Workspaces(cmd.Context(), cfg, cmd.ErrOrStderr(), cmd.OutOrStdout(), build())
		},
	}
}

// requiredFlag adds the required string flag name to cmd, read into p.
func requiredFlag(cmd *cobra.Command, p *string, name, usage string) {
	cmd.Flags().StringVar(p, name, "", usage)
	_ = cmd.MarkFlagRequired(name) // the flag exists
}
