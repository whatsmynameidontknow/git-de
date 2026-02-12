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

	client := git.NewClient("")
	
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
	}

	exp := exporter.New(client, opts)
	
	if err := exp.Export(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
