package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/whatsmynameidontknow/git-de/internal/exporter"
	"github.com/whatsmynameidontknow/git-de/internal/git"
)

// Model is the top-level Bubble Tea model for the TUI.
type Model struct {
	state     sessionState
	gitClient gitClient
	err       error
	titleText string

	// Branch selection
	selectedBranch string

	// Inputs
	fromCommit string
	toCommit   string
	outputPath string

	// Commit limit
	commitLimit int
	limitInput  textinput.Model

	// Commit range stats
	rangeStats git.CommitRangeStats

	// Components
	list     list.Model
	input    textinput.Model
	progress progress.Model

	// Data
	files       []fileItem
	filteredIdx []int // indices into files for current filter
	cursor      int
	inputMode   bool // for file filter
	filterInput textinput.Model

	// Inclusive mode (include FROM commit changes by using commit^)
	inclusiveMode bool

	// Output path input focus
	outputInputFocused bool

	// Window size
	width  int
	height int

	// Progress tracking
	totalFiles   int
	successCount int
	failedCount  int
	currentFile  string
	progressCh   <-chan progressMsg
}

type gitClient interface {
	GetCurrentBranch() (branch string, err error)
	GetBranchesWithAheadBehind() (branches []git.Branch, err error)
	GetRecentCommitsOnBranch(branch string, n int) (commits []git.Commit, err error)
	GetCommitRangeStats(from, to string) (stats git.CommitRangeStats, err error)
	GetRecentCommits(n int) (commits []git.Commit, err error)
	GetCommitsAfter(from string, n int) (commits []git.Commit, err error)
	CheckoutBranch(branch string) (err error)
	IsValid(sha string) (ok bool)
	exporter.GitExporter
}

// NewModel creates a new TUI model with optional pre-filled commit range.
func NewModel(client gitClient, from, to, version string) (Model, error) {
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
		titleText:   "Git Diff Export " + version,
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
	branch, err := client.GetCurrentBranch()
	if err != nil {
		return Model{}, err
	}
	m.selectedBranch = branch

	if from != "" && to != "" {
		m.state = stateCommitRangeSummary
	} else if from != "" {
		m.state = stateToCommit
	} else {
		m.state = stateCommitLimitSelection
	}

	return m, nil
}

// Run starts the TUI program.
func Run(client *git.Client, from, to, version string) error {
	m, err := NewModel(client, from, to, version)
	if err != nil {
		return err
	}
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}

// Init returns the initial command for the Bubble Tea program.
func (m Model) Init() tea.Cmd {
	switch m.state {
	case stateBranchSelection:
		return m.loadBranchesCmd
	case stateCommitLimitSelection:
		return m.loadLimitOptionsCmd
	case stateCommitRangeSummary:
		return m.loadRangeStatsCmd
	case stateToCommit:
		return m.loadToCommitsCmd
	case stateFileSelection:
		return m.loadFilesCmd
	default:
		return m.loadCommitsCmd
	}
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

func (m Model) shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

// selectedFileCount returns the number of non-disabled selected files.
func (m Model) selectedFileCount() int {
	count := 0
	for _, f := range m.files {
		if f.selected && !f.disabled {
			count++
		}
	}
	return count
}
