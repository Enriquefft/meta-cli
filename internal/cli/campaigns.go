package cli

import (
	"fmt"

	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewCampaignsCommand creates the "meta campaigns create" command.
func NewCampaignsCommand(deps *Dependencies) *cobra.Command {
	var name string
	var objective string
	var dailyBudget float64
	var lifetimeBudget float64
	var bidStrategy string
	var status string
	var specialAdCategory string

	cmd := &cobra.Command{
		Use:     "campaigns create",
		Short:   "Create a new campaign",
		Long:    "Create a new advertising campaign under the configured ad account.",
		Aliases: []string{"campaigns"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--name is required"))
				osExit(meta.ExitValidationError)
				return nil
			}
			if objective == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--objective is required"))
				osExit(meta.ExitValidationError)
				return nil
			}

			var dailyCents int64
			var lifetimeCents int64

			if dailyBudget > 0 {
				cents, err := meta.DollarsToCents(dailyBudget)
				if err != nil {
					_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("invalid daily-budget: %w", err))
					osExit(meta.ExitValidationError)
					return nil
				}
				dailyCents = cents
			}

			if lifetimeBudget > 0 {
				cents, err := meta.DollarsToCents(lifetimeBudget)
				if err != nil {
					_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("invalid lifetime-budget: %w", err))
					osExit(meta.ExitValidationError)
					return nil
				}
				lifetimeCents = cents
			}

			ctx := cmd.Context()
			accountID := AccountID(deps)

			result, err := meta.CreateCampaign(ctx, deps.Client, meta.CreateCampaignParams{
				AccountID:           accountID,
				Name:                name,
				Objective:           objective,
				DailyBudgetCents:    dailyCents,
				LifetimeBudgetCents: lifetimeCents,
				BidStrategy:         bidStrategy,
				Status:              status,
				SpecialAdCategory:   specialAdCategory,
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

	cmd.Flags().StringVar(&name, "name", "", "Campaign name (required)")
	cmd.Flags().StringVar(&objective, "objective", "", "Campaign objective, e.g. OUTCOME_SALES (required)")
	cmd.Flags().Float64Var(&dailyBudget, "daily-budget", 0, "Daily budget in dollars")
	cmd.Flags().Float64Var(&lifetimeBudget, "lifetime-budget", 0, "Lifetime budget in dollars")
	cmd.Flags().StringVar(&bidStrategy, "bid-strategy", "", "Bid strategy (default: LOWEST_COST_WITHOUT_CAP)")
	cmd.Flags().StringVar(&status, "status", "PAUSED", "Initial campaign status (default: PAUSED)")
	cmd.Flags().StringVar(&specialAdCategory, "special-ad-category", "", "Special ad category (default: NONE)")

	return cmd
}
