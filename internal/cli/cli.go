package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

type Config struct {
	FromCommit      string
	ToCommit        string
	OutputDir       string
	Overwrite       bool
	Concurrent      bool
	Preview         bool
	Verbose         bool
	IgnorePatterns  []string
	IncludePatterns []string
}

func Parse(args []string) (*Config, error) {
	var config Config

	pflag.StringVarP(&config.FromCommit, "from", "f", "", "Starting commit")
	pflag.StringVarP(&config.ToCommit, "to", "t", "", "Ending commit (defaults to HEAD)")
	pflag.StringVarP(&config.OutputDir, "output", "o", "", "Output directory")
	pflag.BoolVarP(&config.Overwrite, "overwrite", "w", false, "Overwrite existing output directory")
	pflag.BoolVarP(&config.Concurrent, "concurrent", "c", false, "Copy files concurrently")
	pflag.BoolVarP(&config.Verbose, "verbose", "v", false, "Enable verbose output")
	pflag.StringArrayVarP(&config.IgnorePatterns, "ignore", "i", nil, "Ignore patterns (comma-separated or multiple flags)")
	pflag.StringArrayVarP(&config.IncludePatterns, "include", "I", nil, "Include patterns - only export files matching these (comma-separated or multiple flags)")

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: git-de [options] <from-commit> [<to-commit>]

Export files changed between Git commits.

Arguments:
  from-commit    Starting commit (required)
  to-commit      Ending commit (defaults to HEAD)

Options:
  -f, --from string       Starting commit (alternative to positional)
  -t, --to string         Ending commit (defaults to HEAD)
  -o, --output string     Output directory (optional, runs in preview mode if not set)
  -w, --overwrite         Overwrite existing output directory
  -c, --concurrent        Copy files concurrently
  -v, --verbose           Enable verbose output
  -i, --ignore string     Ignore patterns (comma-separated or multiple flags)
  -I, --include string    Include patterns - only export files matching these (comma-separated or multiple flags)
  -h, --help              Show this help message

Examples:
  git-de HEAD~5 HEAD -o ./export
  git-de --from v1.0.0 --to v2.0.0 --output ./export --concurrent
  git-de HEAD~5 -I "*.go" -i "*_test.go" -o ./export
`)
	}

	if err := pflag.CommandLine.Parse(args); err != nil {
		return nil, err
	}

	positional := pflag.Args()

	if config.FromCommit == "" && len(positional) > 0 {
		config.FromCommit = positional[0]
	}
	if config.ToCommit == "" && len(positional) > 1 {
		config.ToCommit = positional[1]
	}

	if config.FromCommit == "" {
		pflag.Usage()
		return nil, fmt.Errorf("from-commit is required")
	}

	if config.ToCommit == "" {
		config.ToCommit = "HEAD"
	}

	if config.OutputDir != "" {
		absPath, err := filepath.Abs(config.OutputDir)
		if err != nil {
			return nil, fmt.Errorf("invalid output directory: %w", err)
		}
		config.OutputDir = absPath
		config.Preview = false
	} else {
		config.Preview = true
	}

	// Split comma-separated patterns for both ignore and include
	var expandedIgnores []string
	for _, p := range config.IgnorePatterns {
		parts := strings.Split(p, ",")
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				expandedIgnores = append(expandedIgnores, trimmed)
			}
		}
	}
	config.IgnorePatterns = expandedIgnores

	var expandedIncludes []string
	for _, p := range config.IncludePatterns {
		parts := strings.Split(p, ",")
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				expandedIncludes = append(expandedIncludes, trimmed)
			}
		}
	}
	config.IncludePatterns = expandedIncludes

	return &config, nil
}
