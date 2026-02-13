package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/whatsmynameidontknow/git-de/internal/exporter"
	"github.com/whatsmynameidontknow/git-de/internal/git"
	"github.com/whatsmynameidontknow/git-de/internal/manifest"
)

type sessionState int

const (
	stateCommitLimitSelection sessionState = iota
	stateCommitLimitCustom
	stateFromCommit
	stateToCommit
	stateFileSelection
	stateOutputPath
	stateConfirm
	stateProgress
	stateDone
)

const (
	defaultOutputPath  = "./export"
	defaultCommitLimit = 50
	commitLimitAll     = 999999
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			MarginBottom(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AD58B4")).
			Bold(true)
)

type commitItem struct {
	sha     string
	message string
}

func (i commitItem) Title() string       { return i.message }
func (i commitItem) Description() string { return i.sha }
func (i commitItem) FilterValue() string { return i.message }

type limitOption struct {
	label string
	value int // -1 for custom
}

func (i limitOption) Title() string       { return i.label }
func (i limitOption) Description() string { return "" }
func (i limitOption) FilterValue() string { return i.label }

var commitLimitOptions = []limitOption{
	{label: "10 commits (quick review)", value: 10},
	{label: "50 commits (standard)", value: 50},
	{label: "100 commits (extended)", value: 100},
	{label: "500 commits (deep history)", value: 500},
	{label: "All commits (may be slow)", value: commitLimitAll},
	{label: "Custom...", value: -1},
}

type fileItem struct {
	path     string
	oldPath  string
	status   git.FileStatus
	selected bool
	disabled bool
}

func (i fileItem) Title() string {
	prefix := "[ ]"
	if i.disabled {
		prefix = "[✗]"
	} else if i.selected {
		prefix = "[✓]"
	}

	statusStr := string(i.status)
	if i.status == git.StatusRenamed || i.status == git.StatusCopied {
		return fmt.Sprintf("%s %s: %s (from %s)", prefix, statusStr, i.path, i.oldPath)
	}
	return fmt.Sprintf("%s %s: %s", prefix, statusStr, i.path)
}

func (i fileItem) Description() string {
	if i.disabled {
		return "(deleted - cannot export)"
	}
	return ""
}
func (i fileItem) FilterValue() string { return i.path }

type progressMsg struct {
	file    string
	current int
	total   int
}

type exportStartedMsg struct {
	ch <-chan progressMsg
}

type (
	exportDoneMsg struct{}
	Model         struct {
		state     sessionState
		gitClient *git.Client
		err       error

		// Inputs
		fromCommit string
		toCommit   string
		outputPath string

		// Commit limit
		commitLimit int
		limitInput  textinput.Model

		// Components
		list     list.Model
		input    textinput.Model
		progress progress.Model

		// Data
		files       []fileItem
		filteredIdx []int // indices into files for current filter
		cursor      int
		inputMode   bool
		filterInput textinput.Model

		// Window size
		width  int
		height int

		// Progress tracking
		totalFiles  int
		doneFiles   int
		currentFile string
		progressCh  <-chan progressMsg
	}
)

func NewModel(client *git.Client, from, to string) Model {
	ti := textinput.New()
	ti.Placeholder = defaultOutputPath
	ti.SetValue(defaultOutputPath)
	ti.Focus()

	commitList := list.New([]list.Item{}, list.NewDefaultDelegate(), 60, 20)
	commitList.KeyMap.Quit = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "quit"))

	fi := textinput.New()
	fi.Placeholder = "type to filter..."
	fi.Prompt = "/ "

	li := textinput.New()
	li.Placeholder = "Enter number of commits (1-999999)"
	li.CharLimit = 6

	prog := progress.New(progress.WithDefaultGradient())

	m := Model{
		gitClient:   client,
		list:        commitList,
		input:       ti,
		filterInput: fi,
		limitInput:  li,
		progress:    prog,
		fromCommit:  from,
		toCommit:    to,
		commitLimit: defaultCommitLimit,
	}

	if from != "" && to != "" {
		m.state = stateFileSelection
	} else if from != "" {
		m.state = stateToCommit
	} else {
		m.state = stateCommitLimitSelection
	}

	return m
}

func Run(client *git.Client, from, to string) error {
	m := NewModel(client, from, to)
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

func validateCommitLimit(input string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return 0, fmt.Errorf("invalid number")
	}
	if n < 1 {
		return 0, fmt.Errorf("must be at least 1")
	}
	if n > commitLimitAll {
		return 0, fmt.Errorf("maximum is %d", commitLimitAll)
	}
	return n, nil
}

