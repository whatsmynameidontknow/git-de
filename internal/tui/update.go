package tui

import (
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/whatsmynameidontknow/git-de/internal/git"
	"github.com/whatsmynameidontknow/git-de/internal/validation"
)

// Update handles all Bubble Tea messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case git.CommitRangeStats:
		m.rangeStats = msg
		m.state = stateCommitRangeSummary
		return m, nil

	case []list.Item:
		return m.handleListItems(msg)

	case []fileItem:
		return m.handleFileItems(msg)

	case exportStartedMsg:
		m.progressCh = msg.ch
		m.totalFiles = msg.fileCount
		return m, waitForProgress(msg.ch)

	case progressMsg:
		return m.handleProgress(msg)

	case exportDoneMsg:
		m.state = stateDone
		return m, nil

	case progress.FrameMsg:
		var progressCmd tea.Cmd
		updatedProgress, progressCmd := m.progress.Update(msg)
		m.progress = updatedProgress.(progress.Model)
		return m, progressCmd

	case error:
		m.err = msg
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if len(m.list.Items()) > 0 {
			m.list.SetSize(msg.Width, msg.Height-5)
		}
		m.progress.Width = msg.Width - 10
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	default:
		// Forward non-key messages (e.g. FilterMatchesMsg, spinner ticks)
		// to the list so filtering actually works.
		if m.state == stateBranchSelection || m.state == stateCommitLimitSelection || m.state == stateFromCommit || m.state == stateToCommit {
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) handleListItems(items []list.Item) (tea.Model, tea.Cmd) {
	w, h := 60, 20
	if m.width > 0 {
		w = m.width
	}
	if m.height > 5 {
		h = m.height - 5
	}
	m.list = list.New(items, list.NewDefaultDelegate(), w, h)
	m.list.KeyMap.Quit = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "quit"))

	switch m.state {
	case stateFromCommit, stateToCommit:
		iBinding := key.NewBinding(key.WithKeys("i", "I"), key.WithHelp("i/I", "toggle inclusive mode"))
		backspaceBinding := key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "back"))
		m.list.AdditionalShortHelpKeys = func() []key.Binding {
			return []key.Binding{backspaceBinding, iBinding}
		}
		m.list.AdditionalFullHelpKeys = func() []key.Binding {
			return []key.Binding{backspaceBinding, iBinding}
		}
	}

	switch m.state {
	case stateBranchSelection:
		m.list.Title = "Select Branch"
		refreshBinding := key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))
		checkoutBinding := key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "checkout"))
		backspaceBinding := key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "back"))
		m.list.AdditionalShortHelpKeys = func() []key.Binding {
			return []key.Binding{refreshBinding, checkoutBinding, backspaceBinding}
		}
		m.list.AdditionalFullHelpKeys = func() []key.Binding {
			return []key.Binding{refreshBinding, checkoutBinding, backspaceBinding}
		}
	case stateCommitLimitSelection:
		if m.selectedBranch != "" {
			m.list.Title = "Select Commit History Depth (on " + m.selectedBranch + ")"
		} else {
			m.list.Title = "Select Commit History Depth"
		}
		// Disable filtering for commit limit selection (only 6 options)
		m.list.SetFilteringEnabled(false)
	case stateFromCommit:
		if m.selectedBranch != "" {
			m.list.Title = "Select From Commit (on " + m.selectedBranch + ")"
		} else {
			m.list.Title = "Select From Commit"
		}
	default:
		if m.selectedBranch != "" {
			m.list.Title = "Select To Commit (on " + m.selectedBranch + ", after " + m.shortHash(m.fromCommit) + ")"
		} else {
			m.list.Title = "Select To Commit (after " + m.shortHash(m.fromCommit) + ")"
		}
	}
	return m, nil
}

func (m Model) handleFileItems(files []fileItem) (tea.Model, tea.Cmd) {
	m.files = files
	m.state = stateFileSelection
	m.filteredIdx = make([]int, len(files))
	for i := range files {
		m.filteredIdx[i] = i
	}
	m.cursor = 0
	return m, nil
}

