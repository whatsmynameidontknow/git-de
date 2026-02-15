package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/whatsmynameidontknow/git-de/internal/git"
)

func TestNewModel_NoCommits(t *testing.T) {
	m := NewModel(nil, "", "")
	if m.state != stateBranchSelection {
		t.Errorf("Expected state stateBranchSelection, got %d", m.state)
	}
	if m.fromCommit != "" {
		t.Errorf("Expected empty fromCommit, got %s", m.fromCommit)
	}
	if m.commitLimit != defaultCommitLimit {
		t.Errorf("Expected commitLimit %d, got %d", defaultCommitLimit, m.commitLimit)
	}
}

func TestNewModel_WithFromCommit(t *testing.T) {
	m := NewModel(nil, "abc123", "")
	if m.state != stateToCommit {
		t.Errorf("Expected state stateToCommit, got %d", m.state)
	}
	if m.fromCommit != "abc123" {
		t.Errorf("Expected fromCommit abc123, got %s", m.fromCommit)
	}
}

func TestNewModel_WithBothCommits(t *testing.T) {
	m := NewModel(nil, "abc123", "def456")
	if m.state != stateCommitRangeSummary {
		t.Errorf("Expected state stateCommitRangeSummary, got %d", m.state)
	}
	if m.fromCommit != "abc123" {
		t.Errorf("Expected fromCommit abc123, got %s", m.fromCommit)
	}
	if m.toCommit != "def456" {
		t.Errorf("Expected toCommit def456, got %s", m.toCommit)
	}
}

func TestUpdate_EarlyKeyPressDoesNotPanic(t *testing.T) {
	m := NewModel(nil, "", "")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Update panicked on early key press: %v", r)
		}
	}()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)

	if model.state != stateBranchSelection {
		t.Errorf("Expected state stateBranchSelection, got %d", model.state)
	}
}

func TestUpdate_FilesLoaded(t *testing.T) {
	m := NewModel(nil, "abc", "def")

	files := []fileItem{
		{path: "main.go", status: git.StatusAdded, selected: true, disabled: false},
		{path: "old.go", status: git.StatusDeleted, selected: false, disabled: true},
		{path: "utils.go", status: git.StatusModified, selected: true, disabled: false},
	}

	updated, _ := m.Update(files)
	model := updated.(Model)

	if model.state != stateFileSelection {
		t.Errorf("Expected state stateFileSelection, got %d", model.state)
	}
	if len(model.files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(model.files))
	}
	if model.files[1].disabled != true {
		t.Error("Expected deleted file to be disabled")
	}
}

func TestUpdate_FileSelection_Toggle(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateFileSelection
	m.files = []fileItem{
		{path: "main.go", status: git.StatusAdded, selected: true, disabled: false},
		{path: "old.go", status: git.StatusDeleted, selected: false, disabled: true},
	}
	m.cursor = 0

	// Toggle first file off
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model := updated.(Model)

	if model.files[0].selected != false {
		t.Error("Expected first file to be deselected after toggle")
	}

	// Toggle back on
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model = updated.(Model)

	if model.files[0].selected != true {
		t.Error("Expected first file to be selected after second toggle")
	}
}

func TestUpdate_FileSelection_ToggleDisabled(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateFileSelection
	m.files = []fileItem{
		{path: "old.go", status: git.StatusDeleted, selected: false, disabled: true},
	}
	m.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model := updated.(Model)

	if model.files[0].selected != false {
		t.Error("Expected disabled file to remain unselected")
	}
}

