package cli

import (
	"testing"

	"github.com/spf13/pflag"
)

func resetFlags() {
	pflag.CommandLine = pflag.NewFlagSet("git-de", pflag.ContinueOnError)
}

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErr    bool
		wantConfig Config
	}{
		{
			name:    "no output flag runs in preview mode",
			args:    []string{"HEAD~5", "HEAD"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "HEAD~5",
				ToCommit:   "HEAD",
				OutputDir:  "",
				Overwrite:  false,
				Concurrent: false,
				Preview:    true,
			},
		},
		{
			name:    "with output flag",
			args:    []string{"-o", "./export", "v1.0.0", "v2.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "v1.0.0",
				ToCommit:   "v2.0.0",
				OutputDir:  "",
				Overwrite:  false,
				Concurrent: false,
			},
		},
		{
			name:    "overwrite and concurrent flags",
			args:    []string{"--overwrite", "--concurrent", "main", "develop"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "main",
				ToCommit:   "develop",
				Overwrite:  true,
				Concurrent: true,
			},
		},
		{
			name:    "short flags",
			args:    []string{"-o", "./out", "-c", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "v1.0.0",
				ToCommit:   "",
				Overwrite:  false,
				Concurrent: true,
			},
		},
		{
			name:    "missing from-commit is allowed at parse stage",
			args:    []string{},
			wantErr: false,
			wantConfig: Config{
				ToCommit: "",
				Preview:  true,
			},
		},
		{
			name:    "verbose flag",
			args:    []string{"--verbose", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "v1.0.0",
				ToCommit:   "",
				Verbose:    true,
			},
		},
		{
			name:    "ignore patterns",
			args:    []string{"--ignore", "*.log", "--ignore", "node_modules/", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit:     "v1.0.0",
				ToCommit:       "",
				IgnorePatterns: []string{"*.log", "node_modules/"},
			},
		},
		{
			name:    "comma-separated ignore patterns",
			args:    []string{"--ignore", "*.log, *.tmp, node_modules/", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit:     "v1.0.0",
				ToCommit:       "",
				IgnorePatterns: []string{"*.log", "*.tmp", "node_modules/"},
			},
		},
		{
			name:    "include patterns",
			args:    []string{"--include", "*.go", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit:      "v1.0.0",
				ToCommit:        "",
				IncludePatterns: []string{"*.go"},
			},
		},
		{
			name:    "multiple include patterns",
			args:    []string{"--include", "*.go", "--include", "*.md", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit:      "v1.0.0",
				ToCommit:        "",
				IncludePatterns: []string{"*.go", "*.md"},
			},
		},
		{
			name:    "comma-separated include patterns",
			args:    []string{"--include", "*.go, *.md, Makefile", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit:      "v1.0.0",
				ToCommit:        "",
				IncludePatterns: []string{"*.go", "*.md", "Makefile"},
			},
		},
		{
			name:    "include and ignore combined",
			args:    []string{"--include", "*.go", "--ignore", "*_test.go", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit:      "v1.0.0",
				ToCommit:        "",
				IncludePatterns: []string{"*.go"},
				IgnorePatterns:  []string{"*_test.go"},
			},
		},
		{
			name:    "max-size flag",
			args:    []string{"--max-size", "10MB", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "v1.0.0",
				ToCommit:   "",
				MaxSize:    10 * 1024 * 1024,
			},
		},
		{
			name:    "max-size with KB",
			args:    []string{"--max-size", "500KB", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "v1.0.0",
				ToCommit:   "",
				MaxSize:    500 * 1024,
			},
		},
		{
			name:    "max-size with GB",
			args:    []string{"--max-size", "1GB", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "v1.0.0",
				ToCommit:   "",
				MaxSize:    1024 * 1024 * 1024,
			},
		},
		{
			name:    "max-size plain bytes",
			args:    []string{"--max-size", "1024", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "v1.0.0",
				ToCommit:   "",
				MaxSize:    1024,
			},
		},
		{
			name:    "max-size short suffix M",
			args:    []string{"--max-size", "10M", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "v1.0.0",
				ToCommit:   "",
				MaxSize:    10 * 1024 * 1024,
			},
		},
		{
			name:    "invalid max-size",
			args:    []string{"--max-size", "abc", "v1.0.0"},
			wantErr: true,
		},
		{
			name:    "archive flag",
			args:    []string{"--archive", "export.zip", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit:  "v1.0.0",
				ToCommit:    "",
				ArchivePath: "export.zip",
			},
		},
		{
			name:    "archive short flag",
			args:    []string{"-a", "export.tar.gz", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit:  "v1.0.0",
				ToCommit:    "",
				ArchivePath: "export.tar.gz",
			},
		},
		{
			name:    "archive and output mutually exclusive",
			args:    []string{"-a", "export.zip", "-o", "./export", "v1.0.0"},
			wantErr: true,
		},
		{
			name:    "archive with invalid extension",
			args:    []string{"-a", "export.rar", "v1.0.0"},
			wantErr: true,
		},
		{
			name:    "tui flag",
			args:    []string{"--tui", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "v1.0.0",
				ToCommit:   "",
				TUI:        true,
			},
		},
		{
			name:    "no-tui flag",
			args:    []string{"--no-tui", "v1.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "v1.0.0",
				ToCommit:   "",
				NoTUI:      true,
			},
		},
		{
			name:    "tui with commits prefilled",
			args:    []string{"--tui", "v1.0.0", "v2.0.0"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "v1.0.0",
				ToCommit:   "v2.0.0",
				TUI:        true,
			},
		},
		{
			name:    "tui without commits",
			args:    []string{"--tui"},
			wantErr: false,
			wantConfig: Config{
				TUI: true,
			},
		},
		{
			name:    "no-tui without commits is allowed at parse stage",
			args:    []string{"--no-tui"},
			wantErr: false,
			wantConfig: Config{
				ToCommit: "",
				NoTUI:    true,
				Preview:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			config, err := Parse(tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if config.FromCommit != tt.wantConfig.FromCommit {
				t.Errorf("FromCommit = %v, want %v", config.FromCommit, tt.wantConfig.FromCommit)
			}
			if config.ToCommit != tt.wantConfig.ToCommit {
				t.Errorf("ToCommit = %v, want %v", config.ToCommit, tt.wantConfig.ToCommit)
			}
			if config.Overwrite != tt.wantConfig.Overwrite {
				t.Errorf("Overwrite = %v, want %v", config.Overwrite, tt.wantConfig.Overwrite)
			}
			if config.Concurrent != tt.wantConfig.Concurrent {
				t.Errorf("Concurrent = %v, want %v", config.Concurrent, tt.wantConfig.Concurrent)
			}
			if config.MaxSize != tt.wantConfig.MaxSize {
				t.Errorf("MaxSize = %v, want %v", config.MaxSize, tt.wantConfig.MaxSize)
			}
			if config.ArchivePath != tt.wantConfig.ArchivePath {
				t.Errorf("ArchivePath = %v, want %v", config.ArchivePath, tt.wantConfig.ArchivePath)
			}
			if config.TUI != tt.wantConfig.TUI {
				t.Errorf("TUI = %v, want %v", config.TUI, tt.wantConfig.TUI)
			}
			if config.NoTUI != tt.wantConfig.NoTUI {
				t.Errorf("NoTUI = %v, want %v", config.NoTUI, tt.wantConfig.NoTUI)
			}
		})
	}
}

func TestParse_OutputDirAbsolute(t *testing.T) {
	resetFlags()
	args := []string{"-o", "./test-export", "v1.0.0"}
	config, err := Parse(args)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	if config.OutputDir == "" {
		t.Error("OutputDir should not be empty")
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"0", 0, false},
		{"1024", 1024, false},
		{"10B", 10, false},
		{"10K", 10 * 1024, false},
		{"10KB", 10 * 1024, false},
		{"10M", 10 * 1024 * 1024, false},
		{"10MB", 10 * 1024 * 1024, false},
		{"1G", 1024 * 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"1T", 1024 * 1024 * 1024 * 1024, false},
		{"1TB", 1024 * 1024 * 1024 * 1024, false},
		{"500kb", 500 * 1024, false},
		{"2mb", 2 * 1024 * 1024, false},
		{"abc", 0, true},
		{"", 0, true},
		{"MB", 0, true},
		{"-1MB", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseSize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSize(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseSize(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
