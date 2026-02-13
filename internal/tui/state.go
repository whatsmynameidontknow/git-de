package tui

import (
	"github.com/charmbracelet/lipgloss"
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

// Messages

type progressMsg struct {
	file    string
	current int
	total   int
}

type exportStartedMsg struct {
	ch <-chan progressMsg
}

type exportDoneMsg struct{}
