package cli

import (
	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewPagesCommand creates the "meta pages list" command.
//
// All business logic (multi-source aggregation, pagination, deduplication,
// partial-failure handling) lives in meta.ListPages. This command is a thin
// shell: parse flags, call the domain, format output.
func NewPagesCommand(deps *Dependencies) *cobra.Command {
	var limit int
	var fetchAll bool

	cmd := &cobra.Command{
		Use:     "pages list",
		Short:   "List Facebook Pages",
		Long:    "List all Facebook Pages the authenticated user can access, including those reached through Business Manager.",
		Aliases: []string{"pages"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			result, err := meta.ListPages(ctx, deps.Client, meta.ListPagesParams{
				Limit: limit,
				All:   fetchAll,
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

	cmd.Flags().IntVar(&limit, "limit", 25, "Per-request page size (forwarded to every underlying Graph call)")
	cmd.Flags().BoolVar(&fetchAll, "all", false, "Exhaustively paginate every source")

	return cmd
}
