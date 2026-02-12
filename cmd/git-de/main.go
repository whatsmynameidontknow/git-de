package main

import (
	"fmt"
	"os"

	"github.com/whatsmynameidontknow/git-de/internal/cli"
	"github.com/whatsmynameidontknow/git-de/internal/exporter"
	"github.com/whatsmynameidontknow/git-de/internal/git"
)

func main() {
	config, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if config.OutputDir == "" {
		fmt.Fprintf(os.Stderr, "Error: output directory is required (use -o or --output)\n")
		os.Exit(1)
	}

	client := git.NewClient("")
	
	opts := exporter.Options{
		FromCommit: config.FromCommit,
		ToCommit:   config.ToCommit,
		OutputDir:  config.OutputDir,
		Overwrite:  config.Overwrite,
		Concurrent: config.Concurrent,
	}

	exp := exporter.New(client, opts)
	
	if err := exp.Export(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