func TestUpdate_FileSelection_SelectAll(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateFileSelection
	m.files = []fileItem{
		{path: "main.go", status: git.StatusAdded, selected: false, disabled: false},
		{path: "old.go", status: git.StatusDeleted, selected: false, disabled: true},
		{path: "utils.go", status: git.StatusModified, selected: false, disabled: false},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model := updated.(Model)

	if model.files[0].selected != true {
		t.Error("Expected main.go to be selected")
	}
	if model.files[1].selected != false {
		t.Error("Expected disabled file to remain unselected")
	}
	if model.files[2].selected != true {
		t.Error("Expected utils.go to be selected")
	}
}

func TestUpdate_FileSelection_SelectNone(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateFileSelection
	m.files = []fileItem{
		{path: "main.go", status: git.StatusAdded, selected: true, disabled: false},
		{path: "utils.go", status: git.StatusModified, selected: true, disabled: false},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := updated.(Model)

	if model.files[0].selected != false {
		t.Error("Expected main.go to be deselected")
	}
	if model.files[1].selected != false {
		t.Error("Expected utils.go to be deselected")
	}
}

func TestUpdate_FileSelection_Navigation(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateFileSelection
	m.files = []fileItem{
		{path: "a.go", status: git.StatusAdded, selected: true},
		{path: "b.go", status: git.StatusAdded, selected: true},
		{path: "c.go", status: git.StatusAdded, selected: true},
	}
	m.cursor = 0

	// Move down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := updated.(Model)
	if model.cursor != 1 {
		t.Errorf("Expected cursor at 1, got %d", model.cursor)
	}

	// Move down again
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.cursor != 2 {
		t.Errorf("Expected cursor at 2, got %d", model.cursor)
	}

	// Move down past end (should stay)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.cursor != 2 {
		t.Errorf("Expected cursor to stay at 2, got %d", model.cursor)
	}

	// Move up
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(Model)
	if model.cursor != 1 {
		t.Errorf("Expected cursor at 1, got %d", model.cursor)
	}
}

func TestUpdate_FileSelection_EnterGoesToOutputPath(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateFileSelection
	m.files = []fileItem{
		{path: "main.go", status: git.StatusAdded, selected: true},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.state != stateOutputPath {
		t.Errorf("Expected state stateOutputPath, got %d", model.state)
	}
}

func TestUpdate_Confirm_BackGoesToOutputPath(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateConfirm
	m.outputPath = "./export"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := updated.(Model)

	if model.state != stateOutputPath {
		t.Errorf("Expected state stateOutputPath after 'n', got %d", model.state)
	}
}

func TestUpdate_CommitsLoaded(t *testing.T) {
	m := NewModel(nil, "", "")
	m.state = stateFromCommit

	items := []list.Item{
		commitItem{sha: "abc1234567890", message: "first commit"},
		commitItem{sha: "def4567890123", message: "second commit"},
	}

	updated, _ := m.Update(items)
	model := updated.(Model)

	if model.list.Title != "Select From Commit" {
		t.Errorf("Expected list title 'Select From Commit', got %s", model.list.Title)
	}
}

func TestUpdate_CommitsLoaded_WithBranch(t *testing.T) {
	m := NewModel(nil, "", "")
	m.state = stateFromCommit
	m.selectedBranch = "feature/auth"

	items := []list.Item{
		commitItem{sha: "abc1234567890", message: "first commit"},
	}

	updated, _ := m.Update(items)
	model := updated.(Model)

	expected := "Select From Commit (on feature/auth)"
	if model.list.Title != expected {
		t.Errorf("Expected list title %q, got %q", expected, model.list.Title)
	}
}

func TestUpdate_ToCommitsLoaded_WithBranch(t *testing.T) {
	m := NewModel(nil, "", "")
	m.state = stateToCommit
	m.selectedBranch = "feature/auth"
	m.fromCommit = "abc1234567890"

	items := []list.Item{
		commitItem{sha: "def4567890123", message: "second commit"},
	}

	updated, _ := m.Update(items)
	model := updated.(Model)

	if !strings.Contains(model.list.Title, "feature/auth") {
		t.Errorf("Expected branch name in title, got %q", model.list.Title)
	}
	if !strings.Contains(model.list.Title, "abc1234") {
		t.Errorf("Expected from commit hash in title, got %q", model.list.Title)
	}
}

func TestUpdate_LimitOptionsLoaded(t *testing.T) {
	m := NewModel(nil, "", "")
	m.state = stateCommitLimitSelection

	items := []list.Item{
		limitOption{label: "50 commits (default)", value: 50},
	}

	updated, _ := m.Update(items)
	model := updated.(Model)

	if model.list.Title != "Select Commit History Depth" {
		t.Errorf("Expected list title 'Select Commit History Depth', got %s", model.list.Title)
	}
}

func TestUpdate_ErrorHandling(t *testing.T) {
	m := NewModel(nil, "", "")

	updated, _ := m.Update(fmt.Errorf("something went wrong"))
	model := updated.(Model)

	if model.err == nil {
		t.Error("Expected error to be set")
	}
	if model.err.Error() != "something went wrong" {
		t.Errorf("Expected 'something went wrong', got %s", model.err.Error())
	}
}

func TestUpdate_ProgressComplete(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateProgress
	m.totalFiles = 6

	updated, _ := m.Update(progressMsg{successCount: 5, failedCount: 1, file: "Done"})
	model := updated.(Model)

	if model.state != stateDone {
		t.Errorf("Expected state stateDone, got %d", model.state)
	}
	if model.successCount != 5 {
		t.Errorf("Expected doneFiles 5, got %d", model.successCount)
	}
}

func TestView_FileSelection(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateFileSelection
	m.files = []fileItem{
		{path: "main.go", status: git.StatusAdded, selected: true},
		{path: "old.go", status: git.StatusDeleted, selected: false, disabled: true},
	}
	m.cursor = 0

	view := m.View()

	if !strings.Contains(view, "Select Files") {
		t.Error("Expected 'Select Files' in view")
	}
	if !strings.Contains(view, "main.go") {
		t.Error("Expected 'main.go' in view")
	}
	if !strings.Contains(view, "old.go") {
		t.Error("Expected 'old.go' in view")
	}
	if !strings.Contains(view, "[space:toggle]") {
		t.Error("Expected keyboard shortcuts in view")
	}
}

func TestView_Confirm(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateConfirm
	m.outputPath = "./my-export"
	m.files = []fileItem{
		{path: "main.go", status: git.StatusAdded, selected: true},
		{path: "utils.go", status: git.StatusModified, selected: true},
		{path: "old.go", status: git.StatusDeleted, selected: false, disabled: true},
	}

	view := m.View()

	if !strings.Contains(view, "Export 2 files") {
		t.Error("Expected 'Export 2 files' in view (only non-disabled selected)")
	}
	if !strings.Contains(view, "./my-export") {
		t.Error("Expected output path in view")
	}
}

func TestView_Done(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateDone
	m.totalFiles = 20
	m.successCount = 15
	m.failedCount = 5
	m.outputPath = "./export"

	view := m.View()

	if !strings.Contains(view, "Summary") {
		t.Error("Expected 'Summary' in view")
	}
	if !strings.Contains(view, "./export") {
		t.Error("Expected output path in view")
	}
	if !strings.Contains(view, totalStyle.Render("Total Files:\t20 files")) {
		t.Error("Expected number of total files in view to be 20")
	}
	if !strings.Contains(view, successStyle.Render("Success Count:\t15 files")) {
		t.Error("Expected number of success count in view to be 15")
	}
	if !strings.Contains(view, errorStyle.Render("Failed Count:\t5 files")) {
		t.Error("Expected number of failed count in view to be 5")
	}
}

func TestFileItem_Title(t *testing.T) {
	tests := []struct {
		name     string
		item     fileItem
		contains string
	}{
		{
			name:     "added file selected",
			item:     fileItem{path: "main.go", status: git.StatusAdded, selected: true},
			contains: "[✓] A: main.go",
		},
		{
			name:     "modified file unselected",
			item:     fileItem{path: "utils.go", status: git.StatusModified, selected: false},
			contains: "[ ] M: utils.go",
		},
		{
			name:     "deleted file disabled",
			item:     fileItem{path: "old.go", status: git.StatusDeleted, disabled: true},
			contains: "[✗] D: old.go",
		},
		{
			name:     "renamed file shows old path",
			item:     fileItem{path: "new.go", status: git.StatusRenamed, selected: true, oldPath: "old.go"},
			contains: "(from old.go)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title := tt.item.Title()
			if !strings.Contains(title, tt.contains) {
				t.Errorf("Title() = %q, expected to contain %q", title, tt.contains)
			}
		})
	}
}

func TestCommitItem_Title(t *testing.T) {
	item := commitItem{sha: "abc1234567890abcdef", message: "feat: add feature"}
	title := item.Title()

	if title != "feat: add feature" {
		t.Errorf("Expected title to be message only, got %s", title)
	}
}

func TestValidateCommitLimit(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "valid number", input: "50", want: 50},
		{name: "minimum", input: "1", want: 1},
		{name: "maximum", input: "999999", want: 999999},
		{name: "with spaces", input: "  100  ", want: 100},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative", input: "-5", wantErr: true},
		{name: "too large", input: "1000000", wantErr: true},
		{name: "not a number", input: "abc", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "decimal", input: "3.14", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateCommitLimit(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input %q, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
				if got != tt.want {
					t.Errorf("Expected %d, got %d", tt.want, got)
				}
			}
		})
	}
}

