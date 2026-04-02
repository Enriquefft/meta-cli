package cli

import (
	"fmt"

	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewTargetingCommand creates the "meta targeting search" command.
func NewTargetingCommand(deps *Dependencies) *cobra.Command {
	var searchType string
	var query string
	var limit int

	cmd := &cobra.Command{
		Use:     "targeting search",
		Short:   "Search targeting options",
		Long:    "Search for targeting options (interests, behaviors, demographics, etc.).",
		Aliases: []string{"targeting"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if searchType == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--type is required"))
				osExit(meta.ExitValidationError)
				return nil
			}
			if query == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--query is required"))
				osExit(meta.ExitValidationError)
				return nil
			}

			ctx := cmd.Context()

			result, err := meta.SearchTargeting(ctx, deps.Client, meta.SearchTargetingParams{
				Type:  searchType,
				Query: query,
				Limit: limit,
			})
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

	cmd.Flags().StringVar(&searchType, "type", "", "Targeting type: interests, behaviors, demographics, etc. (required)")
	cmd.Flags().StringVar(&query, "query", "", "Search query string (required)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of results (default: 25)")

	return cmd
}