func (m Model) Init() tea.Cmd {
	switch m.state {
	case stateCommitLimitSelection:
		return m.loadLimitOptionsCmd
	case stateToCommit:
		return m.loadToCommitsCmd
	case stateFileSelection:
		return m.loadFilesCmd
	default:
		return m.loadCommitsCmd
	}
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

func (m *Model) ensureFilterIdx() {
	if m.filteredIdx == nil && len(m.files) > 0 {
		m.filteredIdx = make([]int, len(m.files))
		for i := range m.files {
			m.filteredIdx[i] = i
		}
	}
}

func (m *Model) rebuildFilter() {
	query := strings.ToLower(m.filterInput.Value())
	m.filteredIdx = m.filteredIdx[:0]
	for i, f := range m.files {
		if query == "" || strings.Contains(strings.ToLower(f.path), query) ||
			strings.Contains(strings.ToLower(string(f.status)), query) {
			m.filteredIdx = append(m.filteredIdx, i)
		}
	}
	if m.cursor >= len(m.filteredIdx) {
		m.cursor = max(0, len(m.filteredIdx)-1)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case []list.Item:
		w, h := 60, 20
		if m.width > 0 {
			w = m.width
		}
		if m.height > 5 {
			h = m.height - 5
		}
		m.list = list.New(msg, list.NewDefaultDelegate(), w, h)
		m.list.KeyMap.Quit = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "quit"))

		if m.state == stateCommitLimitSelection {
			m.list.Title = "Select Commit History Depth"
		} else if m.state == stateFromCommit {
			m.list.Title = "Select From Commit"
		} else {
			m.list.Title = "Select To Commit (after " + m.shortHash(m.fromCommit) + ")"
		}
		return m, nil

	case []fileItem:
		m.files = msg
		m.state = stateFileSelection
		m.filteredIdx = make([]int, len(msg))
		for i := range msg {
			m.filteredIdx[i] = i
		}
		m.cursor = 0
		return m, nil

	case exportStartedMsg:
		m.progressCh = msg.ch
		return m, waitForProgress(msg.ch)

	case progressMsg:
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

	default:
		// Forward non-key messages (e.g. FilterMatchesMsg, spinner ticks) to the list
		// so filtering actually works.
		if m.state == stateCommitLimitSelection || m.state == stateFromCommit || m.state == stateToCommit {
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		if m.state == stateDone {
			return m, tea.Quit
		}

		// ctrl+c always quits
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.state {
		case stateCommitLimitSelection:
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
			m.list, cmd = m.list.Update(msg)
			return m, cmd

		case stateCommitLimitCustom:
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
				m.limitInput, cmd = m.limitInput.Update(msg)
				return m, cmd
			}
			return m, nil

		case stateFromCommit:
			if msg.String() == "enter" && !m.list.SettingFilter() {
				if item := m.list.SelectedItem(); item != nil {
					m.fromCommit = item.(commitItem).sha
					m.state = stateToCommit
					return m, m.loadToCommitsCmd
				}
			}
			m.list, cmd = m.list.Update(msg)
			return m, cmd

		case stateToCommit:
			if msg.String() == "enter" && !m.list.SettingFilter() {
				if item := m.list.SelectedItem(); item != nil {
					m.toCommit = item.(commitItem).sha
					return m, m.loadFilesCmd
				}
			}
			m.list, cmd = m.list.Update(msg)
			return m, cmd

		case stateFileSelection:
			m.ensureFilterIdx()
			if m.inputMode {
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
					m.filterInput, cmd = m.filterInput.Update(msg)
					m.rebuildFilter()
					return m, cmd
				}
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
			case " ": // Toggle
				if len(m.filteredIdx) > 0 {
					idx := m.filteredIdx[m.cursor]
					if !m.files[idx].disabled {
						m.files[idx].selected = !m.files[idx].selected
					}
				}
			case "a", "A": // Select all (visible)
				for _, idx := range m.filteredIdx {
					if !m.files[idx].disabled {
						m.files[idx].selected = true
					}
				}
			case "n", "N": // Select none (visible)
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

		case stateOutputPath:
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

		case stateConfirm:
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
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if len(m.list.Items()) > 0 {
			m.list.SetSize(msg.Width, msg.Height-5)
		}
		m.progress.Width = msg.Width - 10
	}

	return m, nil
}

func (m Model) shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

