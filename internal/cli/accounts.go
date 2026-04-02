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

// NewAccountsCommand creates the "meta accounts list" command.
func NewAccountsCommand(deps *Dependencies) *cobra.Command {
	var limit int
	var fetchAll bool

	cmd := &cobra.Command{
		Use:     "accounts list",
		Short:   "List ad accounts",
		Long:    "List all ad accounts accessible by the authenticated user.",
		Aliases: []string{"accounts"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var result *meta.ListAccountsResult
			var err error

			if fetchAll {
				result, err = listAllAccounts(ctx, deps.Client, limit)
			} else {
				result, err = meta.ListAccounts(ctx, deps.Client, meta.ListAccountsParams{
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

	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of accounts per page")
	cmd.Flags().BoolVar(&fetchAll, "all", false, "Fetch all pages of results")

	return cmd
}

// listAllAccounts paginates through all ad account pages using PageIterator.
func listAllAccounts(ctx context.Context, client meta.Client, limit int) (*meta.ListAccountsResult, error) {
	if limit <= 0 {
		limit = 25
	}

	qp := url.Values{}
	qp.Set("fields", "account_id,name,account_status,currency,timezone_name,amount_spent,balance")
	qp.Set("limit", strconv.Itoa(limit))

	it := client.Paginate(ctx, "/me/adaccounts", qp)

	var allAccounts []meta.AdAccount
	for it.Next(ctx) {
		page, err := it.Page()
		if err != nil {
			return nil, err
		}

		if ge := meta.ParseGraphError(page.Body); ge != nil {
			return nil, ge
		}

		var raw struct {
			Data []meta.AdAccount `json:"data"`
		}
		if err := json.Unmarshal(page.Body, &raw); err != nil {
			return nil, err
		}
		allAccounts = append(allAccounts, raw.Data...)
	}
	if err := it.Err(); err != nil {
		return nil, err
	}

	if allAccounts == nil {
		allAccounts = []meta.AdAccount{}
	}

	return &meta.ListAccountsResult{
		Data:    allAccounts,
		HasNext: false,
	}, nil
}