func (m Model) handleProgress(msg progressMsg) (tea.Model, tea.Cmd) {
	currentProcessed := msg.failedCount + msg.successCount
	m.successCount = msg.successCount
	m.failedCount = msg.failedCount
	m.currentFile = msg.file

	percent := 1.0
	if m.totalFiles > 0 {
		percent = float64(currentProcessed) / float64(m.totalFiles)
	}

	if m.totalFiles > 0 && currentProcessed >= m.totalFiles {
		m.state = stateDone
		return m, m.progress.SetPercent(1)
	}

	return m, tea.Batch(
		m.progress.SetPercent(percent),
		waitForProgress(m.progressCh),
	)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.state {
	case stateBranchSelection:
		return m.handleKeyBranchSelection(msg)
	case stateCommitLimitSelection:
		return m.handleKeyLimitSelection(msg)
	case stateCommitLimitCustom:
		return m.handleKeyLimitCustom(msg)
	case stateFromCommit:
		return m.handleKeyFromCommit(msg)
	case stateToCommit:
		return m.handleKeyToCommit(msg)
	case stateCommitRangeSummary:
		return m.handleKeyCommitRangeSummary(msg)
	case stateFileSelection:
		return m.handleKeyFileSelection(msg)
	case stateOutputPath:
		return m.handleKeyOutputPath(msg)
	case stateConfirm:
		return m.handleKeyConfirm(msg)
	case stateDone:
		return m.handleKeyDone(msg)
	}

	return m, nil
}

func (m Model) handleKeyDone(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if runtime.GOOS == "windows" && msg.String() == "e" || msg.String() == "E" {
		return m, m.openExportDirectory()
	}
	return m, tea.Quit
}

