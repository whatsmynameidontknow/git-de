package tui

import (
	"path/filepath"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/whatsmynameidontknow/git-de/internal/exporter"
	"github.com/whatsmynameidontknow/git-de/internal/git"
	"github.com/whatsmynameidontknow/git-de/internal/manifest"
)

func (m Model) loadBranchesCmd() tea.Msg {
	branches, err := m.gitClient.GetBranches()
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
		items = append(items, commitItem{sha: c.Hash, message: c.Message})
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
			items = append(items, commitItem{sha: c.Hash, message: c.Message})
		}
	}
	// If fromCommit not found in list (e.g., it's older), show all
	if !foundFrom {
		items = nil
		for _, c := range commits {
			if c.Hash != m.fromCommit {
				items = append(items, commitItem{sha: c.Hash, message: c.Message})
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
		items = append(items, commitItem{sha: c.Hash, message: c.Message})
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
		items = append(items, commitItem{sha: c.Hash, message: c.Message})
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

		total := len(selectedFiles)
		progressChan := make(chan progressMsg)
		go func() {
			for i, f := range selectedFiles {
				_ = exp.CopyFile(f)
				progressChan <- progressMsg{
					current: i + 1,
					total:   total,
					file:    f.Path,
				}
			}
			close(progressChan)
		}()

		summary := manifest.Generate(selectedFiles)
		summaryPath := filepath.Join(m.outputPath, "summary.txt")
		_ = manifest.WriteToFile(summaryPath, summary)

		return exportStartedMsg{ch: progressChan}
	}
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