func TestUpdate_LimitSelection_Predefined(t *testing.T) {
	m := NewModel(nil, "", "")
	m.state = stateCommitLimitSelection

	// Load limit options into the list
	var items []list.Item
	for _, opt := range commitLimitOptions {
		items = append(items, opt)
	}
	updated, _ := m.Update(items)
	model := updated.(Model)

	// Select the first item (10 commits) by pressing enter
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if model.state != stateFromCommit {
		t.Errorf("Expected state stateFromCommit, got %d", model.state)
	}
	if model.commitLimit != 10 {
		t.Errorf("Expected commitLimit 10, got %d", model.commitLimit)
	}
	if cmd == nil {
		t.Error("Expected loadCommitsCmd to be returned")
	}
}

func TestUpdate_LimitSelection_Custom(t *testing.T) {
	m := NewModel(nil, "", "")
	m.state = stateCommitLimitSelection

	// Load limit options into the list
	var items []list.Item
	for _, opt := range commitLimitOptions {
		items = append(items, opt)
	}
	updated, _ := m.Update(items)
	model := updated.(Model)

	// Navigate to Custom... (last item, index 5)
	for i := 0; i < 5; i++ {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}

	// Select Custom...
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if model.state != stateCommitLimitCustom {
		t.Errorf("Expected state stateCommitLimitCustom, got %d", model.state)
	}
}

