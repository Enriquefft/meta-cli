package cli

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewPagesCommand creates the "meta pages list" command.
func NewPagesCommand(deps *Dependencies) *cobra.Command {
	var limit int
	var fetchAll bool

	cmd := &cobra.Command{
		Use:     "pages list",
		Short:   "List Facebook Pages",
		Long:    "List all Facebook Pages managed by the authenticated user.",
		Aliases: []string{"pages"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var result *meta.ListPagesResult
			var err error

			if fetchAll {
				result, err = listAllPages(ctx, deps.Client, limit)
			} else {
				result, err = meta.ListPages(ctx, deps.Client, meta.ListPagesParams{
					Limit: limit,
				})
			}

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

	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of pages per page")
	cmd.Flags().BoolVar(&fetchAll, "all", false, "Fetch all pages of results")

	return cmd
}

// listAllPages paginates through all Facebook Pages using PageIterator.
func listAllPages(ctx context.Context, client meta.Client, limit int) (*meta.ListPagesResult, error) {
	if limit <= 0 {
		limit = 25
	}

	qp := url.Values{}
	qp.Set("fields", "id,name,category,access_token")
	qp.Set("limit", strconv.Itoa(limit))

	it := client.Paginate(ctx, "/me/accounts", qp)

	var allPages []meta.Page
	for it.Next(ctx) {
		page, err := it.Page()
		if err != nil {
			return nil, err
		}

		if ge := meta.ParseGraphError(page.Body); ge != nil {
			return nil, ge
		}

		var raw struct {
			Data []meta.Page `json:"data"`
		}
		if err := json.Unmarshal(page.Body, &raw); err != nil {
			return nil, err
		}
		allPages = append(allPages, raw.Data...)
	}
	if err := it.Err(); err != nil {
		return nil, err
	}

	if allPages == nil {
		allPages = []meta.Page{}
	}

	return &meta.ListPagesResult{
		Data:    allPages,
		HasNext: false,
	}, nil
}
