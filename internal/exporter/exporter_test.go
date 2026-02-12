package exporter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/whatsmynameidontknow/git-de/internal/git"
)

type mockGitClient struct {
	commits    map[string]bool
	changes    []git.FileChange
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

func (m *mockGitClient) IsGitRepository() bool { return true }
func (m *mockGitClient) HasCommits() bool       { return true }
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
				"new.go":     []byte("package main"),
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
			wantErr: false,
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
	os.MkdirAll(outputDir, 0755)
	os.WriteFile(filepath.Join(outputDir, "old.txt"), []byte("old"), 0644)

	mock := &mockGitClient{
		commits: map[string]bool{"v1.0.0": true, "v2.0.0": true},
		changes: []git.FileChange{{Status: "A", Path: "new.go"}},
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
	os.MkdirAll(outputDir, 0755)
	os.WriteFile(filepath.Join(outputDir, "old.txt"), []byte("old"), 0644)

	mock := &mockGitClient{
		commits: map[string]bool{"v1.0.0": true, "v2.0.0": true},
		changes: []git.FileChange{{Status: "A", Path: "new.go"}},
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
