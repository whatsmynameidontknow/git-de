package exporter

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/whatsmynameidontknow/git-de/internal/git"
	"github.com/whatsmynameidontknow/git-de/internal/manifest"
)

type JSONReport struct {
	FromCommit string       `json:"from_commit"`
	ToCommit   string       `json:"to_commit"`
	ExportedAt string       `json:"exported_at"`
	Summary    JSONSummary  `json:"summary"`
	Files      []JSONFile   `json:"files"`
}

type JSONSummary struct {
	TotalFiles int         `json:"total_files"`
	Added      int         `json:"added"`
	Modified   int         `json:"modified"`
	Renamed    int         `json:"renamed"`
	Copied     int         `json:"copied"`
	Deleted    int         `json:"deleted"`
	Skipped    JSONSkipped `json:"skipped"`
}

type JSONSkipped struct {
	Ignored    int `json:"ignored"`
	TooLarge   int `json:"too_large"`
	OutsideRepo int `json:"outside_repo"`
}

type JSONFile struct {
	Path     string `json:"path"`
	Status   string `json:"status"`
	Exported bool   `json:"exported"`
	Reason   string `json:"reason,omitempty"`
	OldPath  string `json:"old_path,omitempty"`
}

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
	ArchivePath     string
	JSON            bool
	JSONFile        string
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
		if e.opts.JSON {
			return e.writeJSONReport(changes, nil)
		}
		return nil
	}

	filesToCopy, jsonFiles := e.filterAndProcess(changes)

	if len(filesToCopy) == 0 {
		fmt.Println("No files to export after filtering.")
		if e.opts.JSON {
			return e.writeJSONReport(changes, jsonFiles)
		}
		return nil
	}

	return e.ExportFiles(filesToCopy, changes, jsonFiles)
}

func (e *Exporter) ExportFiles(filesToCopy []git.FileChange, allChanges []git.FileChange, jsonFiles []JSONFile) error {
	var err error
	if e.opts.Preview {
		err = e.runPreview(filesToCopy, allChanges)
	} else if e.opts.ArchivePath != "" {
		err = e.runArchiveExport(filesToCopy, allChanges)
	} else {
		err = e.runExport(filesToCopy, allChanges)
	}

	if err != nil {
		return err
	}

	if e.opts.JSON {
		return e.writeJSONReport(allChanges, jsonFiles)
	}

	return nil
}

