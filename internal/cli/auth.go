package cli

import (
	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewAuthCommand creates the "meta auth" command.
func NewAuthCommand(deps *Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:     "auth status",
		Short:   "Show authentication status",
		Long:    "Validate the current access token and show the authenticated user and permissions.",
		Aliases: []string{"auth status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			result, err := meta.AuthStatus(ctx, deps.Client)
			if err != nil {
				_ = output.PrintError(cmd.ErrOrStderr(), err)
				osExit(meta.ClassifyError(err))
				return nil
			}

			if err := output.Print(cmd.OutOrStdout(), result, deps.Format, deps.Fields); err != nil {
				_ = output.PrintError(cmd.ErrOrStderr(), err)
				osExit(meta.ExitAPIError)
			}

			return nil
		},
	}
}
