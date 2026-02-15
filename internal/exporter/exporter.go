package exporter

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/whatsmynameidontknow/git-de/internal/git"
	"github.com/whatsmynameidontknow/git-de/internal/manifest"
)

type GitExporter interface {
	GetChangedFiles(from, to string) (changedFile []git.FileChange, err error)
	ValidateCommit(commit string) (err error)
	GetFileContent(commit, path string) (content []byte, err error)
	IsGitRepository() (ok bool)
	HasCommits() (ok bool)
	IsFileOutsideRepo(path string) (ok bool)
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
	ArchivePath     string
}

type Exporter struct {
	errors []error
	mu     *sync.RWMutex
	client GitExporter
	opts   Options
}

func New(client GitExporter, opts Options) *Exporter {
	return &Exporter{
		client: client,
		opts:   opts,
		mu:     new(sync.RWMutex),
	}
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

	return e.ExportFiles(filesToCopy, changes)
}

func (e *Exporter) ExportFiles(filesToCopy []git.FileChange, allChanges []git.FileChange) error {
	var err error
	if e.opts.Preview {
		err = e.runPreview(filesToCopy)
	} else if e.opts.ArchivePath != "" {
		err = e.runArchiveExport(filesToCopy, allChanges)
	} else {
		err = e.runExport(filesToCopy, allChanges)
	}

	return err
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

func (e *Exporter) runPreview(files []git.FileChange) error {
	fmt.Println("=== PREVIEW MODE (no files will be copied) ===")
	fmt.Printf("\nFiles that would be exported (%d):\n", len(files))
	for _, f := range files {
		e.printFileInfo(f)
	}
	return nil
}

func (e *Exporter) runExport(files []git.FileChange, allChanges []git.FileChange) error {
	if err := e.PrepareOutputDir(); err != nil {
		return err
	}

	total := len(files)
	e.printProgress(0, 0, total)

	if e.opts.Concurrent {
		e.copyConcurrent(files, total)
	} else {
		e.copySequential(files, total)
	}

	summary := manifest.Generate(allChanges)
	summaryPath := filepath.Join(e.opts.OutputDir, "summary.txt")
	if err := manifest.WriteToFile(summaryPath, summary); err != nil {
		return fmt.Errorf("failed to write summary.txt: %w", err)
	}
	if e.HasErrors() {
		errorFile, err := os.Create(filepath.Join(e.opts.OutputDir, "errors.txt"))
		if err != nil {
			return fmt.Errorf("failed to write errors.txt: %w", err)
		}
		defer errorFile.Close()
		e.WriteError(errorFile)
	}

	fmt.Printf("\n✓ Exported %d files to %s\n", total, e.opts.OutputDir)
	return nil
}

func (e *Exporter) runArchiveExport(files []git.FileChange, allChanges []git.FileChange) error {
	archivePath := e.opts.ArchivePath
	lower := strings.ToLower(archivePath)

	if strings.HasSuffix(lower, ".zip") {
		return e.exportToZip(files, allChanges)
	}
	return e.exportToTarGz(files, allChanges)
}

func (e *Exporter) exportToZip(files []git.FileChange, allChanges []git.FileChange) error {
	f, err := os.Create(e.opts.ArchivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	var (
		successCount, failedCount int
		total                     = len(files)
		fw                        io.Writer
	)
	e.printProgress(successCount, failedCount, total)

	for _, file := range files {
		content, err := e.client.GetFileContent(e.opts.ToCommit, file.Path)
		if err != nil {
			e.AddError(fmt.Errorf("%s: %w", file.Path, err))
			if e.opts.Verbose {
				fmt.Printf("⚠ Failed to read: %s\n", file.Path)
			}
			failedCount++
			goto update_progress
		}

		fw, err = w.Create(file.Path)
		if err != nil {
			return fmt.Errorf("failed to add %s to zip: %w", file.Path, err)
		}

		if _, err := fw.Write(content); err != nil {
			return fmt.Errorf("failed to write %s to zip: %w", file.Path, err)
		}

		if e.opts.Verbose {
			e.printFileInfo(file)
		}
		successCount++
	update_progress:
		e.printProgress(successCount, failedCount, total)
	}

	// Add summary.txt
	summary := manifest.Generate(allChanges)
	fw, err = w.Create("summary.txt")
	if err != nil {
		return fmt.Errorf("failed to add summary.txt to zip: %w", err)
	}
	if _, err := fw.Write([]byte(summary)); err != nil {
		return fmt.Errorf("failed to write summary.txt to zip: %w", err)
	}

	if e.HasErrors() {
		fw, err = w.Create("errors.txt")
		if err != nil {
			return fmt.Errorf("failed to add errors.txt to zip: %w", err)
		}
		e.WriteError(fw)
	}

	fmt.Printf("\n✓ Archived %d files to %s\n", total, e.opts.ArchivePath)
	return nil
}

func (e *Exporter) exportToTarGz(files []git.FileChange, allChanges []git.FileChange) error {
	f, err := os.Create(e.opts.ArchivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	defer f.Close()

	var tw *tar.Writer
	lower := strings.ToLower(e.opts.ArchivePath)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		gw := gzip.NewWriter(f)
		defer gw.Close()
		tw = tar.NewWriter(gw)
	} else {
		tw = tar.NewWriter(f)
	}
	defer tw.Close()

	var (
		successCount, failedCount int
		hdr                       *tar.Header
		total                     = len(files)
	)
	e.printProgress(successCount, failedCount, total)
	for _, file := range files {
		content, err := e.client.GetFileContent(e.opts.ToCommit, file.Path)
		if err != nil {
			e.AddError(fmt.Errorf("%s: %w", file.Path, err))
			if e.opts.Verbose {
				fmt.Printf("⚠ Failed to read: %s\n", file.Path)
			}
			failedCount++
			goto update_progress
		}

		hdr = &tar.Header{
			Name: file.Path,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %w", file.Path, err)
		}
		if _, err := tw.Write(content); err != nil {
			return fmt.Errorf("failed to write %s to tar: %w", file.Path, err)
		}

		if e.opts.Verbose {
			e.printFileInfo(file)
		}
		successCount++
	update_progress:
		e.printProgress(successCount, failedCount, total)
	}

	// Add summary.txt
	summary := manifest.Generate(allChanges)
	hdr = &tar.Header{
		Name: "summary.txt",
		Mode: 0o644,
		Size: int64(len(summary)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("failed to write summary.txt header: %w", err)
	}
	if _, err := tw.Write([]byte(summary)); err != nil {
		return fmt.Errorf("failed to write summary.txt to tar: %w", err)
	}

	errorString := e.errorString()
	if len(errorString) > 0 {
		hdr = &tar.Header{
			Name: "errors.txt",
			Mode: 0o644,
			Size: int64(len(errorString)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("failed to write errors.txt header: %w", err)
		}
		if _, err := io.WriteString(tw, errorString); err != nil {
			return fmt.Errorf("failed to write errors.txt: %w", err)
		}
	}

	fmt.Printf("\n✓ Archived %d files to %s\n", total, e.opts.ArchivePath)
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

func (e *Exporter) printProgress(success, failed, total int) {
	if !e.opts.Verbose {
		current := success + failed
		percent := float64(current) / float64(total) * 100
		fmt.Printf("\r[%3.0f%%] %d/%d files (%d failed)", percent, success, total, failed)
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

func (e *Exporter) PrepareOutputDir() error {
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

	return os.MkdirAll(e.opts.OutputDir, 0o755)
}

func (e *Exporter) copySequential(files []git.FileChange, total int) {
	var successCount, failedCount int
	for _, f := range files {
		err := e.CopyFile(f)
		if err != nil {
			e.AddError(fmt.Errorf("%s: %s", f.Path, err))
			if e.opts.Verbose {
				fmt.Printf("⚠ Failed to copy: %s\n", f.Path)
			}
			failedCount++
			goto update_progress
		}
		successCount++
	update_progress:
		e.printProgress(successCount, failedCount, total)
	}
}

const (
	bufferSize = 20
	numWorkers = 5
)

func (e *Exporter) copyConcurrent(files []git.FileChange, total int) {
	fileCh := make(chan git.FileChange, bufferSize)
	successCount := new(atomic.Int64)
	failedCount := new(atomic.Int64)
	wg := new(sync.WaitGroup)
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	for range numWorkers {
		wg.Go(func() {
			for {
				select {
				case <-signalCh:
					return
				case f, ok := <-fileCh:
					if !ok {
						return
					}
					err := e.CopyFile(f)
					if err != nil {
						e.AddError(fmt.Errorf("%s: %s", f.Path, err))
						if e.opts.Verbose {
							fmt.Printf("⚠ Failed to copy: %s\n", f.Path)
						}
						failedCount.Add(1)
						goto update_progress
					}
					successCount.Add(1)
				update_progress:
					e.printProgress(int(successCount.Load()), int(failedCount.Load()), total)
				}
			}
		})
	}

	go func() {
		for _, f := range files {
			fileCh <- f
		}
		close(fileCh)
	}()
	wg.Wait()
}

func (e *Exporter) CopyFile(change git.FileChange) error {
	if e.client.IsFileOutsideRepo(change.Path) {
		return nil
	}

	content, err := e.client.GetFileContent(e.opts.ToCommit, change.Path)
	if err != nil {
		return err
	}

	targetPath := filepath.Join(e.opts.OutputDir, change.Path)
	targetDir := filepath.Dir(targetPath)

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
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

func (e *Exporter) errorString() string {
	e.mu.RLock()
	errorList := slices.Clone(e.errors)
	e.mu.RUnlock()
	var sb strings.Builder
	for i, err := range errorList {
		sb.WriteString(err.Error())
		if i < len(errorList)-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

func (e *Exporter) WriteError(w io.Writer) {
	errorString := e.errorString()
	_, _ = io.WriteString(w, errorString)
}

func (e *Exporter) AddError(err error) {
	e.mu.Lock()
	e.errors = append(e.errors, err)
	e.mu.Unlock()
}

func (e *Exporter) ErrorCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.errors)
}

func (e *Exporter) HasErrors() bool {
	return e.ErrorCount() > 0
}
