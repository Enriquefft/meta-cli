package main

import (
	"context"
	"os"

	"github.com/enriquefft/meta-cli/internal/cli"
	"github.com/enriquefft/meta-cli/internal/config"
	"github.com/enriquefft/meta-cli/internal/graph"
)

var version = "dev"

func main() {
	cli.Version = version

	store := config.NewStore("")

	cfg, err := store.Load()
	if err != nil {
		os.Exit(1)
	}

	graphClient := graph.NewClient(graph.ClientConfig{
		AccessToken: cfg.AccessToken,
		APIVersion:  cfg.APIVersion,
	})

	if err := cli.Execute(context.Background(), store, graphClient); err != nil {
		os.Exit(1)
	}
}
