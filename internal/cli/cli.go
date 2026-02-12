package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"
)

type Config struct {
	FromCommit string
	ToCommit   string
	OutputDir  string
	Overwrite  bool
	Concurrent bool
}

func Parse(args []string) (*Config, error) {
	var config Config

	pflag.StringVarP(&config.FromCommit, "from", "f", "", "Starting commit")
	pflag.StringVarP(&config.ToCommit, "to", "t", "", "Ending commit (defaults to HEAD)")
	pflag.StringVarP(&config.OutputDir, "output", "o", "", "Output directory")
	pflag.BoolVarP(&config.Overwrite, "overwrite", "w", false, "Overwrite existing output directory")
	pflag.BoolVarP(&config.Concurrent, "concurrent", "c", false, "Copy files concurrently")

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: git-de [options] <from-commit> [<to-commit>]

Export files changed between Git commits.

Arguments:
  from-commit    Starting commit (required)
  to-commit      Ending commit (defaults to HEAD)

Options:
  -f, --from string       Starting commit (alternative to positional)
  -t, --to string         Ending commit (defaults to HEAD)
  -o, --output string     Output directory (required)
  -w, --overwrite         Overwrite existing output directory
  -c, --concurrent        Copy files concurrently
  -h, --help              Show this help message

Examples:
  git-de HEAD~5 HEAD -o ./export
  git-de --from v1.0.0 --to v2.0.0 --output ./export --concurrent
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
	}

	return &config, nil
}
