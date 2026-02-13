package main

import (
	"testing"

	"github.com/whatsmynameidontknow/git-de/internal/cli"
)

func TestShouldUseTUI(t *testing.T) {
	tests := []struct {
		name     string
		config   *cli.Config
		isTTY    bool
		expected bool
	}{
		{
			name:     "explicit --no-tui forces CLI mode",
			config:   &cli.Config{NoTUI: true, FromCommit: ""},
			isTTY:    true,
			expected: false,
		},
		{
			name:     "positional args bypass TUI only if output is set",
			config:   &cli.Config{NoTUI: false, FromCommit: "HEAD~5", OutputDir: "./export"},
			isTTY:    true,
			expected: false,
		},
		{
			name:     "positional args bypass TUI only if archive is set",
			config:   &cli.Config{NoTUI: false, FromCommit: "HEAD~5", ArchivePath: "export.zip"},
			isTTY:    true,
			expected: false,
		},
		{
			name:     "positional args launch TUI if no output/archive set",
			config:   &cli.Config{NoTUI: false, FromCommit: "HEAD~5"},
			isTTY:    true,
			expected: true,
		},
		{
			name:     "TTY auto-detects TUI mode",
			config:   &cli.Config{NoTUI: false, FromCommit: ""},
			isTTY:    true,
			expected: true,
		},
		{
			name:     "non-TTY defaults to CLI",
			config:   &cli.Config{NoTUI: false, FromCommit: ""},
			isTTY:    false,
			expected: false,
		},
		{
			name:     "args override TTY detection",
			config:   &cli.Config{NoTUI: false, FromCommit: "v1.0.0", OutputDir: "./out"},
			isTTY:    true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldUseTUIWithOverride(tt.config, tt.isTTY)
			if result != tt.expected {
				t.Errorf("shouldUseTUIWithOverride() = %v, want %v", result, tt.expected)
			}
		})
	}
}
