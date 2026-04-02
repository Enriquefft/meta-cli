package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/enriquefft/meta-cli/internal/config"
	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// Dependencies holds the shared state for all CLI subcommands.
// Constructed once in Execute() and passed to each subcommand factory.
type Dependencies struct {
	Client  meta.Client
	Config  *config.Config
	Store   *config.ConfigStore
	Format  string // resolved output format (--format or config)
	Fields  string // --fields value
	DryRun  bool
	Verbose bool
}

// Execute is the main entry point for the CLI.
// It loads config, wires subcommands, and runs.
// The client is provided from the composition root (cmd/meta/main.go).
func Execute(ctx context.Context, store *config.ConfigStore, client meta.Client) error {
	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	rootCmd := &cobra.Command{
		Use:           "meta",
		Short:         "CLI and MCP server for the Meta Marketing API",
		Long:          "meta -- CLI and MCP server for the Meta Marketing API.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
	}

	// Global flags resolved via PersistentFlags so subcommands inherit them.
	var format string
	var fields string
	var dryRun bool
	var verbose bool
	var noColor bool
	var account string

	rootCmd.PersistentFlags().StringVarP(&account, "account", "a", "", "Meta ad account ID (overrides config)")
	rootCmd.PersistentFlags().StringVarP(&format, "format", "f", "", "Output format: json, table, csv (overrides config)")
	rootCmd.PersistentFlags().StringVar(&fields, "fields", "", "Comma-separated field names to include in output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Validate-only mode for write operations")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	// Build deps with zero values — will be populated by PersistentPreRunE after flags are parsed.
	deps := &Dependencies{
		Client: client,
		Config: cfg,
		Store:  store,
	}

	// Register all subcommands.
	rootCmd.AddCommand(
		NewConfigCommand(deps),
		NewAuthCommand(deps),
		NewAccountsCommand(deps),
		NewPagesCommand(deps),
		NewAssetsCommand(deps),
		NewCampaignsCommand(deps),
		NewAdSetsCommand(deps),
		NewCreativesCommand(deps),
		NewAdsCommand(deps),
		NewTargetingCommand(deps),
		NewServeCommand(deps),
	)

	// Resolve flags AFTER Cobra parses them, BEFORE any subcommand runs.
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		effectiveFormat := format
		if effectiveFormat == "" {
			effectiveFormat = cfg.OutputFormat
		}

		effectiveAccount := account
		if effectiveAccount == "" {
			effectiveAccount = cfg.DefaultAccount
		}

		client.SetDryRun(dryRun)
		client.SetVerbose(verbose)

		resolvedCfg := *cfg
		resolvedCfg.DefaultAccount = effectiveAccount

		deps.Config = &resolvedCfg
		deps.Format = effectiveFormat
		deps.Fields = fields
		deps.DryRun = dryRun
		deps.Verbose = verbose

		return nil
	}

	return rootCmd.ExecuteContext(ctx)
}

// HandleError prints the error to stderr and exits with the appropriate code.
// This is the standard error handling pattern for all CLI commands.
func HandleError(err error) {
	_ = output.PrintError(os.Stderr, err)
	os.Exit(meta.ClassifyError(err))
}

// AccountID returns the effective account ID from deps.
func AccountID(deps *Dependencies) string {
	if deps == nil || deps.Config == nil {
		return ""
	}
	return deps.Config.DefaultAccount
}