func TestUpdate_LimitCustom_EscapeGoesBack(t *testing.T) {
	m := NewModel(nil, "", "")
	m.state = stateCommitLimitCustom

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.state != stateCommitLimitSelection {
		t.Errorf("Expected state stateCommitLimitSelection, got %d", model.state)
	}
	if model.err != nil {
		t.Errorf("Expected err to be cleared, got %v", model.err)
	}
}

func TestView_LimitCustom(t *testing.T) {
	m := NewModel(nil, "", "")
	m.state = stateCommitLimitCustom

	view := m.View()

	if !strings.Contains(view, "Enter Custom Commit Limit") {
		t.Error("Expected 'Enter Custom Commit Limit' in view")
	}
	if !strings.Contains(view, "1 and 999999") {
		t.Error("Expected range hint in view")
	}
	if !strings.Contains(view, "[esc:back]") {
		t.Error("Expected escape hint in view")
	}
}

func TestUpdate_ShortCommitHashDoesNotPanic(t *testing.T) {
	// Create a model with a short commit hash (length < 7)
	m := NewModel(nil, "HEAD~5", "")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Update panicked on short commit hash: %v", r)
		}
	}()

	// Send list items to trigger the title update in Model.Update
	items := []list.Item{
		commitItem{sha: "abc123456", message: "commit"},
	}
	m.Update(items)
}