func (m Model) View() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Git Diff Export") + "\n\n")

	switch m.state {
	case stateCommitLimitSelection:
		sb.WriteString(m.list.View())

	case stateCommitLimitCustom:
		sb.WriteString("Enter Custom Commit Limit:\n\n")
		sb.WriteString(m.limitInput.View())
		sb.WriteString("\n\n")
		sb.WriteString(statusStyle.Render("Enter a number between 1 and 999999"))
		sb.WriteString("\n[Enter:confirm] [esc:back]\n")

	case stateFromCommit, stateToCommit:
		sb.WriteString(m.list.View())

	case stateFileSelection:
		sb.WriteString("Select Files to Export:\n")
		fmt.Fprintf(&sb, "Range: %s...%s\n\n", m.shortHash(m.fromCommit), m.shortHash(m.toCommit))

		if m.inputMode || m.filterInput.Value() != "" {
			sb.WriteString(m.filterInput.View() + "\n\n")
		}

		displayIdx := m.filteredIdx
		if displayIdx == nil {
			displayIdx = make([]int, len(m.files))
			for i := range m.files {
				displayIdx[i] = i
			}
		}

		visibleStart := 0
		visibleEnd := len(displayIdx)
		maxVisible := m.height - 10
		if maxVisible < 5 {
			maxVisible = 20
		}
		if visibleEnd > maxVisible {
			half := maxVisible / 2
			visibleStart = m.cursor - half
			visibleStart = max(0, visibleStart)
			visibleEnd = visibleStart + maxVisible
			if visibleEnd > len(displayIdx) {
				visibleEnd = len(displayIdx)
				visibleStart = visibleEnd - maxVisible
			}
		}
		if len(displayIdx) > 0 {
			for vi := visibleStart; vi < visibleEnd; vi++ {
				f := m.files[displayIdx[vi]]
				cursor := " "
				if m.cursor == vi {
					cursor = ">"
				}

				line := f.Title()
				if m.cursor == vi {
					line = selectedStyle.Render(line)
				}

				fmt.Fprintf(&sb, "%s %s\n", cursor, line)
			}
		}
		// Build status line based on mode and filter state
		filteredFiles := len(displayIdx)
		totalFiles := len(m.files)
		filteredOut := totalFiles - filteredFiles
		filterText := m.filterInput.Value()
		hasFilter := filterText != ""
		
		if m.inputMode {
			// Input mode: show items without selected count
			if filteredFiles == 0 {
				fmt.Fprintf(&sb, "none matched (%d filtered)", filteredOut)
			} else if hasFilter {
				fmt.Fprintf(&sb, "%d items (%d filtered)", filteredFiles, filteredOut)
			} else {
				fmt.Fprintf(&sb, "%d items", filteredFiles)
			}
		} else {
			// Not in input mode: show items with selected count
			selectedCount := 0
			for _, f := range m.files {
				if f.selected && !f.disabled {
					selectedCount++
				}
			}
			
			if filteredFiles == 0 {
				fmt.Fprintf(&sb, "none matched (%d filtered) | %d selected", filteredOut, selectedCount)
			} else if hasFilter {
				fmt.Fprintf(&sb, "%d items (%d filtered) | %d selected", filteredFiles, filteredOut, selectedCount)
			} else {
				fmt.Fprintf(&sb, "%d items | %d selected", filteredFiles, selectedCount)
			}
		}
		
		if m.inputMode {
			sb.WriteString("\n[enter:apply] [esc:cancel]\n")
		} else {
			sb.WriteString("\n[/:filter] [space:toggle] [a:all] [n:none] [enter:continue] [esc:exit]\n")
		}

	case stateOutputPath:
		sb.WriteString("Enter Output Directory:\n\n")
		sb.WriteString(m.input.View())
		sb.WriteString("\n\n[Enter:confirm] [esc:quit]\n")

	case stateConfirm:
		selectedCount := 0
		for _, f := range m.files {
			if f.selected && !f.disabled {
				selectedCount++
			}
		}
		fmt.Fprintf(&sb, "Export %d files to %s?\n\n", selectedCount, m.outputPath)

		// Check if directory exists
		if _, err := os.Stat(m.outputPath); err == nil {
			sb.WriteString(errorStyle.Render("⚠ Warning: Directory exists and will be overwritten!") + "\n\n")
		}

		sb.WriteString("[Y:confirm] [N:back] [esc:quit]\n")

	case stateProgress:
		fmt.Fprintf(&sb, "Exporting... (%d/%d)\n", m.doneFiles, m.totalFiles)
		sb.WriteString(m.progress.View() + "\n")
		sb.WriteString(statusStyle.Render("Current: "+m.currentFile) + "\n")

	case stateDone:
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render("✓ Export Complete!") + "\n")
		fmt.Fprintf(&sb, "Saved to: %s\n", m.outputPath)
		sb.WriteString("\nPress any key to exit\n")
	}

	if m.err != nil {
		sb.WriteString("\n" + errorStyle.Render(m.err.Error()) + "\n")
	}

	return sb.String()
}
