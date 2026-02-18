package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}

func setupTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@test.com")
	runGit(t, tmpDir, "config", "user.name", "Test")
	return tmpDir
}

func TestNewClient(t *testing.T) {
	t.Run("creates client with working directory", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		client := NewClient(repoDir)
		if client.workDir != repoDir {
			t.Errorf("Expected workDir to be %s, got %s", repoDir, client.workDir)
		}
	})
}

func TestClient_IsGitRepository(t *testing.T) {
	t.Run("returns true for git repository", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		client := NewClient(repoDir)
		if !client.IsGitRepository() {
			t.Error("Expected IsGitRepository to return true for valid repo")
		}
	})

	t.Run("returns false for non-git directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		client := NewClient(tmpDir)
		if client.IsGitRepository() {
			t.Error("Expected IsGitRepository to return false for non-repo")
		}
	})
}

func TestClient_HasCommits(t *testing.T) {
	t.Run("returns false for repo with no commits", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		client := NewClient(repoDir)
		if client.HasCommits() {
			t.Error("Expected HasCommits to return false for empty repo")
		}
	})

	t.Run("returns true for repo with commits", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		client := NewClient(repoDir)
		os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
		runGit(t, repoDir, "add", ".")
		runGit(t, repoDir, "commit", "-m", "initial")
		if !client.HasCommits() {
			t.Error("Expected HasCommits to return true after first commit")
		}
	})
}

