package cli

import (
	"fmt"

	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewConfigCommand creates the "meta config" command with "set" and "get" subcommands.
func NewConfigCommand(deps *Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
		Long:  "Get and set configuration values stored in the config file.",
	}

	cmd.AddCommand(newConfigSetCommand(deps))
	cmd.AddCommand(newConfigGetCommand(deps))

	return cmd
}

func newConfigSetCommand(deps *Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long:  "Set a configuration key to the given value. Persists to the config file.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			if err := deps.Store.Set(key, value); err != nil {
				_ = output.PrintError(cmd.ErrOrStderr(), err)
				osExit(meta.ExitConfigError)
				return nil
			}

			result := map[string]string{
				"key":   key,
				"value": value,
			}

			if err := output.Print(cmd.OutOrStdout(), result, deps.Format, deps.Fields); err != nil {
				_ = output.PrintError(cmd.ErrOrStderr(), err)
				osExit(meta.ExitAPIError)
			}

			return nil
		},
	}
}

func newConfigGetCommand(deps *Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long:  "Get the value of a configuration key.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := deps.Store.Get(key)

			result := map[string]string{
				"key":   key,
				"value": value,
			}

			if err := output.Print(cmd.OutOrStdout(), result, deps.Format, deps.Fields); err != nil {
				_ = output.PrintError(cmd.ErrOrStderr(), err)
				osExit(meta.ExitAPIError)
			}

			return nil
		},
	}

	// Add a PostRun hook to hint if the value was empty.
	cmd.PostRun = func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := deps.Store.Get(key)
		if value == "" {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Note: key %q is not set\n", key)
		}
	}

	return cmd
}
