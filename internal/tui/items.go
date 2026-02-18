package tui

import (
	"fmt"
	"time"

	"github.com/whatsmynameidontknow/git-de/internal/git"
)

type branchItem struct {
	branch git.Branch
}

func (b branchItem) Title() string {
	marker := "  "
	if b.branch.IsCurrent {
		marker = "* "
	}

	remote := ""
	if b.branch.IsRemote {
		remote = " (remote)"
	}

	return fmt.Sprintf("%s%s%s", marker, b.branch.Name, remote)
}

func (b branchItem) Description() string {
	aheadBehind := ""
	if b.branch.Ahead >= 0 && b.branch.Behind >= 0 {
		if b.branch.Ahead > 0 || b.branch.Behind > 0 {
			aheadBehind = fmt.Sprintf("↑%d ↓%d  ", b.branch.Ahead, b.branch.Behind)
		}
	} else {
		aheadBehind = ""
	}

	msg := b.branch.LastMessage
	if msg == "" {
		return aheadBehind
	}

	return fmt.Sprintf("%s%s", aheadBehind, msg)
}

func (b branchItem) FilterValue() string {
	return b.branch.Name
}

type commitItem struct {
	time    time.Time
	sha     string
	message string
}

func newCommitItem(c git.Commit) commitItem {
	return commitItem{c.Time, c.Hash, c.Message}
}

func (i commitItem) Title() string { return i.message }
func (i commitItem) Description() string {
	return fmt.Sprintf("%s\t(%s)", i.sha, i.time.Format("02 January 2006 15:04:05"))
}
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
