package cli

import (
	"fmt"

	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewAdsCommand creates the "meta ads create" command.
func NewAdsCommand(deps *Dependencies) *cobra.Command {
	var name string
	var adset string
	var creative string
	var status string

	cmd := &cobra.Command{
		Use:     "ads create",
		Short:   "Create a new ad",
		Long:    "Create a new ad under the configured ad account.",
		Aliases: []string{"ads"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--name is required"))
				osExit(meta.ExitValidationError)
				return nil
			}
			if adset == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--adset is required"))
				osExit(meta.ExitValidationError)
				return nil
			}
			if creative == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--creative is required"))
				osExit(meta.ExitValidationError)
				return nil
			}

			ctx := cmd.Context()
			accountID := AccountID(deps)

			result, err := meta.CreateAd(ctx, deps.Client, meta.CreateAdParams{
				AccountID:  accountID,
				Name:       name,
				AdSetID:    adset,
				CreativeID: creative,
				Status:     status,
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

	cmd.Flags().StringVar(&name, "name", "", "Ad name (required)")
	cmd.Flags().StringVar(&adset, "adset", "", "Ad set ID (required)")
	cmd.Flags().StringVar(&creative, "creative", "", "Creative ID (required)")
	cmd.Flags().StringVar(&status, "status", "PAUSED", "Initial ad status (default: PAUSED)")

	return cmd
}
