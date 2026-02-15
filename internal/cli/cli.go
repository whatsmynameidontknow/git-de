package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"github.com/whatsmynameidontknow/git-de/internal/validation"
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
	MaxSize         int64
	ArchivePath     string
	NoTUI           bool
	ShowVersion     bool
}

func Parse(args []string) (*Config, error) {
	var config Config
	var maxSizeStr string

	pflag.StringVarP(&config.FromCommit, "from", "f", "", "Starting commit")
	pflag.StringVarP(&config.ToCommit, "to", "t", "", "Ending commit (defaults to HEAD)")
	pflag.StringVarP(&config.OutputDir, "output", "o", "", "Output directory")
	pflag.BoolVarP(&config.Overwrite, "overwrite", "w", false, "Overwrite existing output directory")
	pflag.BoolVarP(&config.Concurrent, "concurrent", "c", false, "Copy files concurrently")
	pflag.BoolVarP(&config.Verbose, "verbose", "v", false, "Enable verbose output")
	pflag.StringArrayVarP(&config.IgnorePatterns, "ignore", "i", nil, "Ignore patterns (comma-separated or multiple flags)")
	pflag.StringArrayVarP(&config.IncludePatterns, "include", "I", nil, "Include patterns - only export files matching these (comma-separated or multiple flags)")
	pflag.StringVar(&maxSizeStr, "max-size", "", "Maximum file size to export (e.g., 10MB, 500KB, 1GB)")
	pflag.StringVarP(&config.ArchivePath, "archive", "a", "", "Export to archive file (.zip, .tar, .tar.gz, .tgz)")
	pflag.BoolVar(&config.NoTUI, "no-tui", false, "Force CLI mode even in terminal")
	pflag.BoolVar(&config.ShowVersion, "version", false, "Show app version")

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: git-de [options] [<from-commit> [<to-commit>]]

Export files changed between Git commits.

By default, git-de launches an interactive TUI when run in a terminal.
Use --no-tui to force CLI mode, or provide commit arguments with an output destination to skip the TUI.

Arguments:
  from-commit    Starting commit (optional in TUI mode)
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
      --max-size string   Maximum file size to export (e.g., 10MB, 500KB, 1GB)
  -a, --archive string    Export to archive file (.zip, .tar, .tar.gz, .tgz)
      --no-tui            Force CLI mode even in terminal
  -h, --help              Show this help message

Examples:
  git-de                          # Launch TUI (in terminal)
  git-de HEAD~5                   # Interactive preview of changes
  git-de HEAD~5 HEAD -o ./export       # CLI mode (args + output provided)
  git-de --no-tui HEAD~5          # Force CLI mode without output (preview only)
  git-de --from v1.0.0 --to v2.0.0 --output ./export --concurrent
  git-de HEAD~5 -I "*.go" -i "*_test.go" -o ./export
  git-de HEAD~5 -o ./export --max-size 10MB
  git-de HEAD~5 -a export.zip
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
		// No validation here - handled by main.go after TTY/TUI mode selection
	}

	if config.ToCommit == "" {
		// No default here. Defaulting is handled by main.go (for CLI)
		// or TUI (for interactive selection).
	}

	if config.OutputDir != "" {
		// Validate path
		if err := validation.ValidatePath(config.OutputDir); err != nil {
			return nil, fmt.Errorf("invalid output directory: %w", err)
		}
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

	// Parse max-size
	if maxSizeStr != "" {
		size, err := ParseSize(maxSizeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid max-size: %w", err)
		}
		config.MaxSize = size
	}

	// Validate archive path
	if config.ArchivePath != "" {
		if config.OutputDir != "" {
			return nil, fmt.Errorf("cannot use both --output and --archive")
		}
		// Validate path
		if err := validation.ValidatePath(config.ArchivePath); err != nil {
			return nil, fmt.Errorf("invalid archive path: %w", err)
		}
		ext := strings.ToLower(config.ArchivePath)
		validExt := strings.HasSuffix(ext, ".zip") ||
			strings.HasSuffix(ext, ".tar") ||
			strings.HasSuffix(ext, ".tar.gz") ||
			strings.HasSuffix(ext, ".tgz")
		if !validExt {
			return nil, fmt.Errorf("unsupported archive format: must be .zip, .tar, .tar.gz, or .tgz")
		}
		config.Preview = false
	}

	return &config, nil
}

// ParseSize parses a human-readable size string (e.g., "10MB", "500KB", "1GB") into bytes.
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size string")
	}

	s = strings.ToUpper(s)

	// Find where the numeric part ends
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}

	if i == 0 {
		return 0, fmt.Errorf("invalid size: %q", s)
	}

	numStr := s[:i]
	suffix := s[i:]

	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size number: %q", numStr)
	}

	if num < 0 {
		return 0, fmt.Errorf("size cannot be negative")
	}

	var multiplier int64
	switch suffix {
	case "", "B":
		multiplier = 1
	case "K", "KB":
		multiplier = 1024
	case "M", "MB":
		multiplier = 1024 * 1024
	case "G", "GB":
		multiplier = 1024 * 1024 * 1024
	case "T", "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown size suffix: %q", suffix)
	}

	return num * multiplier, nil
}
