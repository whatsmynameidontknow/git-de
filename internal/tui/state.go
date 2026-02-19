package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

type sessionState int

const (
	stateBranchSelection sessionState = iota
	stateCommitLimitSelection
	stateCommitLimitCustom
	stateFromCommit
	stateToCommit
	stateCommitRangeSummary
	stateFileSelection
	stateOutputPath
	stateConfirm
	stateProgress
	stateDone
)

const (
	defaultOutputPath   = "./export"
	defaultCommitLimit  = 50
	commitLimitAll      = 999999
	numWorkers          = 5
	bufferSize          = 20
	concurrentThreshold = 100
)

var (
	topBarBlockStyle    = lipgloss.NewStyle().Padding(0, 1).MarginBottom(1).Background(lipgloss.Color("#7D56F4"))
	topBarItemStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4"))
	topBarOKStatusStyle = topBarItemStyle.Foreground(lipgloss.Color("#00FF00"))
	statusStyle         = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#626262"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AD58B4")).
			Bold(true)
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	totalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#0000FF"))
)

// Messages

type progressMsg struct {
	file         string
	successCount int
	failedCount  int
}

type exportStartedMsg struct {
	ch        <-chan progressMsg
	fileCount int
}

type exportDoneMsg struct{}

type copyError struct {
	msg  error
	path string
}

func (c copyError) Error() string {
	return fmt.Sprintf("%s: %s", c.path, c.msg)
}