func TestInit_States(t *testing.T) {
	tests := []struct {
		name          string
		from          string
		to            string
		expectedState sessionState
	}{
		{
			name:          "no args starts with branch selection",
			from:          "",
			to:            "",
			expectedState: stateBranchSelection,
		},
		{
			name:          "from arg starts with to-commit selection screen",
			from:          "abc",
			to:            "",
			expectedState: stateToCommit,
		},
		{
			name:          "both args starts with commit range summary",
			from:          "abc",
			to:            "def",
			expectedState: stateCommitRangeSummary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(nil, tt.from, tt.to)
			if m.state != tt.expectedState {
				t.Errorf("Expected state %d, got %d", tt.expectedState, m.state)
			}
		})
	}
}

func TestView_FileSelection_ShowsCommits(t *testing.T) {
	m := NewModel(nil, "abc1234567", "def4567890")
	m.state = stateFileSelection
	m.files = []fileItem{
		{path: "main.go", status: git.StatusAdded, selected: true},
	}

	view := m.View()

	if !strings.Contains(view, "abc1234") {
		t.Error("Expected from commit hash in view")
	}
	if !strings.Contains(view, "def4567") {
		t.Error("Expected to commit hash in view")
	}
}

func TestView_FileSelection_ShowsSelectedCount(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateFileSelection
	m.files = []fileItem{
		{path: "main.go", status: git.StatusAdded, selected: true},
		{path: "utils.go", status: git.StatusModified, selected: true},
		{path: "old.go", status: git.StatusDeleted, selected: false, disabled: true},
	}

	view := m.View()

	if !strings.Contains(view, "2 selected") {
		t.Error("Expected '2 selected' in view")
	}
}

// --- Branch Selection Tests ---

func TestUpdate_BranchesLoaded(t *testing.T) {
	m := NewModel(nil, "", "")
	// state is stateBranchSelection by default

	items := []list.Item{
		branchItem{branch: git.Branch{Name: "main", IsCurrent: true, LastMessage: "initial"}},
		branchItem{branch: git.Branch{Name: "feature/auth", LastMessage: "add auth"}},
	}

	updated, _ := m.Update(items)
	model := updated.(Model)

	if model.list.Title != "Select Branch" {
		t.Errorf("Expected list title 'Select Branch', got %s", model.list.Title)
	}
}

func TestUpdate_BranchSelection_EnterSelectsBranch(t *testing.T) {
	m := NewModel(nil, "", "")

	// Load branches
	items := []list.Item{
		branchItem{branch: git.Branch{Name: "main", IsCurrent: true, LastMessage: "initial"}},
		branchItem{branch: git.Branch{Name: "feature/auth", LastMessage: "add auth"}},
	}
	updated, _ := m.Update(items)
	model := updated.(Model)

	// Select first branch (main)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if model.state != stateCommitLimitSelection {
		t.Errorf("Expected state stateCommitLimitSelection, got %d", model.state)
	}
	if model.selectedBranch != "main" {
		t.Errorf("Expected selectedBranch 'main', got %q", model.selectedBranch)
	}
	if cmd == nil {
		t.Error("Expected loadLimitOptionsCmd to be returned")
	}
}

func TestBranchItem_Title(t *testing.T) {
	tests := []struct {
		name     string
		branch   git.Branch
		expected string
	}{
		{
			name:     "current branch",
			branch:   git.Branch{Name: "main", IsCurrent: true},
			expected: "* main",
		},
		{
			name:     "non-current branch",
			branch:   git.Branch{Name: "feature/auth"},
			expected: "  feature/auth",
		},
		{
			name:     "remote branch",
			branch:   git.Branch{Name: "origin/develop", IsRemote: true},
			expected: "  origin/develop (remote)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := branchItem{branch: tt.branch}
			if item.Title() != tt.expected {
				t.Errorf("Title() = %q, expected %q", item.Title(), tt.expected)
			}
		})
	}
}

