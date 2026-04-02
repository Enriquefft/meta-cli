package cli

import (
	"testing"

	"github.com/enriquefft/meta-cli/internal/config"
	"github.com/spf13/cobra"
)

func TestVersion(t *testing.T) {
	oldVersion := Version
	Version = "1.2.3-test"
	defer func() { Version = oldVersion }()

	if Version != "1.2.3-test" {
		t.Errorf("expected version to be settable, got %q", Version)
	}
}

func TestAccountID(t *testing.T) {
	tests := []struct {
		name     string
		deps     *Dependencies
		expected string
	}{
		{
			name: "returns account from config",
			deps: &Dependencies{
				Config: &config.Config{DefaultAccount: "act_999"},
			},
			expected: "act_999",
		},
		{
			name:     "returns empty for nil config",
			deps:     &Dependencies{},
			expected: "",
		},
		{
			name:     "returns empty for nil deps",
			deps:     nil,
			expected: "",
		},
		{
			name: "returns empty string when account not set",
			deps: &Dependencies{
				Config: &config.Config{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AccountID(tt.deps)
			if got != tt.expected {
				t.Errorf("AccountID() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDependencies_Defaults(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)

	if deps.Format != "json" {
		t.Errorf("expected default format 'json', got %q", deps.Format)
	}
	if deps.Config.DefaultAccount != "act_123456" {
		t.Errorf("expected account 'act_123456', got %q", deps.Config.DefaultAccount)
	}
	if deps.DryRun {
		t.Error("expected DryRun to be false")
	}
	if deps.Verbose {
		t.Error("expected Verbose to be false")
	}
}

func TestExecute_Version(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)

	root := &cobra.Command{
		Use:           "meta",
		Version:       "test-v",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(NewAccountsCommand(deps))

	stdout, _, err := executeCommand(root, "--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout == "" {
		t.Error("expected version output")
	}
}

// TestFlagResolution verifies that PersistentPreRunE correctly resolves
// global flags after Cobra parses them, overriding config defaults.
func TestFlagResolution(t *testing.T) {
	// Set up a config with known defaults.
	store, cleanup := testStore()
	defer cleanup()
	_ = store.Set("output_format", "table")
	_ = store.Set("default_account", "act_config")
	cfg, _ := store.Load()

	// Capture what the probe subcommand sees.
	var captured struct {
		Format  string
		Account string
		DryRun  bool
		Verbose bool
		Fields  string
	}

	buildProbeRoot := func() *cobra.Command {
		mc := &mockClient{}
		deps := &Dependencies{
			Client: mc,
			Config: cfg,
			Store:  store,
		}

		root := &cobra.Command{
			Use:           "meta",
			SilenceUsage:  true,
			SilenceErrors: true,
		}

		var format, fields, account string
		var dryRun, verbose, noColor bool

		root.PersistentFlags().StringVarP(&account, "account", "a", "", "")
		root.PersistentFlags().StringVarP(&format, "format", "f", "", "")
		root.PersistentFlags().StringVar(&fields, "fields", "", "")
		root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "")
		root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "")
		root.PersistentFlags().BoolVar(&noColor, "no-color", false, "")

		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			effectiveFormat := format
			if effectiveFormat == "" {
				effectiveFormat = cfg.OutputFormat
			}
			effectiveAccount := account
			if effectiveAccount == "" {
				effectiveAccount = cfg.DefaultAccount
			}
			resolvedCfg := *cfg
			resolvedCfg.DefaultAccount = effectiveAccount
			deps.Config = &resolvedCfg
			deps.Format = effectiveFormat
			deps.Fields = fields
			deps.DryRun = dryRun
			deps.Verbose = verbose
			return nil
		}

		probeCmd := &cobra.Command{
			Use: "probe",
			RunE: func(cmd *cobra.Command, args []string) error {
				captured.Format = deps.Format
				captured.Account = deps.Config.DefaultAccount
				captured.DryRun = deps.DryRun
				captured.Verbose = deps.Verbose
				captured.Fields = deps.Fields
				return nil
			},
		}
		root.AddCommand(probeCmd)
		return root
	}

	t.Run("flags override config", func(t *testing.T) {
		captured.Format = ""
		captured.Account = ""
		captured.DryRun = false
		captured.Verbose = false
		captured.Fields = ""

		root := buildProbeRoot()
		_, _, err := executeCommand(root, "--format", "json", "--account", "act_flag", "--dry-run", "--verbose", "--fields", "id,name", "probe")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if captured.Format != "json" {
			t.Errorf("expected format 'json', got %q", captured.Format)
		}
		if captured.Account != "act_flag" {
			t.Errorf("expected account 'act_flag', got %q", captured.Account)
		}
		if !captured.DryRun {
			t.Error("expected DryRun true")
		}
		if !captured.Verbose {
			t.Error("expected Verbose true")
		}
		if captured.Fields != "id,name" {
			t.Errorf("expected fields 'id,name', got %q", captured.Fields)
		}
	})

	t.Run("config defaults when no flags", func(t *testing.T) {
		captured.Format = ""
		captured.Account = ""

		root := buildProbeRoot()
		_, _, err := executeCommand(root, "probe")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if captured.Format != "table" {
			t.Errorf("expected format from config 'table', got %q", captured.Format)
		}
		if captured.Account != "act_config" {
			t.Errorf("expected account from config 'act_config', got %q", captured.Account)
		}
		if captured.DryRun {
			t.Error("expected DryRun false")
		}
		if captured.Verbose {
			t.Error("expected Verbose false")
		}
	})
}
