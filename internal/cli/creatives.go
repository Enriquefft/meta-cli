package cli

import (
	"fmt"

	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewCreativesCommand creates the "meta creatives create" command.
func NewCreativesCommand(deps *Dependencies) *cobra.Command {
	var name string
	var page string
	var video string
	var imageHash string
	var imageURL string
	var message string
	var headline string
	var description string
	var cta string
	var link string
	var instagramAccount string

	cmd := &cobra.Command{
		Use:     "creatives create",
		Short:   "Create a new ad creative",
		Long:    "Create a new ad creative under the configured ad account.",
		Aliases: []string{"creatives"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if page == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--page is required"))
				osExit(meta.ExitValidationError)
				return nil
			}
			if message == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--message is required"))
				osExit(meta.ExitValidationError)
				return nil
			}
			if link == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--link is required"))
				osExit(meta.ExitValidationError)
				return nil
			}

			ctx := cmd.Context()
			accountID := AccountID(deps)

			result, err := meta.CreateCreative(ctx, deps.Client, meta.CreateCreativeParams{
				AccountID:          accountID,
				Name:               name,
				PageID:             page,
				VideoID:            video,
				ImageHash:          imageHash,
				ImageURL:           imageURL,
				Message:            message,
				Headline:           headline,
				Description:        description,
				CTA:                cta,
				Link:               link,
				InstagramAccountID: instagramAccount,
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

	cmd.Flags().StringVar(&name, "name", "", "Creative name")
	cmd.Flags().StringVar(&page, "page", "", "Facebook Page ID (required)")
	cmd.Flags().StringVar(&video, "video", "", "Video ID for video creative")
	cmd.Flags().StringVar(&imageHash, "image-hash", "", "Image hash for image creative")
	cmd.Flags().StringVar(&imageURL, "image-url", "", "Image URL for image creative")
	cmd.Flags().StringVar(&message, "message", "", "Ad message text (required)")
	cmd.Flags().StringVar(&headline, "headline", "", "Ad headline")
	cmd.Flags().StringVar(&description, "description", "", "Ad description")
	cmd.Flags().StringVar(&cta, "cta", "SHOP_NOW", "Call-to-action type (default: SHOP_NOW)")
	cmd.Flags().StringVar(&link, "link", "", "Destination URL (required)")
	cmd.Flags().StringVar(&instagramAccount, "instagram-account", "", "Instagram account ID for cross-posting")

	return cmd
}
