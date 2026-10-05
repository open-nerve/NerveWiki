package main

import (
	"fmt"
	"uuid"

	"github.com/spf13/cobra"

	"github.com/open-nerve/NerveWiki/server/internal/bootstrap"
)

// newReindexCommand is `nervewiki reindex` (M6/P3 design 3.6): it rebuilds
// the link index of every notebook, or of the one --notebook names. Run it
// once after an upgrade that asks for it; a notebook's writes wait while
// its index is rebuilt.
func newReindexCommand(load configLoader) *cobra.Command {
	var notebook string
	cmd := &cobra.Command{
		Use:   "reindex [--notebook <id>]",
		Short: "Rebuild the link index of every notebook, or of one",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var id uuid.UUID
			if cmd.Flags().Changed("notebook") {
				var err error
				if id, err = uuid.Parse(notebook); err != nil || id == uuid.Nil() {
					return fmt.Errorf("--notebook %q is not a notebook id", notebook)
				}
			}
			cfg, err := load()
			if err != nil {
				return err
			}
			return bootstrap.Reindex(cmd.Context(), cfg, cmd.ErrOrStderr(), cmd.OutOrStdout(), id)
		},
	}
	cmd.Flags().StringVar(&notebook, "notebook", "", "the id of the one notebook to reindex; every notebook without it")
	return cmd
}
