package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/enriquefft/meta-cli/internal/updater"
	"github.com/spf13/cobra"
)

// Indirection seams for tests. The update command depends on several
// side-effectful operations (constructing an updater, reading the executable
// path, resolving symlinks, classifying the install method). Routing every
// call through a package-level var lets update_test.go substitute fakes
// without touching production code paths — this mirrors the osExit pattern
// established in syscalls.go.
var (
	updateNewUpdater   = updater.New
	updateExecutable   = os.Executable
	updateEvalSymlinks = filepath.EvalSymlinks
	updateDetectMethod = updater.DetectInstallMethod
)

// NewUpdateCommand creates the "meta update" command.
//
// The command deliberately does not reimplement any installer logic. It
// inspects how meta-cli was installed, queries GitHub for the latest
// release, and — for script-installed binaries — downloads and executes
// install.sh pinned to the target tag. install.sh is the single source of
// truth for downloading, checksum verification, extraction, and install
// location; the CLI never duplicates any of that.
func NewUpdateCommand(deps *Dependencies) *cobra.Command {
	var checkOnly bool
	var force bool
	var pinVersion string
	var apiBaseURL string
	var rawBaseURL string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update meta-cli to the latest release",
		Long: "Check for and install the latest published release of meta-cli.\n\n" +
			"Delegates to the official install.sh script pinned to the target tag, " +
			"so the upgrade procedure is identical to a fresh install (same checksums, " +
			"same install directory).",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			up := updateNewUpdater(updater.Config{
				APIBaseURL: apiBaseURL,
				RawBaseURL: rawBaseURL,
			})

			// Resolve which version to install. If the user passed --version
			// we trust them and skip the "latest" lookup entirely; otherwise
			// compare the running binary against the GitHub latest release.
			var targetVersion string
			var status updater.Status
			if pinVersion != "" {
				targetVersion = updater.StripLeadingV(pinVersion)
			} else {
				var err error
				status, err = up.Check(ctx, Version)
				if err != nil {
					_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("checking for updates: %w", err))
					osExit(meta.ExitNetworkError)
					return nil
				}

				if checkOnly {
					return output.Print(cmd.OutOrStdout(), status, deps.Format, deps.Fields)
				}

				if status.UpToDate && !force {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "meta %s is already the latest version\n", Version)
					return nil
				}
				targetVersion = status.Latest
			}

			// Resolve the current binary's install directory from its own
			// path. EvalSymlinks is important here: on systems where the
			// user symlinks ~/.local/bin/meta into /usr/local/bin, we must
			// point install.sh at the real directory so the new binary
			// overwrites the old one instead of the symlink.
			execPath, err := updateExecutable()
			if err != nil {
				_ = output.PrintError(cmd.ErrOrStderr(), fmt.Errorf("resolving executable path: %w", err))
				osExit(meta.ExitConfigError)
				return nil
			}
			resolvedPath, err := updateEvalSymlinks(execPath)
			if err != nil {
				// If the binary is reachable but its symlink chain cannot
				// be walked (for example, a transient filesystem issue),
				// fall back to the unresolved path — that is strictly
				// better than failing the whole upgrade.
				resolvedPath = execPath
			}
			installDir := filepath.Dir(resolvedPath)

			// Decide how to upgrade based on how the binary was installed.
			method := updateDetectMethod(resolvedPath)
			switch method {
			case updater.MethodWindows:
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "meta update is not supported on Windows. Download a release manually from https://github.com/enriquefft/meta-cli/releases")
				osExit(meta.ExitConfigError)
				return nil
			case updater.MethodGoInstall:
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "meta-cli was installed via 'go install'.")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Upgrade with: go install github.com/enriquefft/meta-cli/cmd/meta@latest")
				return nil
			case updater.MethodScript, updater.MethodUnknown:
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updating meta to v%s in %s...\n", targetVersion, installDir)
				if err := up.RunInstallScript(ctx, targetVersion, installDir); err != nil {
					_ = output.PrintError(cmd.ErrOrStderr(), err)
					osExit(meta.ExitAPIError)
					return nil
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "meta v%s installed\n", targetVersion)
				return nil
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "Check for a new version without installing")
	cmd.Flags().BoolVar(&force, "force", false, "Reinstall even if already on the latest version")
	cmd.Flags().StringVar(&pinVersion, "version", "", "Pin to a specific version (e.g. 0.3.0)")
	cmd.Flags().StringVar(&apiBaseURL, "github-api-base-url", "", "Override GitHub API base URL (testing)")
	cmd.Flags().StringVar(&rawBaseURL, "github-raw-base-url", "", "Override GitHub raw base URL (testing)")
	_ = cmd.Flags().MarkHidden("github-api-base-url")
	_ = cmd.Flags().MarkHidden("github-raw-base-url")

	// --check ("tell me if a new version exists") and --version ("install
	// this exact tag") express opposing intents — combining them used to
	// silently install the pinned version. Reject the combination at parse
	// time so the user gets an immediate error instead of unexpected work.
	cmd.MarkFlagsMutuallyExclusive("check", "version")

	return cmd
}
