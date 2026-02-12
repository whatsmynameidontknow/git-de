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
			name:    "positional args only",
			args:    []string{"HEAD~5", "HEAD"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "HEAD~5",
				ToCommit:   "HEAD",
				Overwrite:  false,
				Concurrent: false,
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
				ToCommit:   "HEAD",
				Overwrite:  false,
				Concurrent: true,
			},
		},
		{
			name:    "missing from-commit",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "defaults to-commit to HEAD",
			args:    []string{"HEAD~5"},
			wantErr: false,
			wantConfig: Config{
				FromCommit: "HEAD~5",
				ToCommit:   "HEAD",
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