func TestClient_ValidateCommit(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	os.WriteFile(filepath.Join(repoDir, "file1.txt"), []byte("content1"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "first")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ := cmd.Output()
	commitHash := strings.TrimSpace(string(out))

	os.WriteFile(filepath.Join(repoDir, "file2.txt"), []byte("content2"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "second")

	tests := []struct {
		name    string
		commit  string
		wantErr bool
	}{
		{name: "valid HEAD", commit: "HEAD", wantErr: false},
		{name: "valid full hash", commit: commitHash, wantErr: false},
		{name: "valid HEAD~1", commit: "HEAD~1", wantErr: false},
		{name: "invalid commit hash", commit: "invalid1234567890", wantErr: true},
		{name: "non-existent ref", commit: "nonexistent-branch", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.ValidateCommit(tt.commit)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCommit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_GetChangedFiles(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	os.WriteFile(filepath.Join(repoDir, "file1.txt"), []byte("content1"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "first")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ := cmd.Output()
	firstCommit := strings.TrimSpace(string(out))

	os.WriteFile(filepath.Join(repoDir, "file2.txt"), []byte("new file"), 0o644)
	os.WriteFile(filepath.Join(repoDir, "with space.txt"), []byte("name contains space"), 0o644)
	os.WriteFile(filepath.Join(repoDir, "file1.txt"), []byte("modified"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "second")

	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ = cmd.Output()
	secondCommit := strings.TrimSpace(string(out))

	t.Run("returns changes between commits", func(t *testing.T) {
		files, err := client.GetChangedFiles(firstCommit, secondCommit)
		if err != nil {
			t.Fatalf("GetChangedFiles() failed: %v", err)
		}

		if len(files) != 3 {
			t.Errorf("Expected 3 changes, got %d", len(files))
		}

		foundAddedCount := 0
		foundModifiedCount := 0
		for _, f := range files {
			if f.Path == "file2.txt" && f.Status == StatusAdded {
				foundAddedCount++
			}
			if f.Path == "with space.txt" && f.Status == StatusAdded {
				foundAddedCount++
			}
			if f.Path == "file1.txt" && f.Status == StatusModified {
				foundModifiedCount++
			}
		}

		if foundAddedCount != 2 {
			t.Errorf("Expected 2 files to be added, got %d", foundAddedCount)
		}
		if foundModifiedCount != 1 {
			t.Errorf("Expected 1 file to be modified, got %d", foundModifiedCount)
		}
	})

	t.Run("ignores .git directory", func(t *testing.T) {
		files, err := client.GetChangedFiles(firstCommit, secondCommit)
		if err != nil {
			t.Fatalf("GetChangedFiles() failed: %v", err)
		}

		for _, f := range files {
			if strings.HasPrefix(f.Path, ".git/") {
				t.Errorf("Should not include .git directory files, got: %s", f.Path)
			}
		}
	})
}

func TestClient_GetChangedFiles_WithRenames(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	os.WriteFile(filepath.Join(repoDir, "oldname.txt"), []byte("content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "first")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ := cmd.Output()
	firstCommit := strings.TrimSpace(string(out))

	os.Rename(filepath.Join(repoDir, "oldname.txt"), filepath.Join(repoDir, "newname.txt"))
	runGit(t, repoDir, "add", "-A")
	runGit(t, repoDir, "commit", "-m", "rename")

	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ = cmd.Output()
	secondCommit := strings.TrimSpace(string(out))

	t.Run("detects renamed files with old and new names", func(t *testing.T) {
		files, err := client.GetChangedFiles(firstCommit, secondCommit)
		if err != nil {
			t.Fatalf("GetChangedFiles() failed: %v", err)
		}

		foundRename := false
		for _, f := range files {
			if f.Status == StatusRenamed && f.Path == "newname.txt" && f.OldPath == "oldname.txt" {
				foundRename = true
				break
			}
		}

		if !foundRename {
			t.Errorf("Expected rename from oldname.txt to newname.txt, got: %+v", files)
		}
	})
}

func TestClient_GetChangedFiles_WithDeleted(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	os.WriteFile(filepath.Join(repoDir, "todelete.txt"), []byte("content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "first")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ := cmd.Output()
	firstCommit := strings.TrimSpace(string(out))

	os.Remove(filepath.Join(repoDir, "todelete.txt"))
	runGit(t, repoDir, "add", "-A")
	runGit(t, repoDir, "commit", "-m", "delete")

	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ = cmd.Output()
	secondCommit := strings.TrimSpace(string(out))

	t.Run("detects deleted files", func(t *testing.T) {
		files, err := client.GetChangedFiles(firstCommit, secondCommit)
		if err != nil {
			t.Fatalf("GetChangedFiles() failed: %v", err)
		}

		foundDelete := false
		for _, f := range files {
			if f.Status == StatusDeleted && f.Path == "todelete.txt" {
				foundDelete = true
				break
			}
		}

		if !foundDelete {
			t.Errorf("Expected deleted file, got: %+v", files)
		}
	})
}

func TestClient_GetFileContent(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	content := []byte("hello world")
	os.WriteFile(filepath.Join(repoDir, "test.txt"), content, 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ := cmd.Output()
	commit := strings.TrimSpace(string(out))

	t.Run("retrieves file content at commit", func(t *testing.T) {
		got, err := client.GetFileContent(commit, "test.txt")
		if err != nil {
			t.Fatalf("GetFileContent() failed: %v", err)
		}

		if string(got) != string(content) {
			t.Errorf("Content mismatch: got %q, want %q", got, content)
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		_, err := client.GetFileContent(commit, "nonexistent.txt")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})
}

func TestClient_IsFileOutsideRepo(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	tests := []struct {
		name     string
		path     string
		expected bool
		isUnix   bool
	}{
		{name: "file inside repo", path: "src/main.go", expected: false},
		{name: "file at root", path: "readme.md", expected: false},
		{name: "file outside repo (parent dir)", path: "../config.txt", expected: true},
		{name: "absolute path outside", path: "/etc/passwd", expected: true, isUnix: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isUnix && runtime.GOOS == "windows" {
				return
			}
			got := client.IsFileOutsideRepo(tt.path)
			if got != tt.expected {
				t.Errorf("IsFileOutsideRepo(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestClient_GetRecentCommits(t *testing.T) {
	repoDir := setupTestRepo(t)

	commitTime := time.Date(2012, 12, 21, 14, 15, 25, 0, time.Local)

	// Create 3 commits
	for i := 1; i <= 3; i++ {
		filename := fmt.Sprintf("file%d.txt", i)
		os.WriteFile(filepath.Join(repoDir, filename), []byte("content"), 0o644)
		runGit(t, repoDir, "add", filename)
		runGit(t, repoDir, "commit", "--date", commitTime.Format(time.RFC3339), "-m", fmt.Sprintf("commit %d", i))
	}

	c := NewClient(repoDir)
	commits, err := c.GetRecentCommits(10)
	if err != nil {
		t.Fatalf("GetRecentCommits failed: %v", err)
	}

	if len(commits) != 3 {
		t.Errorf("Expected 3 commits, got %d", len(commits))
	}

	if commits[0].Message != "commit 3" {
		t.Errorf("Expected first commit message to be 'commit 3', got %s", commits[0].Message)
	}

	if !commits[0].Time.Equal(commitTime.Truncate(time.Millisecond)) {
		t.Errorf("Expected first commit date to be %s, got %s", commitTime, commits[0].Time)
	}
}
