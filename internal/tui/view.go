package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// View renders the current TUI state.
func (m Model) View() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Git Diff Export") + "\n\n")

	switch m.state {
	case stateBranchSelection:
		sb.WriteString(m.list.View())

	case stateCommitLimitSelection:
		sb.WriteString(m.list.View())

	case stateCommitLimitCustom:
		m.viewLimitCustom(&sb)

	case stateFromCommit, stateToCommit:
		sb.WriteString(m.list.View())

	case stateCommitRangeSummary:
		m.viewCommitRangeSummary(&sb)

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

func (m Model) viewCommitRangeSummary(sb *strings.Builder) {
	sb.WriteString("Commit Range Summary:\n\n")

	if m.selectedBranch != "" {
		fmt.Fprintf(sb, "Branch:        %s\n", m.selectedBranch)
	}
	fmt.Fprintf(sb, "From:          %s\n", m.shortHash(m.fromCommit))
	fmt.Fprintf(sb, "To:            %s\n", m.shortHash(m.toCommit))
	sb.WriteString("\n")
	fmt.Fprintf(sb, "Commits:       %s\n", totalStyle.Render(strconv.Itoa(m.rangeStats.CommitCount)))
	fmt.Fprintf(sb, "Files changed: %s\n", totalStyle.Render(strconv.Itoa(m.rangeStats.FilesChanged)))
	fmt.Fprintf(sb, "Additions:     %s\n", successStyle.Render(fmt.Sprintf("+%d", m.rangeStats.Additions)))
	fmt.Fprintf(sb, "Deletions:     %s\n", errorStyle.Render(fmt.Sprintf("-%d", m.rangeStats.Deletions)))
	sb.WriteString("\n[enter:proceed] [backspace:change range] [esc:quit]\n")
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
		sb.WriteString("\n[/:filter] [space:toggle] [a:all] [n:none] [c:clear filter] [backspace:back] [enter:continue] [esc:exit]\n")
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
	if m.outputInputFocused {
		sb.WriteString("\n\n[enter:confirm] [esc:blur]\n")
	} else {
		sb.WriteString("\n\n[any key:focus] [backspace:back] [esc:quit]\n")
	}
}

func (m Model) viewConfirm(sb *strings.Builder) {
	fmt.Fprintf(sb, "Export %d files to %s?\n\n", m.selectedFileCount(), m.outputPath)

	if _, err := os.Stat(m.outputPath); err == nil {
		sb.WriteString(warningStyle.Render("⚠ Warning: Directory exists and will be overwritten!") + "\n\n")
	}

	sb.WriteString("[Y:confirm] [N/backspace:back] [esc:quit]\n")
}

func (m Model) viewProgress(sb *strings.Builder) {
	fmt.Fprintf(sb, "Exporting %d/%d... (%s)\n", m.successCount, m.totalFiles, errorStyle.Render(fmt.Sprintf("%d failed", m.failedCount)))
	sb.WriteString(m.progress.View() + "\n")
	sb.WriteString(statusStyle.Render("Current: "+m.currentFile) + "\n")
}

func (m Model) viewDone(sb *strings.Builder) {
	sb.WriteString("Summary:\n")
	fmt.Fprint(sb, totalStyle.Render(fmt.Sprintf("- Total Files:\t%d files", m.totalFiles))+"\n")
	fmt.Fprint(sb, successStyle.Render(fmt.Sprintf("- Success Count:\t%d files", m.successCount))+"\n")
	fmt.Fprint(sb, errorStyle.Render(fmt.Sprintf("- Failed Count:\t%d files", m.failedCount))+"\n")
	fmt.Fprintf(sb, "Saved to: %s\n", m.outputPath)
	if m.failedCount > 0 {
		fmt.Fprintf(sb, "List of failed files saved to: %s\n", filepath.Join(m.outputPath, "errors.txt"))
	}
	sb.WriteString("\nPress any key to exit\n")
}
