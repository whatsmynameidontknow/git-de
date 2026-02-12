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
	FromCommit string
	ToCommit   string
	OutputDir  string
	Overwrite  bool
	Concurrent bool
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

	if err := e.prepareOutputDir(); err != nil {
		return err
	}

	filesToCopy := e.filterFiles(changes)
	
	if e.opts.Concurrent {
		e.copyConcurrent(filesToCopy)
	} else {
		e.copySequential(filesToCopy)
	}

	summary := manifest.Generate(changes)
	summaryPath := filepath.Join(e.opts.OutputDir, "summary.txt")
	if err := manifest.WriteToFile(summaryPath, summary); err != nil {
		return fmt.Errorf("failed to write summary: %w", err)
	}

	fmt.Printf("✓ Exported %d files to %s\n", len(filesToCopy), e.opts.OutputDir)
	return nil
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

func (e *Exporter) filterFiles(changes []git.FileChange) []git.FileChange {
	var result []git.FileChange
	for _, c := range changes {
		if c.ShouldCopy() {
			result = append(result, c)
		}
	}
	return result
}

func (e *Exporter) copySequential(files []git.FileChange) {
	for _, f := range files {
		e.copyFile(f)
	}
}

func (e *Exporter) copyConcurrent(files []git.FileChange) {
	var wg sync.WaitGroup
	for _, f := range files {
		wg.Add(1)
		go func(file git.FileChange) {
			defer wg.Done()
			e.copyFile(file)
		}(f)
	}
	wg.Wait()
}

func (e *Exporter) copyFile(change git.FileChange) error {
	if e.client.IsFileOutsideRepo(change.Path) {
		fmt.Printf("⚠ Skipping file outside repo: %s\n", change.Path)
		return nil
	}

	content, err := e.client.GetFileContent(e.opts.ToCommit, change.Path)
	if err != nil {
		fmt.Printf("⚠ Failed to get content for %s: %v\n", change.Path, err)
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

	switch change.Status {
	case git.StatusRenamed, git.StatusCopied:
		fmt.Printf("→ %s (%s → %s)\n", change.Status, change.OldPath, change.Path)
	default:
		fmt.Printf("→ %s: %s\n", change.Status, change.Path)
	}

	return nil
}
