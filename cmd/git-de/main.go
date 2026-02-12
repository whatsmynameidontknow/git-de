package main

import (
	"fmt"
	"os"

	"github.com/whatsmynameidontknow/git-de/internal/cli"
	"github.com/whatsmynameidontknow/git-de/internal/exporter"
	"github.com/whatsmynameidontknow/git-de/internal/git"
	"github.com/whatsmynameidontknow/git-de/internal/tui"
)

func main() {
	config, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	client := git.NewClient("")

	if config.TUI {
		if err := tui.Run(client, config.FromCommit, config.ToCommit); err != nil {
			fmt.Fprintf(os.Stderr, "TUI Error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	
	opts := exporter.Options{
		FromCommit:      config.FromCommit,
		ToCommit:        config.ToCommit,
		OutputDir:       config.OutputDir,
		Overwrite:       config.Overwrite,
		Concurrent:      config.Concurrent,
		Preview:         config.Preview,
		Verbose:         config.Verbose,
		IgnorePatterns:  config.IgnorePatterns,
		IncludePatterns: config.IncludePatterns,
		MaxSize:         config.MaxSize,
		ArchivePath:     config.ArchivePath,
		JSON:            config.JSON,
		JSONFile:        config.JSONFile,
	}

	exp := exporter.New(client, opts)
	
	if err := exp.Export(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
