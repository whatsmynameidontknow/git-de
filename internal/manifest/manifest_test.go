package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/whatsmynameidontknow/git-de/internal/git"
)

func TestGenerate(t *testing.T) {
	tests := []struct {
		name         string
		changes      []git.FileChange
		wantContains []string
		wantMissing  []string
	}{
		{
			name: "all change types",
			changes: []git.FileChange{
				{Status: "A", Path: "new.go"},
				{Status: "M", Path: "modified.go"},
				{Status: "R", Path: "renamed.go", OldPath: "oldname.go"},
				{Status: "D", Path: "deleted.go"},
			},
			wantContains: []string{
				"new files:", "- new.go",
				"modified:", "- modified.go",
				"renamed:", "- renamed.go (previously oldname.go)",
				"deleted:", "- deleted.go",
			},
			wantMissing: []string{},
		},
		{
			name: "only new files",
			changes: []git.FileChange{
				{Status: "A", Path: "a.go"},
				{Status: "A", Path: "b.go"},
			},
			wantContains: []string{
				"new files:", "- a.go", "- b.go",
			},
			wantMissing: []string{"modified:", "renamed:", "deleted:"},
		},
		{
			name:         "no changes",
			changes:      []git.FileChange{},
			wantContains: []string{},
			wantMissing:  []string{"new files:", "modified:", "renamed:", "deleted:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Generate(tt.changes)

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("Generate() missing: %q\nin:\n%s", want, result)
				}
			}

			for _, unwanted := range tt.wantMissing {
				if strings.Contains(result, unwanted) {
					t.Errorf("Generate() should not contain: %q\nin:\n%s", unwanted, result)
				}
			}
		})
	}
}

func TestWriteToFile(t *testing.T) {
	content := "test summary content"
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "summary.txt")

	err := WriteToFile(outputPath, content)
	if err != nil {
		t.Fatalf("WriteToFile() failed: %v", err)
	}

	readContent, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(readContent) != content {
		t.Errorf("Content mismatch.\nExpected:\n%s\n\nGot:\n%s", content, string(readContent))
	}
}
