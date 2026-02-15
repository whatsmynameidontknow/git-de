package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/whatsmynameidontknow/git-de/internal/cli"
	"github.com/whatsmynameidontknow/git-de/internal/exporter"
	"github.com/whatsmynameidontknow/git-de/internal/git"
	"github.com/whatsmynameidontknow/git-de/internal/tui"
	"golang.org/x/term"
)

var version string

func init() {
	if version == "" {
		version = "dev"
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			version = strings.TrimPrefix(bi.Main.Version, "v")
		}
	}
}

func main() {
	config, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if config.ShowVersion {
		fmt.Printf("Git Diff Export version %s\n", version)
		return
	}

	client := git.NewClient("")

	// Check if we're in a git repository
	if !client.IsGitRepository() {
		fmt.Fprintf(os.Stderr, "Error: not a git repository\n")
		os.Exit(1)
	}

	// Determine if we should use TUI mode
	useTUI := shouldUseTUI(config)

	if useTUI {
		if err := tui.Run(client, config.FromCommit, config.ToCommit); err != nil {
			fmt.Fprintf(os.Stderr, "TUI Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// CLI mode
	if config.FromCommit == "" {
		fmt.Fprintf(os.Stderr, "Error: from-commit is required (or use --tui for interactive mode)\n")
		os.Exit(1)
	}

	if config.ToCommit == "" {
		config.ToCommit = "HEAD"
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
	}

	exp := exporter.New(client, opts)

	if err := exp.Export(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// shouldUseTUI determines whether to launch the TUI based on configuration and environment
func shouldUseTUI(config *cli.Config) bool {
	return shouldUseTUIWithOverride(config, term.IsTerminal(int(os.Stdin.Fd())))
}

// shouldUseTUIWithOverride is the testable version that accepts an explicit TTY flag
func shouldUseTUIWithOverride(config *cli.Config, isTTY bool) bool {
	// Explicit --no-tui flag forces CLI mode
	if config.NoTUI {
		return false
	}

	// Auto-detect: if not in a terminal, always use CLI
	if !isTTY {
		return false
	}

	// If in a terminal, use CLI only if an output destination is specified
	// (user wants to "just do it"). Otherwise, show TUI (interactive preview).
	if config.FromCommit != "" && (config.OutputDir != "" || config.ArchivePath != "") {
		return false
	}

	return true
}