func TestBranchItem_Description(t *testing.T) {
	tests := []struct {
		name     string
		branch   git.Branch
		contains string
	}{
		{
			name:     "with ahead/behind",
			branch:   git.Branch{Name: "feature", Ahead: 3, Behind: 1, LastMessage: "some msg"},
			contains: "↑3 ↓1",
		},
		{
			name:     "not loaded yet",
			branch:   git.Branch{Name: "feature", Ahead: -1, Behind: -1, LastMessage: "some msg"},
			contains: "some msg",
		},
		{
			name:     "zero ahead/behind shows nothing",
			branch:   git.Branch{Name: "feature", Ahead: 0, Behind: 0, LastMessage: "msg"},
			contains: "msg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := branchItem{branch: tt.branch}
			desc := item.Description()
			if !strings.Contains(desc, tt.contains) {
				t.Errorf("Description() = %q, expected to contain %q", desc, tt.contains)
			}
		})
	}
}

func TestBranchItem_FilterValue(t *testing.T) {
	item := branchItem{branch: git.Branch{Name: "feature/auth"}}
	if item.FilterValue() != "feature/auth" {
		t.Errorf("FilterValue() = %q, expected 'feature/auth'", item.FilterValue())
	}
}

func TestView_CommitRangeSummary(t *testing.T) {
	m := NewModel(nil, "abc1234567", "def4567890")
	m.state = stateCommitRangeSummary
	m.selectedBranch = "feature/auth"
	m.rangeStats = git.CommitRangeStats{
		CommitCount:  12,
		FilesChanged: 47,
		Additions:    1234,
		Deletions:    567,
	}

	view := m.View()

	if !strings.Contains(view, "feature/auth") {
		t.Error("Expected branch name in view")
	}
	if !strings.Contains(view, "abc1234") {
		t.Error("Expected from commit in view")
	}
	if !strings.Contains(view, "def4567") {
		t.Error("Expected to commit in view")
	}
	if !strings.Contains(view, "12") {
		t.Error("Expected commit count in view")
	}
	if !strings.Contains(view, "47") {
		t.Error("Expected files changed in view")
	}
	if !strings.Contains(view, "+1234") {
		t.Error("Expected additions in view")
	}
	if !strings.Contains(view, "-567") {
		t.Error("Expected deletions in view")
	}
}

func TestUpdate_CommitRangeSummary_EnterProceedsToFileSelection(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateCommitRangeSummary
	m.rangeStats = git.CommitRangeStats{CommitCount: 5}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	// Enter should trigger loadFilesCmd (can't check state directly since cmd is async)
	_ = model
	if cmd == nil {
		t.Error("Expected loadFilesCmd to be returned")
	}
}

func TestNewModel_WithBothCommits_AutoDetectsBranch(t *testing.T) {
	m := NewModel(nil, "abc123", "def456")
	// selectedBranch should be empty since we don't have a git client here
	// but Init should attempt to detect it
	if m.state != stateCommitRangeSummary {
		t.Errorf("Expected state stateCommitRangeSummary, got %d", m.state)
	}
}

func TestUpdate_CommitRangeSummary_BackspaceGoesToToCommit(t *testing.T) {
	m := NewModel(nil, "abc", "def")
	m.state = stateCommitRangeSummary
	m.rangeStats = git.CommitRangeStats{CommitCount: 5}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	model := updated.(Model)

	if model.state != stateToCommit {
		t.Errorf("Expected state stateToCommit, got %d", model.state)
	}
	if cmd == nil {
		t.Error("Expected loadToCommitsCmd to be returned")
	}
}