func (e *Exporter) filterAndProcess(changes []git.FileChange) ([]git.FileChange, []JSONFile) {
	var result []git.FileChange
	var jsonFiles []JSONFile

	for _, c := range changes {
		jf := JSONFile{
			Path:   c.Path,
			Status: string(c.Status),
		}
		if c.OldPath != "" {
			jf.OldPath = c.OldPath
		}

		// Skip deleted files
		if c.Status == git.StatusDeleted {
			fmt.Printf("⚠ Deleted: %s\n", c.Path)
			jf.Exported = false
			jf.Reason = "deleted"
			jsonFiles = append(jsonFiles, jf)
			continue
		}

		// Check if should copy
		if !c.ShouldCopy() {
			jf.Exported = false
			jf.Reason = "not copyable"
			jsonFiles = append(jsonFiles, jf)
			continue
		}

		// Check include patterns first (if any specified)
		if len(e.opts.IncludePatterns) > 0 {
			if !e.shouldInclude(c.Path) {
				if e.opts.Verbose {
					fmt.Printf("⊘ Not included: %s\n", c.Path)
				}
				jf.Exported = false
				jf.Reason = "not included"
				jsonFiles = append(jsonFiles, jf)
				continue
			}
		}

		// Check ignore patterns (ignore wins over include)
		if e.shouldIgnore(c.Path) {
			if e.opts.Verbose {
				fmt.Printf("⊘ Ignored: %s\n", c.Path)
			}
			jf.Exported = false
			jf.Reason = "ignored"
			jsonFiles = append(jsonFiles, jf)
			continue
		}

		// Check if outside repo
		if e.client.IsFileOutsideRepo(c.Path) {
			fmt.Printf("⚠ Outside repo: %s\n", c.Path)
			jf.Exported = false
			jf.Reason = "outside repo"
			jsonFiles = append(jsonFiles, jf)
			continue
		}

		// Check file size limit
		if e.opts.MaxSize > 0 {
			content, err := e.client.GetFileContent(e.opts.ToCommit, c.Path)
			if err == nil && int64(len(content)) > e.opts.MaxSize {
				fmt.Printf("⚠ Skipped (too large): %s (%s > %s)\n", c.Path, formatSize(int64(len(content))), formatSize(e.opts.MaxSize))
				jf.Exported = false
				jf.Reason = "too large"
				jsonFiles = append(jsonFiles, jf)
				continue
			}
		}

		jf.Exported = true
		jsonFiles = append(jsonFiles, jf)
		result = append(result, c)
	}
	return result, jsonFiles
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
	if err := e.PrepareOutputDir(); err != nil {
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

	total := len(files)
	e.printProgress(0, total)

	for i, file := range files {
		content, err := e.client.GetFileContent(e.opts.ToCommit, file.Path)
		if err != nil {
			fmt.Printf("⚠ Failed to read: %s\n", file.Path)
			continue
		}

		fw, err := w.Create(file.Path)
		if err != nil {
			return fmt.Errorf("failed to add %s to zip: %w", file.Path, err)
		}

		if _, err := fw.Write(content); err != nil {
			return fmt.Errorf("failed to write %s to zip: %w", file.Path, err)
		}

		if e.opts.Verbose {
			e.printFileInfo(file)
		}
		e.printProgress(i+1, total)
	}

	// Add summary.txt
	summary := manifest.Generate(allChanges)
	fw, err := w.Create("summary.txt")
	if err != nil {
		return fmt.Errorf("failed to add summary.txt to zip: %w", err)
	}
	if _, err := fw.Write([]byte(summary)); err != nil {
		return fmt.Errorf("failed to write summary.txt to zip: %w", err)
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

	total := len(files)
	e.printProgress(0, total)

	for i, file := range files {
		content, err := e.client.GetFileContent(e.opts.ToCommit, file.Path)
		if err != nil {
			fmt.Printf("⚠ Failed to read: %s\n", file.Path)
			continue
		}

		hdr := &tar.Header{
			Name: file.Path,
			Mode: 0644,
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
		e.printProgress(i+1, total)
	}

	// Add summary.txt
	summary := manifest.Generate(allChanges)
	hdr := &tar.Header{
		Name: "summary.txt",
		Mode: 0644,
		Size: int64(len(summary)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("failed to write summary header: %w", err)
	}
	if _, err := tw.Write([]byte(summary)); err != nil {
		return fmt.Errorf("failed to write summary to tar: %w", err)
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

	return os.MkdirAll(e.opts.OutputDir, 0755)
}

func (e *Exporter) copySequential(files []git.FileChange, total int) {
	for i, f := range files {
		e.CopyFile(f)
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
			e.CopyFile(file)
			mu.Lock()
			counter++
			e.printProgress(counter, total)
			mu.Unlock()
		}(f)
	}
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

func (e *Exporter) writeJSONReport(allChanges []git.FileChange, jsonFiles []JSONFile) error {
	var summary JSONSummary
	summary.TotalFiles = len(allChanges)

	for _, c := range allChanges {
		switch c.Status {
		case git.StatusAdded:
			summary.Added++
		case git.StatusModified:
			summary.Modified++
		case git.StatusRenamed:
			summary.Renamed++
		case git.StatusCopied:
			summary.Copied++
		case git.StatusDeleted:
			summary.Deleted++
		}
	}

	for _, jf := range jsonFiles {
		if !jf.Exported {
			switch jf.Reason {
			case "ignored", "not included":
				summary.Skipped.Ignored++
			case "too large":
				summary.Skipped.TooLarge++
			case "outside repo":
				summary.Skipped.OutsideRepo++
			}
		}
	}

	var report JSONReport
	report.FromCommit = e.opts.FromCommit
	report.ToCommit = e.opts.ToCommit
	report.ExportedAt = time.Now().UTC().Format(time.RFC3339)
	report.Summary = summary
	report.Files = jsonFiles

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if e.opts.JSONFile != "" {
		if err := os.WriteFile(e.opts.JSONFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write JSON file: %w", err)
		}
		fmt.Printf("📄 JSON report written to %s\n", e.opts.JSONFile)
	} else {
		fmt.Println(string(data))
	}

	return nil
}
