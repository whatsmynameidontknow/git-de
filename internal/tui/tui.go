package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/whatsmynameidontknow/git-de/internal/git"
	"github.com/whatsmynameidontknow/git-de/internal/exporter"
	"github.com/whatsmynameidontknow/git-de/internal/manifest"
)

type sessionState int

const (
	stateFromCommit sessionState = iota
	stateToCommit
	stateFileSelection
	stateOutputPath
	stateConfirm
	stateProgress
	stateDone
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

func (i commitItem) Title() string       { return i.sha[:7] + " " + i.message }
func (i commitItem) Description() string { return i.sha }
func (i commitItem) FilterValue() string { return i.sha + " " + i.message }

type fileItem struct {
	path     string
	status   git.FileStatus
	selected bool
	disabled bool
	oldPath  string
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
	current int
	total   int
	file    string
}

type Model struct {
	state      sessionState
	gitClient  *git.Client
	err        error
	
	// Inputs
	fromCommit string
	toCommit   string
	outputPath string
	
	// Components
	list       list.Model
	input      textinput.Model
	progress   progress.Model
	
	// Data
	commits       []commitItem
	files         []fileItem
	filteredIdx   []int // indices into files for current filter
	cursor        int
	filterMode    bool
	filterInput   textinput.Model
	
	// Window size
	width  int
	height int
	
	// Progress tracking
	totalFiles int
	doneFiles  int
	currentFile string
}

func NewModel(client *git.Client, from, to string) Model {
	ti := textinput.New()
	ti.Placeholder = "./export"
	ti.SetValue("./export")
	ti.Focus()

	fi := textinput.New()
	fi.Placeholder = "type to filter..."
	fi.Prompt = "/ "

	prog := progress.New(progress.WithDefaultGradient())

	m := Model{
		gitClient:   client,
		input:       ti,
		filterInput: fi,
		progress:    prog,
		fromCommit:  from,
		toCommit:    to,
	}

	if from != "" && to != "" {
		m.state = stateFileSelection
	} else if from != "" {
		m.state = stateToCommit
	} else {
		m.state = stateFromCommit
	}

	return m
}

func Run(client *git.Client, from, to string) error {
	m := NewModel(client, from, to)
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

func (m Model) Init() tea.Cmd {
	switch m.state {
	case stateFromCommit, stateToCommit:
		return m.loadCommitsCmd
	case stateFileSelection:
		return m.loadFilesCmd
	}
	return nil
}

func (m Model) loadCommitsCmd() tea.Msg {
	commits, err := m.gitClient.GetRecentCommits(50)
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
	commits, err := m.gitClient.GetCommitsAfter(m.fromCommit, 50)
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
		for _, f := range selectedFiles {
			// Update UI
			// Note: This is simplified. tea.Cmd should usually return a single message.
			// But since we are in a closure, we can't easily send multiple updates 
			// without a channel or similar. 
			// For TUI, it's better to use a channel or just do it in chunks.
			// However, for this implementation, we'll just do it synchronously 
			// and return the final message, or use a workaround.
			
			exp.CopyFile(f)
			// (Progress update would go here if we used a more complex setup)
		}
		
		// Add summary.txt
		// We'd need all changes for a proper summary, but for now:
		summary := manifest.Generate(selectedFiles)
		summaryPath := filepath.Join(m.outputPath, "summary.txt")
		manifest.WriteToFile(summaryPath, summary)

		return progressMsg{current: total, total: total, file: "Done"}
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
		if m.state == stateFromCommit {
			m.list.Title = "Select From Commit"
		} else {
			m.list.Title = "Select To Commit (after " + m.fromCommit[:7] + ")"
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

	case progressMsg:
		m.doneFiles = msg.current
		m.totalFiles = msg.total
		m.currentFile = msg.file
		m.progress.SetPercent(1.0)
		m.state = stateDone
		return m, nil

	case error:
		m.err = msg
		return m, nil

	default:
		// Forward non-key messages (e.g. FilterMatchesMsg, spinner ticks) to the list
		// so filtering actually works.
		if m.state == stateFromCommit || m.state == stateToCommit {
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
		
		// q only quits when NOT in a filter mode
		if msg.String() == "q" {
			if m.filterMode {
				break
			}
			isListState := m.state == stateFromCommit || m.state == stateToCommit
			if isListState && m.list.SettingFilter() {
				break
			}
			return m, tea.Quit
		}

		switch m.state {
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
			if m.filterMode {
				switch msg.String() {
				case "esc":
					m.filterMode = false
					m.filterInput.Blur()
					return m, nil
				case "enter":
					m.filterMode = false
					m.filterInput.Blur()
					return m, nil
				default:
					m.filterInput, cmd = m.filterInput.Update(msg)
					m.rebuildFilter()
					return m, cmd
				}
			}

			switch msg.String() {
			case "/":
				m.filterMode = true
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
			case "a": // Select all (visible)
				for _, idx := range m.filteredIdx {
					if !m.files[idx].disabled {
						m.files[idx].selected = true
					}
				}
			case "n": // Select none (visible)
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
			}

		case stateOutputPath:
			m.input, cmd = m.input.Update(msg)
			if msg.String() == "enter" {
				m.outputPath = m.input.Value()
				if m.outputPath == "" {
					m.outputPath = "./export"
				}
				m.state = stateConfirm
				return m, nil
			}
			return m, cmd

		case stateConfirm:
			if msg.String() == "y" || msg.String() == "enter" {
				m.state = stateProgress
				return m, m.startExport()
			}
			if msg.String() == "n" || msg.String() == "backspace" {
				m.state = stateOutputPath
				return m, nil
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

func (m Model) View() string {
	var s string
	
	s += titleStyle.Render("git-de") + "\n\n"

	switch m.state {
	case stateFromCommit, stateToCommit:
		s += m.list.View()

	case stateFileSelection:
		s += "Select Files to Export:\n\n"
		
		if m.filterMode || m.filterInput.Value() != "" {
			s += m.filterInput.View() + "\n\n"
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
			if visibleStart < 0 {
				visibleStart = 0
			}
			visibleEnd = visibleStart + maxVisible
			if visibleEnd > len(displayIdx) {
				visibleEnd = len(displayIdx)
				visibleStart = visibleEnd - maxVisible
			}
		}

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
			
			s += fmt.Sprintf("%s %s\n", cursor, line)
		}

		s += fmt.Sprintf("\n%d/%d files", len(displayIdx), len(m.files))
		if m.filterInput.Value() != "" {
			s += fmt.Sprintf(" (filter: %s)", m.filterInput.Value())
		}
		s += "\n[/:filter] [Space:toggle] [A:all] [N:none] [Enter:continue] [Q:quit]\n"

	case stateOutputPath:
		s += "Enter Output Directory:\n\n"
		s += m.input.View()
		s += "\n\n[Tab:complete] [Enter:confirm] [Q:quit]\n"

	case stateConfirm:
		selectedCount := 0
		for _, f := range m.files {
			if f.selected && !f.disabled {
				selectedCount++
			}
		}
		s += fmt.Sprintf("Export %d files to %s?\n\n", selectedCount, m.outputPath)
		
		// Check if directory exists
		if _, err := os.Stat(m.outputPath); err == nil {
			s += errorStyle.Render("⚠ Warning: Directory exists and will be overwritten!") + "\n\n"
		}
		
		s += "[Y:confirm] [N:back] [Q:quit]\n"

	case stateProgress:
		s += fmt.Sprintf("Exporting... (%d/%d)\n", m.doneFiles, m.totalFiles)
		s += m.progress.View() + "\n"
		s += statusStyle.Render("Current: "+m.currentFile) + "\n"

	case stateDone:
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render("✓ Export Complete!") + "\n"
		s += fmt.Sprintf("Saved to: %s\n", m.outputPath)
		s += "\nPress any key to exit\n"
	}

	if m.err != nil {
		s += "\n" + errorStyle.Render(m.err.Error()) + "\n"
	}

	return s
}
