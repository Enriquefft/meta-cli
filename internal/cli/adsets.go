package cli

import (
	"fmt"

	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewAdSetsCommand creates the "meta adsets create" command.
func NewAdSetsCommand(deps *Dependencies) *cobra.Command {
	var name string
	var campaign string
	var dailyBudget float64
	var lifetimeBudget float64
	var optimizationGoal string
	var billingEvent string
	var bidAmount float64
	var countries []string
	var ageMin int
	var ageMax int
	var genders []int
	var interests []string
	var behaviors []string
	var customAudiences []string
	var excludedCountries []string
	var publisherPlatforms []string
	var pixelID string
	var customEventType string
	var startTime string
	var endTime string
	var advantageAudience bool
	var status string

	cmd := &cobra.Command{
		Use:     "adsets create",
		Short:   "Create a new ad set",
		Long:    "Create a new ad set under the configured ad account with targeting parameters.",
		Aliases: []string{"adsets"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--name is required"))
				osExit(meta.ExitValidationError)
				return nil
			}
			if campaign == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--campaign is required"))
				osExit(meta.ExitValidationError)
				return nil
			}
			if optimizationGoal == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--optimization-goal is required"))
				osExit(meta.ExitValidationError)
				return nil
			}
			if len(countries) == 0 {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--countries is required"))
				osExit(meta.ExitValidationError)
				return nil
			}

			var dailyCents int64
			var lifetimeCents int64
			var bidCents int64

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

			if bidAmount > 0 {
				cents, err := meta.DollarsToCents(bidAmount)
				if err != nil {
					_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("invalid bid-amount: %w", err))
					osExit(meta.ExitValidationError)
					return nil
				}
				bidCents = cents
			}

			ctx := cmd.Context()
			accountID := AccountID(deps)

			result, err := meta.CreateAdSet(ctx, deps.Client, meta.CreateAdSetParams{
				AccountID:           accountID,
				Name:                name,
				CampaignID:          campaign,
				DailyBudgetCents:    dailyCents,
				LifetimeBudgetCents: lifetimeCents,
				OptimizationGoal:    optimizationGoal,
				BillingEvent:        billingEvent,
				BidAmountCents:      bidCents,
				Countries:           countries,
				AgeMin:              ageMin,
				AgeMax:              ageMax,
				Genders:             genders,
				Interests:           interests,
				Behaviors:           behaviors,
				CustomAudiences:     customAudiences,
				ExcludedCountries:   excludedCountries,
				PublisherPlatforms:  publisherPlatforms,
				PixelID:             pixelID,
				CustomEventType:     customEventType,
				StartTime:           startTime,
				EndTime:             endTime,
				AdvantageAudience:   advantageAudience,
				Status:              status,
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

	cmd.Flags().StringVar(&name, "name", "", "Ad set name (required)")
	cmd.Flags().StringVar(&campaign, "campaign", "", "Campaign ID (required)")
	cmd.Flags().Float64Var(&dailyBudget, "daily-budget", 0, "Daily budget in dollars")
	cmd.Flags().Float64Var(&lifetimeBudget, "lifetime-budget", 0, "Lifetime budget in dollars")
	cmd.Flags().StringVar(&optimizationGoal, "optimization-goal", "", "Optimization goal (required)")
	cmd.Flags().StringVar(&billingEvent, "billing-event", "", "Billing event (default: IMPRESSIONS)")
	cmd.Flags().Float64Var(&bidAmount, "bid-amount", 0, "Bid amount in dollars")
	cmd.Flags().StringSliceVar(&countries, "countries", nil, "Target countries, e.g. US,GB (required)")
	cmd.Flags().IntVar(&ageMin, "age-min", 0, "Minimum age (default: 18)")
	cmd.Flags().IntVar(&ageMax, "age-max", 0, "Maximum age (default: 65)")
	cmd.Flags().IntSliceVar(&genders, "genders", nil, "Target genders: 1=male, 2=female (default: [1,2])")
	cmd.Flags().StringSliceVar(&interests, "interests", nil, "Interest IDs to target")
	cmd.Flags().StringSliceVar(&behaviors, "behaviors", nil, "Behavior IDs to target")
	cmd.Flags().StringSliceVar(&customAudiences, "custom-audiences", nil, "Custom audience IDs")
	cmd.Flags().StringSliceVar(&excludedCountries, "excluded-countries", nil, "Countries to exclude")
	cmd.Flags().StringSliceVar(&publisherPlatforms, "publisher-platforms", nil, "Publisher platforms, e.g. facebook,instagram")
	cmd.Flags().StringVar(&pixelID, "pixel-id", "", "Pixel ID for conversion tracking")
	cmd.Flags().StringVar(&customEventType, "custom-event-type", "", "Custom event type for pixel")
	cmd.Flags().StringVar(&startTime, "start-time", "", "Start time (ISO 8601)")
	cmd.Flags().StringVar(&endTime, "end-time", "", "End time (ISO 8601)")
	cmd.Flags().BoolVar(&advantageAudience, "advantage-audience", false, "Enable Advantage+ audience expansion")
	cmd.Flags().StringVar(&status, "status", "PAUSED", "Initial ad set status (default: PAUSED)")

	return cmd
}
