package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/open-nerve/NerveWiki/server/configs"
	"github.com/open-nerve/NerveWiki/server/internal/bootstrap"
	"github.com/open-nerve/NerveWiki/server/internal/platform/buildinfo"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// localConfigFile is the personal override file, relative to the working
// directory: `make run` starts nervewiki in server/.
const localConfigFile = "configs/config.local.yaml"

type configLoader func() (config.Config, error)

func newRootCommand(environ []string, stdin io.Reader) *cobra.Command {
	root := &cobra.Command{
		Use:           "nervewiki",
		Short:         "Nerve Wiki: a self-hosted team notes server",
		SilenceErrors: true, // run prints the error once
		SilenceUsage:  true,
	}
	root.CompletionOptions.DisableDefaultCmd = true

	load := func() (config.Config, error) {
		return config.Load(config.Sources{Embedded: configs.FS(), Environ: environ, LocalFile: localConfigFile})
	}
	root.AddCommand(newServeCommand(load), newMigrateCommand(load), newUsersCommand(load, stdin, realTerminal{}), newVersionCommand())
	return root
}

func newServeCommand(load configLoader) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP server until SIGINT or SIGTERM",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			return bootstrap.Serve(cmd.Context(), cfg, cmd.ErrOrStderr())
		},
	}
}

func newMigrateCommand(load configLoader) *cobra.Command {
	migrate := &cobra.Command{
		Use:   "migrate",
		Short: "Manage the database schema",
		// A runnable parent makes cobra validate its args, so a mistyped
		// subcommand fails instead of printing help and exiting 0.
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	sub := func(use, short string, action func(context.Context, config.Config, io.Writer) error) *cobra.Command {
		return &cobra.Command{
			Use:   use,
			Short: short,
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				cfg, err := load()
				if err != nil {
					return err
				}
				return action(cmd.Context(), cfg, cmd.OutOrStdout())
			},
		}
	}
	migrate.AddCommand(
		sub("up", "Apply all pending migrations, then check the database", bootstrap.MigrateUp),
		sub("down", "Roll back the most recent migration", bootstrap.MigrateDown),
		sub("status", "List migrations and whether they are applied", bootstrap.MigrateStatus),
	)
	return migrate
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the build version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := buildinfo.Get()
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "nervewiki %s commit=%s commit_time=%s modified=%t\n",
				info.Version, info.Commit, info.CommitTime, info.Modified)
			return err
		},
	}
}
