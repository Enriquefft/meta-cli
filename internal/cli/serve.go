package cli

import (
	"context"

	"github.com/enriquefft/meta-cli/internal/mcp"
	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewServeCommand creates the "meta serve" command that starts the MCP server.
func NewServeCommand(deps *Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server",
		Long:  "Start the MCP (Model Context Protocol) server for AI tool integration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := runMCPServer(ctx, deps); err != nil {
				_ = output.PrintError(cmd.ErrOrStderr(), err)
				osExit(meta.ClassifyError(err))
				return nil
			}

			return nil
		},
	}
}

// runMCPServer starts the MCP server via the internal/mcp package.
func runMCPServer(ctx context.Context, deps *Dependencies) error {
	return mcp.RunServer(ctx, deps.Client)
}
