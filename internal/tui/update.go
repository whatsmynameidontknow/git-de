package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles all Bubble Tea messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case []list.Item:
		return m.handleListItems(msg)

	case []fileItem:
		return m.handleFileItems(msg)

	case exportStartedMsg:
		m.progressCh = msg.ch
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
		if m.state == stateCommitLimitSelection || m.state == stateFromCommit || m.state == stateToCommit {
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
	case stateCommitLimitSelection:
		m.list.Title = "Select Commit History Depth"
	case stateFromCommit:
		m.list.Title = "Select From Commit"
	default:
		m.list.Title = "Select To Commit (after " + m.shortHash(m.fromCommit) + ")"
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
	m.doneFiles = msg.current
	m.totalFiles = msg.total
	m.currentFile = msg.file

	percent := 1.0
	if msg.total > 0 {
		percent = float64(msg.current) / float64(msg.total)
	}

	if msg.total > 0 && msg.current >= msg.total {
		m.state = stateDone
		return m, m.progress.SetPercent(1)
	}

	return m, tea.Batch(
		m.progress.SetPercent(percent),
		waitForProgress(m.progressCh),
	)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.state == stateDone {
		return m, tea.Quit
	}

	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.state {
	case stateCommitLimitSelection:
		return m.handleKeyLimitSelection(msg)
	case stateCommitLimitCustom:
		return m.handleKeyLimitCustom(msg)
	case stateFromCommit:
		return m.handleKeyFromCommit(msg)
	case stateToCommit:
		return m.handleKeyToCommit(msg)
	case stateFileSelection:
		return m.handleKeyFileSelection(msg)
	case stateOutputPath:
		return m.handleKeyOutputPath(msg)
	case stateConfirm:
		return m.handleKeyConfirm(msg)
	}

	return m, nil
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
			return m, m.loadCommitsCmd
		}
	}
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
	if msg.String() == "enter" && !m.list.SettingFilter() {
		if item := m.list.SelectedItem(); item != nil {
			m.fromCommit = item.(commitItem).sha
			m.state = stateToCommit
			return m, m.loadToCommitsCmd
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleKeyToCommit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" && !m.list.SettingFilter() {
		if item := m.list.SelectedItem(); item != nil {
			m.toCommit = item.(commitItem).sha
			return m, m.loadFilesCmd
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
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
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.filteredIdx)-1 {
			m.cursor++
		}
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
		if m.filterInput.Value() != "" {
			m.filterInput.SetValue("")
			m.rebuildFilter()
		}
	case "enter":
		m.state = stateOutputPath
		return m, nil
	case "esc":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleKeyFileFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "down":
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
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	switch msg.String() {
	case "enter":
		m.outputPath = m.input.Value()
		if m.outputPath == "" {
			m.outputPath = defaultOutputPath
		}
		m.state = stateConfirm
		return m, nil
	case "esc":
		return m, tea.Quit
	}
	return m, cmd
}

func (m Model) handleKeyConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if strings.ToLower(msg.String()) == "y" || msg.String() == "enter" {
		m.state = stateProgress
		return m, m.startExport()
	}
	if strings.ToLower(msg.String()) == "n" || msg.String() == "backspace" {
		m.state = stateOutputPath
		return m, nil
	}
	if msg.String() == "esc" {
		return m, tea.Quit
	}
	return m, nil
}