func (m Model) handleKeyBranchSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" && !m.list.SettingFilter() {
		if item := m.list.SelectedItem(); item != nil {
			bi := item.(branchItem)
			m.selectedBranch = bi.branch.Name
			m.state = stateCommitLimitSelection
			return m, m.loadLimitOptionsCmd
		}
	}
	if msg.String() == "r" && !m.list.SettingFilter() {
		return m, m.loadBranchesCmd
	}
	if msg.String() == "c" && !m.list.SettingFilter() {
		if item := m.list.SelectedItem(); item != nil {
			bi := item.(branchItem)
			if err := m.gitClient.CheckoutBranch(bi.branch.Name); err != nil {
				m.err = err
				return m, nil
			}
			// Refresh branches after checkout
			return m, m.loadBranchesCmd
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleKeyLimitSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" && !m.list.SettingFilter() {
		if item := m.list.SelectedItem(); item != nil {
			opt := item.(limitOption)
			if opt.value == -1 {
				m.state = stateCommitLimitCustom
				m.limitInput.Focus()
				m.limitInput.SetValue("")
				return m, nil
			}
			m.commitLimit = opt.value
			m.state = stateFromCommit
			if m.selectedBranch != "" {
				return m, m.loadCommitsOnBranchCmd
			}
			return m, m.loadCommitsCmd
		}
	}
	// if msg.String() == "backspace" && !m.list.SettingFilter() {
	// 	if m.selectedBranch != "" {
	// 		m.state = stateBranchSelection
	// 		return m, m.loadBranchesCmd
	// 	}
	// }
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleKeyLimitCustom(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.limitInput.Value() != "" {
			limit, err := validateCommitLimit(m.limitInput.Value())
			if err != nil {
				m.err = err
				return m, nil
			}
			m.commitLimit = limit
			m.err = nil
			m.state = stateFromCommit
			if m.selectedBranch != "" {
				return m, m.loadCommitsOnBranchCmd
			}
			return m, m.loadCommitsCmd
		}
	case "esc":
		m.state = stateCommitLimitSelection
		m.err = nil
		return m, nil
	default:
		var cmd tea.Cmd
		m.limitInput, cmd = m.limitInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKeyFromCommit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key := msg.String(); !m.list.SettingFilter() && (key == "i" || key == "I") {
		m.inclusiveMode = !m.inclusiveMode
		return m, nil
	}
	if msg.String() == "enter" && !m.list.SettingFilter() {
		if item := m.list.SelectedItem(); item != nil {
			sha := item.(commitItem).sha
			m.fromCommit = m.getFromCommit(sha)
			m.state = stateToCommit
			if m.selectedBranch != "" {
				return m, m.loadToCommitsOnBranchCmd
			}
			return m, m.loadToCommitsCmd
		}
	}
	if msg.String() == "backspace" && !m.list.SettingFilter() {
		m.state = stateCommitLimitSelection
		return m, m.loadLimitOptionsCmd
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleKeyToCommit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key := msg.String(); !m.list.SettingFilter() && (key == "i" || key == "I") {
		m.inclusiveMode = !m.inclusiveMode
		return m, nil
	}
	if msg.String() == "enter" && !m.list.SettingFilter() {
		m.fromCommit = m.getFromCommit(m.fromCommit)
		if item := m.list.SelectedItem(); item != nil {
			m.toCommit = item.(commitItem).sha
			return m, m.loadRangeStatsCmd
		}
	}
	if msg.String() == "backspace" && !m.list.SettingFilter() {
		m.state = stateFromCommit
		if m.selectedBranch != "" {
			return m, m.loadCommitsOnBranchCmd
		}
		return m, m.loadCommitsCmd
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleKeyCommitRangeSummary(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "i", "I":
		m.inclusiveMode = !m.inclusiveMode
		m.fromCommit = m.getFromCommit(m.fromCommit)
		return m.Update(m.loadRangeStatsCmd())
	case "enter", "y", "Y":
		return m, m.loadFilesCmd
	case "backspace", "n", "N":
		m.state = stateToCommit
		if m.selectedBranch != "" {
			return m, m.loadToCommitsOnBranchCmd
		}
		return m, m.loadToCommitsCmd
	case "esc":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleKeyFileSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.ensureFilterIdx()

	if m.inputMode {
		return m.handleKeyFileFilter(msg)
	}

	switch msg.String() {
	case "/":
		m.inputMode = true
		m.filterInput.Focus()
		return m, nil
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case " ":
		if len(m.filteredIdx) > 0 {
			idx := m.filteredIdx[m.cursor]
			if !m.files[idx].disabled {
				m.files[idx].selected = !m.files[idx].selected
			}
		}
	case "a", "A":
		for _, idx := range m.filteredIdx {
			if !m.files[idx].disabled {
				m.files[idx].selected = true
			}
		}
	case "n", "N":
		for _, idx := range m.filteredIdx {
			m.files[idx].selected = false
		}
	case "backspace":
		m.clearFilter()
		m.state = stateCommitRangeSummary
		return m, m.loadToCommitsCmd
	case "c", "C":
		m.clearFilter()
	case "enter":
		m.clearFilter()
		m.outputInputFocused = true
		m.input.Focus()
		m.state = stateOutputPath
		return m, nil
	case "esc":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleKeyFileFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "up", "down":
		if len(m.filteredIdx) == 0 {
			return m, nil
		}
		switch key {
		case "up":
			m.moveCursor(-1)
		case "down":
			m.moveCursor(1)
		}
		m.inputMode = false
		m.filterInput.Blur()
		if len(m.filteredIdx) == 0 {
			m.rebuildFilter()
			m.filterInput.SetValue("")
		}
		return m, nil
	case "enter":
		m.inputMode = false
		m.filterInput.Blur()
		return m, nil
	case "esc":
		m.inputMode = false
		m.filterInput.SetValue("")
		m.filterInput.Blur()
		m.rebuildFilter()
		return m, nil
	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.rebuildFilter()
		return m, cmd
	}
}

func (m Model) handleKeyOutputPath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.outputInputFocused {
		// Input is focused - handle editing
		switch msg.String() {
		case "esc":
			// Blur input
			m.outputInputFocused = false
			m.input.Blur()
			return m, nil
		case "enter":
			// Confirm and proceed
			m.outputPath = m.input.Value()
			if m.outputPath == "" {
				m.outputPath = defaultOutputPath
			}
			// Validate path
			if err := validation.ValidatePath(m.outputPath); err != nil {
				m.err = err
				return m, nil
			}
			m.err = nil
			m.state = stateConfirm
			return m, nil
		default:
			// Pass to input for editing
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}

	// Input is blurred - navigation mode
	switch msg.String() {
	case "esc":
		return m, tea.Quit
	case "backspace":
		// Go back to file selection
		m.state = stateFileSelection
		return m, nil
	case "enter", " ":
		// Focus input
		m.outputInputFocused = true
		m.input.Focus()
		return m, nil
	default:
		// Any printable key - focus input and insert character
		if msg.Type == tea.KeyRunes {
			m.outputInputFocused = true
			m.input.Focus()
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) handleKeyConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if strings.ToLower(msg.String()) == "y" || msg.String() == "enter" {
		m.state = stateProgress
		return m, m.startExport()
	}
	if strings.ToLower(msg.String()) == "n" || msg.String() == "backspace" {
		m.state = stateOutputPath
		m.outputInputFocused = true
		m.input.Focus()
		return m, nil
	}
	if msg.String() == "esc" {
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) moveCursor(delta int) {
	n := len(m.filteredIdx)
	if n == 0 {
		m.cursor = 0
		return
	}
	m.cursor = (m.cursor + delta + n) % n
}

func (m Model) getFromCommit(sha string) string {
	if !m.inclusiveMode {
		return strings.TrimSuffix(sha, "^")
	} else if m.inclusiveMode && !strings.HasSuffix(m.fromCommit, "^") && m.gitClient.IsValid(sha+"^") {
		return sha + "^"
	}

	return sha
}
