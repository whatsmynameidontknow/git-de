package git

import (
	"bufio"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Branch represents a git branch with metadata.
type Branch struct {
	Name        string
	IsRemote    bool
	IsCurrent   bool
	Ahead       int // -1 means not loaded yet
	Behind      int // -1 means not loaded yet
	LastCommit  time.Time
	LastMessage string
}

// CommitRangeStats holds summary statistics for a range of commits.
type CommitRangeStats struct {
	CommitCount  int
	FilesChanged int
	Additions    int
	Deletions    int
}

// GetDefaultBranch detects the repository's default branch.
// Tries origin/HEAD first, then falls back to main/master.
func (c *Client) GetDefaultBranch() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = c.workDir

	output, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(output))
		// Parse "refs/remotes/origin/main" -> "main"
		parts := strings.Split(ref, "/")
		if len(parts) >= 4 {
			return strings.Join(parts[3:], "/"), nil
		}
	}

	// Fallback: try common default branch names
	for _, branch := range []string{"main", "master"} {
		if c.BranchExists(branch) {
			return branch, nil
		}
	}

	return "", fmt.Errorf("could not determine default branch")
}

// BranchExists checks if a branch ref exists.
func (c *Client) BranchExists(branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = c.workDir
	return cmd.Run() == nil
}

// GetBranches returns all local branches sorted by current first, then by last commit time.
func (c *Client) GetBranches() ([]Branch, error) {
	// Get current branch name
	currentCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	currentCmd.Dir = c.workDir
	currentOut, err := currentCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get current branch: %w", err)
	}
	currentBranch := strings.TrimSpace(string(currentOut))

	// Get all branches with metadata
	cmd := exec.Command("git", "branch", "-a",
		"--format=%(refname:short)|%(committerdate:iso8601)|%(contents:subject)")
	cmd.Dir = c.workDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch failed: %w", err)
	}

	var branches []Branch
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Skip HEAD -> refs
		if strings.Contains(line, "->") {
			continue
		}

		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		dateStr := strings.TrimSpace(parts[1])
		message := strings.TrimSpace(parts[2])

		branch := Branch{
			Name:        name,
			LastMessage: truncateStr(message, 50),
			Ahead:       -1,
			Behind:      -1,
		}

		// Parse time
		if t, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr); err == nil {
			branch.LastCommit = t
		}

		// Determine if remote
		if strings.HasPrefix(name, "origin/") {
			branch.IsRemote = true
		}

		// Check if current
		if name == currentBranch && !branch.IsRemote {
			branch.IsCurrent = true
		}

		branches = append(branches, branch)
	}

	// Sort: current first, then local before remote, then by time (newest first)
	sort.SliceStable(branches, func(i, j int) bool {
		if branches[i].IsCurrent != branches[j].IsCurrent {
			return branches[i].IsCurrent
		}
		if branches[i].IsRemote != branches[j].IsRemote {
			return !branches[i].IsRemote
		}
		return branches[i].LastCommit.After(branches[j].LastCommit)
	})

	return branches, nil
}

// GetBranchAheadBehind returns how many commits a branch is ahead/behind the default branch.
func (c *Client) GetBranchAheadBehind(branch, defaultBranch string) (ahead, behind int, err error) {
	if defaultBranch == "" {
		return -1, -1, fmt.Errorf("no default branch")
	}

	cmd := exec.Command("git", "rev-list", "--left-right", "--count",
		fmt.Sprintf("%s...%s", defaultBranch, branch))
	cmd.Dir = c.workDir

	output, err := cmd.Output()
	if err != nil {
		// Try with origin/ prefix
		cmd = exec.Command("git", "rev-list", "--left-right", "--count",
			fmt.Sprintf("origin/%s...%s", defaultBranch, branch))
		cmd.Dir = c.workDir
		output, err = cmd.Output()
		if err != nil {
			return -1, -1, fmt.Errorf("rev-list failed: %w", err)
		}
	}

	// Format: "behind\tahead\n"
	fields := strings.Fields(strings.TrimSpace(string(output)))
	if len(fields) != 2 {
		return -1, -1, fmt.Errorf("unexpected rev-list output: %q", string(output))
	}

	behind, err = strconv.Atoi(fields[0])
	if err != nil {
		return -1, -1, fmt.Errorf("parse behind count: %w", err)
	}

	ahead, err = strconv.Atoi(fields[1])
	if err != nil {
		return -1, -1, fmt.Errorf("parse ahead count: %w", err)
	}

	return ahead, behind, nil
}

// IsBranchMerged checks if a branch has been fully merged into the default branch.
func (c *Client) IsBranchMerged(branch, defaultBranch string) bool {
	if defaultBranch == "" {
		return false
	}

	cmd := exec.Command("git", "branch", "-a", "--merged", defaultBranch, "--format=%(refname:short)")
	cmd.Dir = c.workDir

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name == branch || name == "origin/"+branch {
			return true
		}
	}

	return false
}

// GetRecentCommitsOnBranch returns recent commits on a branch, excluding merge commits.
func (c *Client) GetRecentCommitsOnBranch(branch string, n int) ([]Commit, error) {
	return c.getCommits("git", "log", branch,
		"-n", fmt.Sprintf("%d", n),
		"--no-merges",
		"--pretty=format:%H %s")
}

// CheckoutBranch checks out the specified branch.
func (c *Client) CheckoutBranch(branch string) error {
	cmd := exec.Command("git", "checkout", branch)
	cmd.Dir = c.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("checkout %s: %s", branch, strings.TrimSpace(string(output)))
	}
	return nil
}

// GetCommitRangeStats returns summary statistics for a commit range.
func (c *Client) GetCommitRangeStats(from, to string) (CommitRangeStats, error) {
	var stats CommitRangeStats

	// Count commits
	countCmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", from, to))
	countCmd.Dir = c.workDir
	countOut, err := countCmd.Output()
	if err != nil {
		return stats, fmt.Errorf("rev-list count: %w", err)
	}
	stats.CommitCount, _ = strconv.Atoi(strings.TrimSpace(string(countOut)))

	// Get diff stats (numstat for precise +/-)
	statCmd := exec.Command("git", "diff", "--numstat", from, to)
	statCmd.Dir = c.workDir
	statOut, err := statCmd.Output()
	if err != nil {
		return stats, fmt.Errorf("diff numstat: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(statOut)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		stats.FilesChanged++

		if add, err := strconv.Atoi(fields[0]); err == nil {
			stats.Additions += add
		}
		if del, err := strconv.Atoi(fields[1]); err == nil {
			stats.Deletions += del
		}
	}

	return stats, nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
