package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the current TUI state.
func (m Model) View() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Git Diff Export") + "\n\n")

	switch m.state {
	case stateCommitLimitSelection:
		sb.WriteString(m.list.View())

	case stateCommitLimitCustom:
		m.viewLimitCustom(&sb)

	case stateFromCommit, stateToCommit:
		sb.WriteString(m.list.View())

	case stateFileSelection:
		m.viewFileSelection(&sb)

	case stateOutputPath:
		m.viewOutputPath(&sb)

	case stateConfirm:
		m.viewConfirm(&sb)

	case stateProgress:
		m.viewProgress(&sb)

	case stateDone:
		m.viewDone(&sb)
	}

	if m.err != nil {
		sb.WriteString("\n" + errorStyle.Render(m.err.Error()) + "\n")
	}

	return sb.String()
}

func (m Model) viewLimitCustom(sb *strings.Builder) {
	sb.WriteString("Enter Custom Commit Limit:\n\n")
	sb.WriteString(m.limitInput.View())
	sb.WriteString("\n\n")
	sb.WriteString(statusStyle.Render("Enter a number between 1 and 999999"))
	sb.WriteString("\n[Enter:confirm] [esc:back]\n")
}

func (m Model) viewFileSelection(sb *strings.Builder) {
	sb.WriteString("Select Files to Export:\n")
	fmt.Fprintf(sb, "Range: %s...%s\n\n", m.shortHash(m.fromCommit), m.shortHash(m.toCommit))

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

	// Pagination
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

	// Render file list
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

			fmt.Fprintf(sb, "%s %s\n", cursor, line)
		}
	}

	// Status line
	m.viewFileStatusLine(sb, displayIdx)

	// Keyboard hints
	if m.inputMode {
		sb.WriteString("\n[enter:apply] [esc:cancel]\n")
	} else {
		sb.WriteString("\n[/:filter] [space:toggle] [a:all] [n:none] [enter:continue] [esc:exit]\n")
	}
}

func (m Model) viewFileStatusLine(sb *strings.Builder, displayIdx []int) {
	filteredFiles := len(displayIdx)
	totalFiles := len(m.files)
	filteredOut := totalFiles - filteredFiles
	hasFilter := m.filterInput.Value() != ""

	if m.inputMode {
		if filteredFiles == 0 {
			fmt.Fprintf(sb, "none matched (%d filtered)", filteredOut)
		} else if hasFilter {
			fmt.Fprintf(sb, "%d items (%d filtered)", filteredFiles, filteredOut)
		} else {
			fmt.Fprintf(sb, "%d items", filteredFiles)
		}
	} else {
		selectedCount := m.selectedFileCount()

		if filteredFiles == 0 {
			fmt.Fprintf(sb, "none matched (%d filtered) | %d selected", filteredOut, selectedCount)
		} else if hasFilter {
			fmt.Fprintf(sb, "%d items (%d filtered) | %d selected", filteredFiles, filteredOut, selectedCount)
		} else {
			fmt.Fprintf(sb, "%d items | %d selected", filteredFiles, selectedCount)
		}
	}
}

func (m Model) viewOutputPath(sb *strings.Builder) {
	sb.WriteString("Enter Output Directory:\n\n")
	sb.WriteString(m.input.View())
	sb.WriteString("\n\n[Enter:confirm] [esc:quit]\n")
}

func (m Model) viewConfirm(sb *strings.Builder) {
	fmt.Fprintf(sb, "Export %d files to %s?\n\n", m.selectedFileCount(), m.outputPath)

	if _, err := os.Stat(m.outputPath); err == nil {
		sb.WriteString(errorStyle.Render("⚠ Warning: Directory exists and will be overwritten!") + "\n\n")
	}

	sb.WriteString("[Y:confirm] [N:back] [esc:quit]\n")
}

func (m Model) viewProgress(sb *strings.Builder) {
	fmt.Fprintf(sb, "Exporting... (%d/%d)\n", m.doneFiles, m.totalFiles)
	sb.WriteString(m.progress.View() + "\n")
	sb.WriteString(statusStyle.Render("Current: "+m.currentFile) + "\n")
}

func (m Model) viewDone(sb *strings.Builder) {
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render("✓ Export Complete!") + "\n")
	fmt.Fprintf(sb, "Saved to: %s\n", m.outputPath)
	sb.WriteString("\nPress any key to exit\n")
}
