package cli

import (
	"fmt"
	"os"

	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewAssetsCommand creates the "meta assets" command with subcommands.
func NewAssetsCommand(deps *Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assets",
		Short: "Manage ad assets (videos)",
		Long:  "Upload videos and check their encoding status.",
	}

	cmd.AddCommand(newUploadVideoCommand(deps))
	cmd.AddCommand(newVideoStatusCommand(deps))

	return cmd
}

func newUploadVideoCommand(deps *Dependencies) *cobra.Command {
	var filePath string
	var title string

	cmd := &cobra.Command{
		Use:   "upload-video",
		Short: "Upload a video to an ad account",
		Long:  "Upload a video file to the configured ad account for use in ad creatives.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("--file is required"))
				osExit(meta.ExitValidationError)
				return nil
			}

			f, err := os.Open(filePath)
			if err != nil {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("opening file: %w", err))
				osExit(meta.ExitValidationError)
				return nil
			}
			defer func() { _ = f.Close() }()

			fi, err := f.Stat()
			if err != nil {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("stat file: %w", err))
				osExit(meta.ExitValidationError)
				return nil
			}

			ctx := cmd.Context()
			accountID := AccountID(deps)

			result, err := meta.UploadVideo(ctx, deps.Client, meta.UploadVideoParams{
				AccountID: accountID,
				File:      f,
				Filename:  fi.Name(),
				FileSize:  fi.Size(),
				Title:     title,
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

	cmd.Flags().StringVar(&filePath, "file", "", "Path to video file (required)")
	cmd.Flags().StringVar(&title, "title", "", "Video title")

	return cmd
}

func newVideoStatusCommand(deps *Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "video-status <video-id>",
		Short: "Check video encoding status",
		Long:  "Check the encoding status of a previously uploaded video.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoID := args[0]
			ctx := cmd.Context()

			result, err := meta.GetVideoStatus(ctx, deps.Client, meta.VideoStatusParams{
				VideoID: videoID,
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
}
