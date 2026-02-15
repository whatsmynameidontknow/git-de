package exporter

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/whatsmynameidontknow/git-de/internal/git"
)

type mockGitClient struct {
	commits     map[string]bool
	changes     []git.FileChange
	fileContent map[string][]byte
}

func (m *mockGitClient) GetChangedFiles(from, to string) ([]git.FileChange, error) {
	return m.changes, nil
}

func (m *mockGitClient) ValidateCommit(commit string) error {
	if !m.commits[commit] {
		return git.ErrInvalidCommit
	}
	return nil
}

func (m *mockGitClient) GetFileContent(commit, path string) ([]byte, error) {
	content, ok := m.fileContent[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return content, nil
}

func (m *mockGitClient) IsGitRepository() bool              { return true }
func (m *mockGitClient) HasCommits() bool                   { return true }
func (m *mockGitClient) IsFileOutsideRepo(path string) bool { return false }

func TestExporter_Export(t *testing.T) {
	tests := []struct {
		name      string
		opts      Options
		changes   []git.FileChange
		files     map[string][]byte
		wantErr   bool
		wantFiles []string
	}{
		{
			name: "copies added and modified files",
			opts: Options{
				FromCommit: "v1.0.0",
				ToCommit:   "v2.0.0",
				OutputDir:  "",
				Concurrent: false,
			},
			changes: []git.FileChange{
				{Status: "A", Path: "new.go"},
				{Status: "M", Path: "modified.go"},
			},
			files: map[string][]byte{
				"new.go":      []byte("package main"),
				"modified.go": []byte("func main() {}"),
			},
			wantErr:   false,
			wantFiles: []string{"new.go", "modified.go"},
		},
		{
			name: "preserves directory structure",
			opts: Options{
				FromCommit: "v1.0.0",
				ToCommit:   "v2.0.0",
				OutputDir:  "",
				Concurrent: false,
			},
			changes: []git.FileChange{
				{Status: "M", Path: "src/utils/helper.go"},
				{Status: "A", Path: "cmd/app/main.go"},
			},
			files: map[string][]byte{
				"src/utils/helper.go": []byte("package utils"),
				"cmd/app/main.go":     []byte("package main"),
			},
			wantErr:   false,
			wantFiles: []string{"src/utils/helper.go", "cmd/app/main.go"},
		},
		{
			name: "does not copy deleted files",
			opts: Options{
				FromCommit: "v1.0.0",
				ToCommit:   "v2.0.0",
				OutputDir:  "",
				Concurrent: false,
			},
			changes: []git.FileChange{
				{Status: "M", Path: "kept.go"},
				{Status: "D", Path: "deleted.go"},
			},
			files: map[string][]byte{
				"kept.go": []byte("package main"),
			},
			wantErr:   false,
			wantFiles: []string{"kept.go"},
		},
		{
			name: "include patterns filter files",
			opts: Options{
				FromCommit:      "v1.0.0",
				ToCommit:        "v2.0.0",
				OutputDir:       "",
				IncludePatterns: []string{"*.go"},
			},
			changes: []git.FileChange{
				{Status: "A", Path: "main.go"},
				{Status: "M", Path: "README.md"},
				{Status: "A", Path: "utils.go"},
			},
			files: map[string][]byte{
				"main.go":   []byte("package main"),
				"README.md": []byte("# README"),
				"utils.go":  []byte("package utils"),
			},
			wantErr:   false,
			wantFiles: []string{"main.go", "utils.go"},
		},
		{
			name: "include and ignore combined - ignore wins",
			opts: Options{
				FromCommit:      "v1.0.0",
				ToCommit:        "v2.0.0",
				OutputDir:       "",
				IncludePatterns: []string{"*.go"},
				IgnorePatterns:  []string{"*_test.go"},
			},
			changes: []git.FileChange{
				{Status: "A", Path: "main.go"},
				{Status: "M", Path: "main_test.go"},
				{Status: "A", Path: "utils.go"},
			},
			files: map[string][]byte{
				"main.go":      []byte("package main"),
				"main_test.go": []byte("package main"),
				"utils.go":     []byte("package utils"),
			},
			wantErr:   false,
			wantFiles: []string{"main.go", "utils.go"},
		},
		{
			name: "include with path patterns",
			opts: Options{
				FromCommit:      "v1.0.0",
				ToCommit:        "v2.0.0",
				OutputDir:       "",
				IncludePatterns: []string{"cmd/*"},
			},
			changes: []git.FileChange{
				{Status: "A", Path: "cmd/main.go"},
				{Status: "M", Path: "pkg/utils.go"},
				{Status: "A", Path: "cmd/app.go"},
			},
			files: map[string][]byte{
				"cmd/main.go":  []byte("package main"),
				"pkg/utils.go": []byte("package pkg"),
				"cmd/app.go":   []byte("package main"),
			},
			wantErr:   false,
			wantFiles: []string{"cmd/main.go", "cmd/app.go"},
		},
		{
			name: "max-size skips large files",
			opts: Options{
				FromCommit: "v1.0.0",
				ToCommit:   "v2.0.0",
				OutputDir:  "",
				MaxSize:    20,
			},
			changes: []git.FileChange{
				{Status: "A", Path: "small.go"},
				{Status: "A", Path: "large.bin"},
			},
			files: map[string][]byte{
				"small.go":  []byte("package main"),
				"large.bin": make([]byte, 100),
			},
			wantErr:   false,
			wantFiles: []string{"small.go"},
		},
		{
			name: "max-size at exact limit exports file",
			opts: Options{
				FromCommit: "v1.0.0",
				ToCommit:   "v2.0.0",
				OutputDir:  "",
				MaxSize:    12,
			},
			changes: []git.FileChange{
				{Status: "A", Path: "exact.go"},
			},
			files: map[string][]byte{
				"exact.go": []byte("package main"),
			},
			wantErr:   false,
			wantFiles: []string{"exact.go"},
		},
		{
			name: "max-size zero means no limit",
			opts: Options{
				FromCommit: "v1.0.0",
				ToCommit:   "v2.0.0",
				OutputDir:  "",
				MaxSize:    0,
			},
			changes: []git.FileChange{
				{Status: "A", Path: "big.bin"},
			},
			files: map[string][]byte{
				"big.bin": make([]byte, 1000),
			},
			wantErr:   false,
			wantFiles: []string{"big.bin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputDir := t.TempDir() + "/output"
			tt.opts.OutputDir = outputDir

			mock := &mockGitClient{
				commits: map[string]bool{
					tt.opts.FromCommit: true,
					tt.opts.ToCommit:   true,
				},
				changes:     tt.changes,
				fileContent: tt.files,
			}

			exp := New(mock, tt.opts)
			err := exp.Export()

			if (err != nil) != tt.wantErr {
				t.Errorf("Export() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			for _, wantFile := range tt.wantFiles {
				fullPath := filepath.Join(outputDir, wantFile)
				if _, err := os.Stat(fullPath); os.IsNotExist(err) {
					t.Errorf("Expected file %s to exist", wantFile)
				}
			}

			summaryPath := filepath.Join(outputDir, "summary.txt")
			if _, err := os.Stat(summaryPath); os.IsNotExist(err) {
				t.Error("Expected summary.txt to exist")
			}
		})
	}
}

func TestExporter_OutputDirExists(t *testing.T) {
	outputDir := t.TempDir() + "/existing"
	os.MkdirAll(outputDir, 0o755)
	os.WriteFile(filepath.Join(outputDir, "old.txt"), []byte("old"), 0o644)

	mock := &mockGitClient{
		commits:     map[string]bool{"v1.0.0": true, "v2.0.0": true},
		changes:     []git.FileChange{{Status: "A", Path: "new.go"}},
		fileContent: map[string][]byte{"new.go": []byte("new")},
	}

	opts := Options{
		FromCommit: "v1.0.0",
		ToCommit:   "v2.0.0",
		OutputDir:  outputDir,
		Overwrite:  false,
	}

	exp := New(mock, opts)
	err := exp.Export()

	if err == nil {
		t.Error("Expected error when output dir exists without overwrite flag")
	}
}

func TestExporter_Overwrite(t *testing.T) {
	outputDir := t.TempDir() + "/existing"
	os.MkdirAll(outputDir, 0o755)
	os.WriteFile(filepath.Join(outputDir, "old.txt"), []byte("old"), 0o644)

	mock := &mockGitClient{
		commits:     map[string]bool{"v1.0.0": true, "v2.0.0": true},
		changes:     []git.FileChange{{Status: "A", Path: "new.go"}},
		fileContent: map[string][]byte{"new.go": []byte("new")},
	}

	opts := Options{
		FromCommit: "v1.0.0",
		ToCommit:   "v2.0.0",
		OutputDir:  outputDir,
		Overwrite:  true,
	}

	exp := New(mock, opts)
	err := exp.Export()
	if err != nil {
		t.Fatalf("Export() failed: %v", err)
	}

	_, err = os.Stat(filepath.Join(outputDir, "old.txt"))
	if !os.IsNotExist(err) {
		t.Error("Expected old file to be deleted with overwrite flag")
	}

	_, err = os.Stat(filepath.Join(outputDir, "new.go"))
	if os.IsNotExist(err) {
		t.Error("Expected new file to exist")
	}
}

func TestExporter_ArchiveZip(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "export.zip")

	mock := &mockGitClient{
		commits: map[string]bool{"v1.0.0": true, "v2.0.0": true},
		changes: []git.FileChange{
			{Status: "A", Path: "main.go"},
			{Status: "M", Path: "cmd/app.go"},
		},
		fileContent: map[string][]byte{
			"main.go":    []byte("package main"),
			"cmd/app.go": []byte("package cmd"),
		},
	}

	opts := Options{
		FromCommit:  "v1.0.0",
		ToCommit:    "v2.0.0",
		ArchivePath: archivePath,
	}

	exp := New(mock, opts)
	err := exp.Export()
	if err != nil {
		t.Fatalf("Export() failed: %v", err)
	}

	// Verify archive exists
	info, err := os.Stat(archivePath)
	if os.IsNotExist(err) {
		t.Fatal("Expected archive file to exist")
	}
	if info.Size() == 0 {
		t.Error("Expected archive file to not be empty")
	}

	// Verify it's a valid zip by reading it
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("Failed to open zip: %v", err)
	}
	defer r.Close()

	fileNames := make(map[string]bool)
	for _, f := range r.File {
		fileNames[f.Name] = true
	}

	if !fileNames["main.go"] {
		t.Error("Expected main.go in zip")
	}
	if !fileNames["cmd/app.go"] {
		t.Error("Expected cmd/app.go in zip")
	}
	if !fileNames["summary.txt"] {
		t.Error("Expected summary.txt in zip")
	}
}

