package tui

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/whatsmynameidontknow/git-de/internal/exporter"
	"github.com/whatsmynameidontknow/git-de/internal/git"
	"github.com/whatsmynameidontknow/git-de/internal/manifest"
)

func (m Model) loadBranchesCmd() tea.Msg {
	branches, err := m.gitClient.GetBranchesWithAheadBehind()
	if err != nil {
		return err
	}
	var items []list.Item
	for _, b := range branches {
		items = append(items, branchItem{branch: b})
	}
	return items
}

func (m Model) loadCommitsOnBranchCmd() tea.Msg {
	commits, err := m.gitClient.GetRecentCommitsOnBranch(m.selectedBranch, m.commitLimit)
	if err != nil {
		return err
	}
	var items []list.Item
	for _, c := range commits {
		items = append(items, newCommitItem(c))
	}
	return items
}

func (m Model) loadToCommitsOnBranchCmd() tea.Msg {
	// Get commits after fromCommit on the selected branch
	commits, err := m.gitClient.GetRecentCommitsOnBranch(m.selectedBranch, m.commitLimit)
	if err != nil {
		return err
	}
	// Filter to only commits after fromCommit
	var items []list.Item
	foundFrom := false
	for _, c := range commits {
		if c.Hash == m.fromCommit {
			foundFrom = true
			continue
		}
		if !foundFrom {
			items = append(items, newCommitItem(c))
		}
	}
	// If fromCommit not found in list (e.g., it's older), show all
	if !foundFrom {
		items = nil
		for _, c := range commits {
			if c.Hash != m.fromCommit {
				items = append(items, newCommitItem(c))
			}
		}
	}
	return items
}

func (m Model) loadRangeStatsCmd() tea.Msg {
	stats, err := m.gitClient.GetCommitRangeStats(m.fromCommit, m.toCommit)
	if err != nil {
		return err
	}
	return stats
}

func (m Model) loadLimitOptionsCmd() tea.Msg {
	var items []list.Item
	for _, opt := range commitLimitOptions {
		items = append(items, opt)
	}
	return items
}

func (m Model) loadCommitsCmd() tea.Msg {
	commits, err := m.gitClient.GetRecentCommits(m.commitLimit)
	if err != nil {
		return err
	}
	var items []list.Item
	for _, c := range commits {
		items = append(items, newCommitItem(c))
	}
	return items
}

func (m Model) loadToCommitsCmd() tea.Msg {
	commits, err := m.gitClient.GetCommitsAfter(m.fromCommit, m.commitLimit)
	if err != nil {
		return err
	}
	var items []list.Item
	for _, c := range commits {
		items = append(items, newCommitItem(c))
	}
	return items
}

func (m Model) loadFilesCmd() tea.Msg {
	changes, err := m.gitClient.GetChangedFiles(m.fromCommit, m.toCommit)
	if err != nil {
		return err
	}
	var items []fileItem
	for _, c := range changes {
		disabled := c.Status == git.StatusDeleted
		items = append(items, fileItem{
			path:     c.Path,
			status:   c.Status,
			selected: !disabled,
			disabled: disabled,
			oldPath:  c.OldPath,
		})
	}
	return items
}

func (m Model) startExport() tea.Cmd {
	return func() tea.Msg {
		var selectedFiles []git.FileChange
		for _, f := range m.files {
			if f.selected && !f.disabled {
				selectedFiles = append(selectedFiles, git.FileChange{
					Status:  f.status,
					Path:    f.path,
					OldPath: f.oldPath,
				})
			}
		}

		opts := exporter.Options{
			FromCommit: m.fromCommit,
			ToCommit:   m.toCommit,
			OutputDir:  m.outputPath,
			Overwrite:  true,
		}

		exp := exporter.New(m.gitClient, opts)
		if err := exp.PrepareOutputDir(); err != nil {
			return err
		}

		progressCh := make(chan progressMsg)
		if len(selectedFiles) > concurrentThreshold {
			m.exportConcurrent(exp, selectedFiles, progressCh)
		} else {
			m.exportSequential(exp, selectedFiles, progressCh)
		}

		summary := manifest.Generate(selectedFiles)
		summaryPath := filepath.Join(m.outputPath, "summary.txt")
		_ = manifest.WriteToFile(summaryPath, summary)

		return exportStartedMsg{ch: progressCh, fileCount: len(selectedFiles)}
	}
}

func (m Model) exportSequential(exp *exporter.Exporter, files []git.FileChange, progressCh chan<- progressMsg) {
	go func() {
		var successCount, failedCount int
		for _, f := range files {
			err := exp.CopyFile(f)
			if err != nil {
				failedCount++
				exp.AddError(copyError{
					path: f.Path,
					msg:  err,
				})
				goto send_progress
			}
			successCount++
		send_progress:
			progressCh <- progressMsg{
				successCount: successCount,
				failedCount:  failedCount,
				file:         f.Path,
			}
		}
		close(progressCh)
		if exp.HasErrors() {
			errorFile, _ := os.Create(filepath.Join(m.outputPath, "errors.txt"))

			defer errorFile.Close()
			exp.WriteError(errorFile)
		}
	}()
}

func (m Model) exportConcurrent(exp *exporter.Exporter, files []git.FileChange, progressCh chan<- progressMsg) {
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
					err := exp.CopyFile(f)
					if err != nil {
						failedCount.Add(1)
						exp.AddError(copyError{
							msg:  err,
							path: f.Path,
						})
						goto send_progress
					}
					successCount.Add(1)
				send_progress:
					progressCh <- progressMsg{
						file:         f.Path,
						successCount: int(successCount.Load()),
						failedCount:  int(failedCount.Load()),
					}
				}
			}
		})
	}

	go func() {
		for _, f := range files {
			fileCh <- f
		}
		close(fileCh)
		wg.Wait()
		close(progressCh)
		if exp.HasErrors() {
			errorFile, _ := os.Create(filepath.Join(m.outputPath, "errors.txt"))

			defer errorFile.Close()
			exp.WriteError(errorFile)
		}
	}()
}

func (m Model) openExportDirectory() tea.Cmd {
	wd, err := os.Getwd()
	if err != nil {
		return tea.Quit
	}
	absolutePath := filepath.Join(wd, m.outputPath)
	cmd := exec.Command("explorer", absolutePath)
	_ = cmd.Run()
	return tea.Quit
}

func waitForProgress(ch <-chan progressMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return exportDoneMsg{}
		}
		return msg
	}
}
