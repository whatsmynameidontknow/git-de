package git

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	StatusAdded    FileStatus = "A"
	StatusModified FileStatus = "M"
	StatusDeleted  FileStatus = "D"
	StatusRenamed  FileStatus = "R"
	StatusCopied   FileStatus = "C"
)

type FileStatus string

var ErrInvalidCommit = errors.New("invalid commit reference")

type FileChange struct {
	Status  FileStatus
	Path    string
	OldPath string
}

type Commit struct {
	Hash    string
	Message string
}

func (fc FileChange) ShouldCopy() bool {
	return fc.Status == StatusAdded ||
		fc.Status == StatusModified ||
		fc.Status == StatusRenamed ||
		fc.Status == StatusCopied
}

type Client struct {
	workDir string
}

func NewClient(workDir string) *Client {
	var c Client
	c.workDir = workDir
	return &c
}

func (c *Client) IsGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = c.workDir
	return cmd.Run() == nil
}

func (c *Client) HasCommits() bool {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = c.workDir
	return cmd.Run() == nil
}

func (c *Client) ValidateCommit(commit string) error {
	cmd := exec.Command("git", "cat-file", "-t", commit)
	cmd.Dir = c.workDir
	output, err := cmd.Output()
	if err != nil {
		return ErrInvalidCommit
	}
	if strings.TrimSpace(string(output)) != "commit" {
		return ErrInvalidCommit
	}
	return nil
}

func (c *Client) GetChangedFiles(fromCommit, toCommit string) ([]FileChange, error) {
	cmd := exec.Command("git", "diff", "--name-status", "-M", "-C", fromCommit, toCommit)
	cmd.Dir = c.workDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	return c.parseDiffOutput(string(output))
}

func (c *Client) parseDiffOutput(output string) ([]FileChange, error) {
	var changes []FileChange
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.Contains(line, ".git/") {
			continue
		}

		change, err := c.parseLine(line)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}

	return changes, scanner.Err()
}

func (c *Client) parseLine(line string) (FileChange, error) {
	fields := strings.Split(line, "\t")
	if len(fields) < 2 {
		return FileChange{}, fmt.Errorf("invalid diff line: %s", line)
	}

	status := FileStatus(fields[0][0])

	switch status {
	case StatusRenamed, StatusCopied:
		if len(fields) < 3 {
			return FileChange{}, fmt.Errorf("invalid rename/copy line: %s", line)
		}
		return FileChange{
			Status:  status,
			OldPath: fields[1],
			Path:    fields[2],
		}, nil
	default:
		return FileChange{
			Status: status,
			Path:   fields[1],
		}, nil
	}
}

func (c *Client) GetFileContent(commit, path string) ([]byte, error) {
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", commit, path))
	cmd.Dir = c.workDir
	return cmd.Output()
}

func (c *Client) IsFileOutsideRepo(path string) bool {
	if strings.HasPrefix(path, "../") || strings.HasPrefix(path, "..") {
		return true
	}
	if filepath.IsAbs(path) {
		return true
	}
	cleanPath := filepath.Clean(path)

	return strings.HasPrefix(cleanPath, "../")
}

func (c *Client) GetRecentCommits(n int) ([]Commit, error) {
	return c.getCommits("git", "log", "-n", fmt.Sprintf("%d", n), "--pretty=format:%H %s")
}

func (c *Client) GetCommitsAfter(after string, n int) ([]Commit, error) {
	return c.getCommits("git", "log", "-n", fmt.Sprintf("%d", n), "--pretty=format:%H %s", after+"..HEAD")
}

func (c *Client) getCommits(name string, args ...string) ([]Commit, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = c.workDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	var commits []Commit
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			var commit Commit
			commit.Hash = parts[0]
			commit.Message = parts[1]
			commits = append(commits, commit)
		}
	}

	return commits, scanner.Err()
}