func TestExporter_Error(t *testing.T) {
	mock := mockGitClient{
		commits: map[string]bool{"v1.0.0": true, "v2.0.0": true},
		changes: []git.FileChange{
			{Status: "A", Path: "main.go"},
		},
		fileContent: map[string][]byte{
			"main.go": []byte("package main"),
		},
	}
	opts := Options{
		FromCommit: "v1.0.0",
		ToCommit:   "v2.0.0",
	}

	exp := New(&mock, opts)
	errors := []error{
		errors.New("main.og: exit status 128"),
		errors.New("main.gg: exit status 128"),
		errors.New("main.oo: exit status 128"),
	}
	hasErrors := exp.HasErrors()
	if hasErrors {
		t.Error("expected hasErrors to be false")
	}
	errCount := exp.ErrorCount()
	if errCount != 0 {
		t.Errorf("expected error count to be 0, got %d", errCount)
	}
	for _, err := range errors {
		exp.AddError(err)
	}
	hasErrors = exp.HasErrors()
	if !hasErrors {
		t.Errorf("expected hasErrors to be true")
	}
	errCount = exp.ErrorCount()
	if errCount != 3 {
		t.Errorf("expected error count to be 3, got %d", errCount)
	}

	var sb strings.Builder
	exp.WriteError(&sb)
	lines := strings.Split(sb.String(), "\n")
	for i, line := range lines {
		if line != errors[i].Error() {
			t.Errorf("expected line[%d] to be %s, got %s", i+1, errors[i].Error(), line)
		}
	}
}

func TestExporter_ArchiveTarGz(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "export.tar.gz")

	mock := &mockGitClient{
		commits: map[string]bool{"v1.0.0": true, "v2.0.0": true},
		changes: []git.FileChange{
			{Status: "A", Path: "main.go"},
		},
		fileContent: map[string][]byte{
			"main.go": []byte("package main"),
		},
	}

	opts := Options{
		FromCommit:  "v1.0.0",
		ToCommit:    "v2.0.0",
		ArchivePath: archivePath,
	}

	exp := New(mock, opts)
	err := exp.Export()
	if err != nil {
		t.Fatalf("Export() failed: %v", err)
	}

	// Verify archive exists and not empty
	info, err := os.Stat(archivePath)
	if os.IsNotExist(err) {
		t.Fatal("Expected archive file to exist")
	}
	if info.Size() == 0 {
		t.Error("Expected archive file to not be empty")
	}

	// Verify it's a valid tar.gz by reading it
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	fileNames := make(map[string]bool)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		fileNames[hdr.Name] = true
	}

	if !fileNames["main.go"] {
		t.Error("Expected main.go in tar.gz")
	}
	if !fileNames["summary.txt"] {
		t.Error("Expected summary.txt in tar.gz")
	}
}
