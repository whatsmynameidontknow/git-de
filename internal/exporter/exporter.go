package exporter

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/whatsmynameidontknow/git-de/internal/git"
	"github.com/whatsmynameidontknow/git-de/internal/manifest"
)

type GitClient interface {
	GetChangedFiles(from, to string) ([]git.FileChange, error)
	ValidateCommit(commit string) error
	GetFileContent(commit, path string) ([]byte, error)
	IsGitRepository() bool
	HasCommits() bool
	IsFileOutsideRepo(path string) bool
}

type Options struct {
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
}

type Exporter struct {
	client GitClient
	opts   Options
}

func New(client GitClient, opts Options) *Exporter {
	var e Exporter
	e.client = client
	e.opts = opts
	return &e
}

func (e *Exporter) Export() error {
	if err := e.validate(); err != nil {
		return err
	}

	changes, err := e.client.GetChangedFiles(e.opts.FromCommit, e.opts.ToCommit)
	if err != nil {
		return err
	}

	if len(changes) == 0 {
		fmt.Println("No changes found.")
		return nil
	}

	filesToCopy := e.filterAndProcess(changes)

	if len(filesToCopy) == 0 {
		fmt.Println("No files to export after filtering.")
		return nil
	}

	if e.opts.Preview {
		return e.runPreview(filesToCopy, changes)
	}

	return e.runExport(filesToCopy, changes)
}

func (e *Exporter) filterAndProcess(changes []git.FileChange) []git.FileChange {
	var result []git.FileChange
	for _, c := range changes {
		// Skip deleted files
		if c.Status == git.StatusDeleted {
			fmt.Printf("⚠ Deleted: %s\n", c.Path)
			continue
		}

		// Check if should copy
		if !c.ShouldCopy() {
			continue
		}

		// Check include patterns first (if any specified)
		if len(e.opts.IncludePatterns) > 0 {
			if !e.shouldInclude(c.Path) {
				if e.opts.Verbose {
					fmt.Printf("⊘ Not included: %s\n", c.Path)
				}
				continue
			}
		}

		// Check ignore patterns (ignore wins over include)
		if e.shouldIgnore(c.Path) {
			if e.opts.Verbose {
				fmt.Printf("⊘ Ignored: %s\n", c.Path)
			}
			continue
		}

		// Check if outside repo
		if e.client.IsFileOutsideRepo(c.Path) {
			fmt.Printf("⚠ Outside repo: %s\n", c.Path)
			continue
		}

		// Check file size limit
		if e.opts.MaxSize > 0 {
			content, err := e.client.GetFileContent(e.opts.ToCommit, c.Path)
			if err == nil && int64(len(content)) > e.opts.MaxSize {
				fmt.Printf("⚠ Skipped (too large): %s (%s > %s)\n", c.Path, formatSize(int64(len(content))), formatSize(e.opts.MaxSize))
				continue
			}
		}

		result = append(result, c)
	}
	return result
}

func (e *Exporter) shouldIgnore(path string) bool {
	for _, pattern := range e.opts.IgnorePatterns {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

func (e *Exporter) shouldInclude(path string) bool {
	for _, pattern := range e.opts.IncludePatterns {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

func (e *Exporter) runPreview(files []git.FileChange, allChanges []git.FileChange) error {
	fmt.Println("=== PREVIEW MODE (no files will be copied) ===")
	fmt.Printf("\nFiles that would be exported (%d):\n", len(files))
	for _, f := range files {
		e.printFileInfo(f)
	}
	fmt.Println("\n=== Summary ===")
	fmt.Println(manifest.Generate(allChanges))
	return nil
}

func (e *Exporter) runExport(files []git.FileChange, allChanges []git.FileChange) error {
	if err := e.prepareOutputDir(); err != nil {
		return err
	}

	total := len(files)
	e.printProgress(0, total)

	if e.opts.Concurrent {
		e.copyConcurrent(files, total)
	} else {
		e.copySequential(files, total)
	}

	summary := manifest.Generate(allChanges)
	summaryPath := filepath.Join(e.opts.OutputDir, "summary.txt")
	if err := manifest.WriteToFile(summaryPath, summary); err != nil {
		return fmt.Errorf("failed to write summary: %w", err)
	}

	fmt.Printf("\n✓ Exported %d files to %s\n", total, e.opts.OutputDir)
	return nil
}

func (e *Exporter) printFileInfo(f git.FileChange) {
	switch f.Status {
	case git.StatusRenamed:
		fmt.Printf("  → R: %s (from %s)\n", f.Path, f.OldPath)
	case git.StatusCopied:
		fmt.Printf("  → C: %s (from %s)\n", f.Path, f.OldPath)
	default:
		fmt.Printf("  → %s: %s\n", f.Status, f.Path)
	}
}

func (e *Exporter) printProgress(current, total int) {
	if !e.opts.Verbose {
		percent := float64(current) / float64(total) * 100
		fmt.Printf("\r[%3.0f%%] %d/%d files", percent, current, total)
		if current == total {
			fmt.Println()
		}
	}
}

func (e *Exporter) validate() error {
	if !e.client.IsGitRepository() {
		return fmt.Errorf("not a git repository")
	}
	if !e.client.HasCommits() {
		return fmt.Errorf("repository has no commits")
	}
	if err := e.client.ValidateCommit(e.opts.FromCommit); err != nil {
		return fmt.Errorf("invalid from-commit: %w", err)
	}
	if err := e.client.ValidateCommit(e.opts.ToCommit); err != nil {
		return fmt.Errorf("invalid to-commit: %w", err)
	}
	return nil
}

func (e *Exporter) prepareOutputDir() error {
	info, err := os.Stat(e.opts.OutputDir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("output path exists and is not a directory")
		}
		if !e.opts.Overwrite {
			return fmt.Errorf("output directory already exists (use --overwrite to replace)")
		}
		if err := os.RemoveAll(e.opts.OutputDir); err != nil {
			return fmt.Errorf("failed to clear output directory: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check output directory: %w", err)
	}

	return os.MkdirAll(e.opts.OutputDir, 0755)
}

func (e *Exporter) copySequential(files []git.FileChange, total int) {
	for i, f := range files {
		e.copyFile(f)
		e.printProgress(i+1, total)
	}
}

func (e *Exporter) copyConcurrent(files []git.FileChange, total int) {
	var wg sync.WaitGroup
	var counter int
	var mu sync.Mutex

	for _, f := range files {
		wg.Add(1)
		go func(file git.FileChange) {
			defer wg.Done()
			e.copyFile(file)
			mu.Lock()
			counter++
			e.printProgress(counter, total)
			mu.Unlock()
		}(f)
	}
	wg.Wait()
}

func (e *Exporter) copyFile(change git.FileChange) error {
	if e.client.IsFileOutsideRepo(change.Path) {
		return nil
	}

	content, err := e.client.GetFileContent(e.opts.ToCommit, change.Path)
	if err != nil {
		return err
	}

	targetPath := filepath.Join(e.opts.OutputDir, change.Path)
	targetDir := filepath.Dir(targetPath)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(targetPath, content, 0644); err != nil {
		return err
	}

	if e.opts.Verbose {
		switch change.Status {
		case git.StatusRenamed, git.StatusCopied:
			fmt.Printf("→ %s: %s (from %s)\n", change.Status, change.Path, change.OldPath)
		default:
			fmt.Printf("→ %s: %s\n", change.Status, change.Path)
		}
	}

	return nil
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1fGB", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
