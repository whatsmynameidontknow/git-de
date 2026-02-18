package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClient_GetDefaultBranch(t *testing.T) {
	t.Run("detects main as default branch", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		client := NewClient(repoDir)

		// Create initial commit on main
		os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
		runGit(t, repoDir, "add", ".")
		runGit(t, repoDir, "commit", "-m", "initial")

		// Rename default branch to main (in case it's master)
		runGit(t, repoDir, "branch", "-M", "main")

		branch, err := client.GetDefaultBranch()
		if err != nil {
			t.Fatalf("GetDefaultBranch() failed: %v", err)
		}
		if branch != "main" {
			t.Errorf("Expected 'main', got %q", branch)
		}
	})

	t.Run("detects master as fallback", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		client := NewClient(repoDir)

		os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
		runGit(t, repoDir, "add", ".")
		runGit(t, repoDir, "commit", "-m", "initial")

		// Rename default branch to master
		runGit(t, repoDir, "branch", "-M", "master")

		branch, err := client.GetDefaultBranch()
		if err != nil {
			t.Fatalf("GetDefaultBranch() failed: %v", err)
		}
		if branch != "master" {
			t.Errorf("Expected 'master', got %q", branch)
		}
	})

	t.Run("returns error when no default branch found", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		client := NewClient(repoDir)

		os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
		runGit(t, repoDir, "add", ".")
		runGit(t, repoDir, "commit", "-m", "initial")

		// Rename to something non-standard
		runGit(t, repoDir, "branch", "-M", "develop")

		_, err := client.GetDefaultBranch()
		if err == nil {
			t.Error("Expected error when no main/master branch exists")
		}
	})
}

func TestClient_BranchExists(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")
	runGit(t, repoDir, "branch", "-M", "main")

	t.Run("returns true for existing branch", func(t *testing.T) {
		if !client.BranchExists("main") {
			t.Error("Expected BranchExists('main') to return true")
		}
	})

	t.Run("returns false for non-existing branch", func(t *testing.T) {
		if client.BranchExists("nonexistent") {
			t.Error("Expected BranchExists('nonexistent') to return false")
		}
	})
}

func TestClient_GetBranches(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	// Create initial commit
	os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial commit")
	runGit(t, repoDir, "branch", "-M", "main")

	// Create feature branch
	runGit(t, repoDir, "checkout", "-b", "feature/auth")
	os.WriteFile(filepath.Join(repoDir, "auth.go"), []byte("package auth"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "add auth")

	// Create another branch
	runGit(t, repoDir, "checkout", "main")
	runGit(t, repoDir, "checkout", "-b", "bugfix/login")
	os.WriteFile(filepath.Join(repoDir, "login.go"), []byte("package login"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "fix login")

	// Go back to main
	runGit(t, repoDir, "checkout", "main")

	t.Run("returns all local branches", func(t *testing.T) {
		branches, err := client.GetBranches()
		if err != nil {
			t.Fatalf("GetBranches() failed: %v", err)
		}

		if len(branches) < 3 {
			t.Errorf("Expected at least 3 branches, got %d", len(branches))
		}

		names := make(map[string]bool)
		for _, b := range branches {
			names[b.Name] = true
		}

		for _, expected := range []string{"main", "feature/auth", "bugfix/login"} {
			if !names[expected] {
				t.Errorf("Expected branch %q in results, got: %v", expected, names)
			}
		}
	})

	t.Run("marks current branch", func(t *testing.T) {
		branches, err := client.GetBranches()
		if err != nil {
			t.Fatalf("GetBranches() failed: %v", err)
		}

		foundCurrent := false
		for _, b := range branches {
			if b.IsCurrent {
				foundCurrent = true
				if b.Name != "main" {
					t.Errorf("Expected current branch to be 'main', got %q", b.Name)
				}
			}
		}

		if !foundCurrent {
			t.Error("Expected one branch to be marked as current")
		}
	})

	t.Run("includes last commit message", func(t *testing.T) {
		branches, err := client.GetBranches()
		if err != nil {
			t.Fatalf("GetBranches() failed: %v", err)
		}

		for _, b := range branches {
			if b.Name == "feature/auth" {
				if b.LastMessage != "add auth" {
					t.Errorf("Expected LastMessage 'add auth', got %q", b.LastMessage)
				}
				return
			}
		}
		t.Error("feature/auth branch not found")
	})

	t.Run("includes last commit time", func(t *testing.T) {
		branches, err := client.GetBranches()
		if err != nil {
			t.Fatalf("GetBranches() failed: %v", err)
		}

		for _, b := range branches {
			if b.Name == "feature/auth" {
				if b.LastCommit.IsZero() {
					t.Error("Expected non-zero LastCommit time")
				}
				// Should be recent (within last minute)
				if time.Since(b.LastCommit) > time.Minute {
					t.Errorf("Expected recent commit time, got %v ago", time.Since(b.LastCommit))
				}
				return
			}
		}
		t.Error("feature/auth branch not found")
	})

	t.Run("sorts current branch first", func(t *testing.T) {
		branches, err := client.GetBranches()
		if err != nil {
			t.Fatalf("GetBranches() failed: %v", err)
		}

		if len(branches) == 0 {
			t.Fatal("Expected at least one branch")
		}

		if !branches[0].IsCurrent {
			t.Errorf("Expected first branch to be current, got %q (IsCurrent=%v)", branches[0].Name, branches[0].IsCurrent)
		}
	})
}

func TestClient_GetBranchAheadBehind(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	// Create initial commit on main
	os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")
	runGit(t, repoDir, "branch", "-M", "main")

	// Create feature branch with 2 commits ahead
	runGit(t, repoDir, "checkout", "-b", "feature/test")
	for i := 1; i <= 2; i++ {
		os.WriteFile(filepath.Join(repoDir, fmt.Sprintf("feat%d.go", i)), []byte("package feat"), 0o644)
		runGit(t, repoDir, "add", ".")
		runGit(t, repoDir, "commit", "-m", fmt.Sprintf("feat %d", i))
	}

	t.Run("returns correct ahead count", func(t *testing.T) {
		ahead, behind, err := client.GetBranchAheadBehind("feature/test", "main")
		if err != nil {
			t.Fatalf("GetBranchAheadBehind() failed: %v", err)
		}
		if ahead != 2 {
			t.Errorf("Expected ahead=2, got %d", ahead)
		}
		if behind != 0 {
			t.Errorf("Expected behind=0, got %d", behind)
		}
	})

	t.Run("returns correct behind count", func(t *testing.T) {
		// Add commit on main
		runGit(t, repoDir, "checkout", "main")
		os.WriteFile(filepath.Join(repoDir, "main_update.go"), []byte("package main"), 0o644)
		runGit(t, repoDir, "add", ".")
		runGit(t, repoDir, "commit", "-m", "main update")

		ahead, behind, err := client.GetBranchAheadBehind("feature/test", "main")
		if err != nil {
			t.Fatalf("GetBranchAheadBehind() failed: %v", err)
		}
		if ahead != 2 {
			t.Errorf("Expected ahead=2, got %d", ahead)
		}
		if behind != 1 {
			t.Errorf("Expected behind=1, got %d", behind)
		}
	})

	t.Run("returns error for empty default branch", func(t *testing.T) {
		_, _, err := client.GetBranchAheadBehind("feature/test", "")
		if err == nil {
			t.Error("Expected error for empty default branch")
		}
	})
}

func TestClient_IsBranchMerged(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	// Create initial commit on main
	os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")
	runGit(t, repoDir, "branch", "-M", "main")

	// Create and merge a branch
	runGit(t, repoDir, "checkout", "-b", "feature/merged")
	os.WriteFile(filepath.Join(repoDir, "merged.go"), []byte("package merged"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "merged feature")
	runGit(t, repoDir, "checkout", "main")
	runGit(t, repoDir, "merge", "feature/merged")

	// Create an unmerged branch
	runGit(t, repoDir, "checkout", "-b", "feature/unmerged")
	os.WriteFile(filepath.Join(repoDir, "unmerged.go"), []byte("package unmerged"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "unmerged feature")
	runGit(t, repoDir, "checkout", "main")

	t.Run("returns true for merged branch", func(t *testing.T) {
		if !client.IsBranchMerged("feature/merged", "main") {
			t.Error("Expected feature/merged to be merged")
		}
	})

	t.Run("returns false for unmerged branch", func(t *testing.T) {
		if client.IsBranchMerged("feature/unmerged", "main") {
			t.Error("Expected feature/unmerged to not be merged")
		}
	})

	t.Run("returns false for empty default branch", func(t *testing.T) {
		if client.IsBranchMerged("feature/merged", "") {
			t.Error("Expected false for empty default branch")
		}
	})
}

func TestClient_GetCurrentBranch(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")
	runGit(t, repoDir, "branch", "-M", "main")

	t.Run("returns current branch name", func(t *testing.T) {
		branch, err := client.GetCurrentBranch()
		if err != nil {
			t.Fatalf("GetCurrentBranch() failed: %v", err)
		}
		if branch != "main" {
			t.Errorf("Expected 'main', got %q", branch)
		}
	})

	t.Run("returns correct branch after checkout", func(t *testing.T) {
		runGit(t, repoDir, "checkout", "-b", "feature/test")
		branch, err := client.GetCurrentBranch()
		if err != nil {
			t.Fatalf("GetCurrentBranch() failed: %v", err)
		}
		if branch != "feature/test" {
			t.Errorf("Expected 'feature/test', got %q", branch)
		}
	})
}

func TestClient_GetBranchesFiltered(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	// Create initial commit on main
	os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial commit")
	runGit(t, repoDir, "branch", "-M", "main")

	// Create and merge a branch (simulating remote merged)
	runGit(t, repoDir, "checkout", "-b", "feature/merged")
	os.WriteFile(filepath.Join(repoDir, "merged.go"), []byte("package merged"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "add merged feature")
	runGit(t, repoDir, "checkout", "main")
	runGit(t, repoDir, "merge", "feature/merged")

	// Create an unmerged branch
	runGit(t, repoDir, "checkout", "-b", "feature/unmerged")
	os.WriteFile(filepath.Join(repoDir, "unmerged.go"), []byte("package unmerged"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "add unmerged feature")
	runGit(t, repoDir, "checkout", "main")

	t.Run("GetBranchesFiltered excludes merged remote branches", func(t *testing.T) {
		// For local branches, merged filtering shouldn't apply
		// (only remote merged should be hidden)
		branches, err := client.GetBranchesFiltered(true)
		if err != nil {
			t.Fatalf("GetBranchesFiltered() failed: %v", err)
		}

		// All local branches should be present
		names := make(map[string]bool)
		for _, b := range branches {
			names[b.Name] = true
		}

		if !names["main"] {
			t.Error("Expected 'main' in branches")
		}
		if !names["feature/unmerged"] {
			t.Error("Expected 'feature/unmerged' in branches")
		}
	})
}

func TestClient_GetBranchesWithAheadBehind(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	// Create initial commit on main
	os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")
	runGit(t, repoDir, "branch", "-M", "main")

	// Create feature branch with 2 commits
	runGit(t, repoDir, "checkout", "-b", "feature/test")
	os.WriteFile(filepath.Join(repoDir, "feat1.go"), []byte("package feat"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "feat 1")
	os.WriteFile(filepath.Join(repoDir, "feat2.go"), []byte("package feat"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "feat 2")
	runGit(t, repoDir, "checkout", "main")

	t.Run("populates ahead/behind when requested", func(t *testing.T) {
		branches, err := client.GetBranchesWithAheadBehind()
		if err != nil {
			t.Fatalf("GetBranchesWithAheadBehind() failed: %v", err)
		}

		for _, b := range branches {
			if b.Name == "feature/test" {
				if b.Ahead != 2 {
					t.Errorf("Expected ahead=2 for feature/test, got %d", b.Ahead)
				}
				if b.Behind != 0 {
					t.Errorf("Expected behind=0 for feature/test, got %d", b.Behind)
				}
				return
			}
		}
		t.Error("feature/test not found in branches")
	})
}

func TestClient_GetRecentCommitsOnBranch(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)
	commitTime := time.Date(2012, 12, 21, 14, 15, 25, 0, time.Local)

	// Create initial commit on main
	os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")
	runGit(t, repoDir, "branch", "-M", "main")

	// Create feature branch with some commits
	runGit(t, repoDir, "checkout", "-b", "feature/test")
	for i := 1; i <= 3; i++ {
		os.WriteFile(filepath.Join(repoDir, fmt.Sprintf("feat%d.go", i)), []byte("package feat"), 0o644)
		runGit(t, repoDir, "add", ".")
		runGit(t, repoDir, "commit", "-m", fmt.Sprintf("feat commit %d", i), "--date", commitTime.Format(time.RFC3339))
	}

	t.Run("returns commits from specific branch", func(t *testing.T) {
		commits, err := client.GetRecentCommitsOnBranch("feature/test", 10)
		if err != nil {
			t.Fatalf("GetRecentCommitsOnBranch() failed: %v", err)
		}

		if len(commits) != 4 { // 3 feature + 1 initial
			t.Errorf("Expected 4 commits, got %d", len(commits))
		}

		if commits[0].Message != "feat commit 3" {
			t.Errorf("Expected first commit to be 'feat commit 3', got %q", commits[0].Message)
		}

		if commits[0].Time != commitTime.Truncate(time.Millisecond) {
			t.Errorf("Expected first commit date to be %s, got %s", commitTime, commits[0].Time)
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		commits, err := client.GetRecentCommitsOnBranch("feature/test", 2)
		if err != nil {
			t.Fatalf("GetRecentCommitsOnBranch() failed: %v", err)
		}

		if len(commits) != 2 {
			t.Errorf("Expected 2 commits, got %d", len(commits))
		}
	})

	t.Run("filters out merge commits", func(t *testing.T) {
		// Go back to main and create a merge commit
		runGit(t, repoDir, "checkout", "main")
		runGit(t, repoDir, "merge", "--no-ff", "feature/test", "-m", "Merge feature/test")

		commits, err := client.GetRecentCommitsOnBranch("main", 10)
		if err != nil {
			t.Fatalf("GetRecentCommitsOnBranch() failed: %v", err)
		}

		for _, c := range commits {
			if strings.HasPrefix(c.Message, "Merge ") {
				t.Errorf("Expected no merge commits, found: %q", c.Message)
			}
		}
	})
}

func TestClient_CheckoutBranch(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	// Create initial commit on main
	os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")
	runGit(t, repoDir, "branch", "-M", "main")

	// Create feature branch
	runGit(t, repoDir, "checkout", "-b", "feature/test")
	runGit(t, repoDir, "checkout", "main")

	t.Run("checks out existing branch", func(t *testing.T) {
		err := client.CheckoutBranch("feature/test")
		if err != nil {
			t.Fatalf("CheckoutBranch() failed: %v", err)
		}

		// Verify we're on the right branch
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		cmd.Dir = repoDir
		out, _ := cmd.Output()
		if strings.TrimSpace(string(out)) != "feature/test" {
			t.Errorf("Expected to be on feature/test, got %q", strings.TrimSpace(string(out)))
		}
	})

	t.Run("returns error for non-existent branch", func(t *testing.T) {
		err := client.CheckoutBranch("nonexistent")
		if err == nil {
			t.Error("Expected error for non-existent branch")
		}
	})
}

func TestClient_GetCommitRangeStats(t *testing.T) {
	repoDir := setupTestRepo(t)
	client := NewClient(repoDir)

	// Create initial commit on main
	os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ := cmd.Output()
	firstCommit := strings.TrimSpace(string(out))

	// Add more commits
	os.WriteFile(filepath.Join(repoDir, "file2.txt"), []byte("new file content"), 0o644)
	os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("modified content"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "second")

	os.WriteFile(filepath.Join(repoDir, "file3.txt"), []byte("another file"), 0o644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "third")

	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ = cmd.Output()
	lastCommit := strings.TrimSpace(string(out))

	t.Run("returns correct commit count", func(t *testing.T) {
		stats, err := client.GetCommitRangeStats(firstCommit, lastCommit)
		if err != nil {
			t.Fatalf("GetCommitRangeStats() failed: %v", err)
		}
		if stats.CommitCount != 2 {
			t.Errorf("Expected 2 commits, got %d", stats.CommitCount)
		}
	})

	t.Run("returns correct files changed count", func(t *testing.T) {
		stats, err := client.GetCommitRangeStats(firstCommit, lastCommit)
		if err != nil {
			t.Fatalf("GetCommitRangeStats() failed: %v", err)
		}
		if stats.FilesChanged != 3 { // file.txt modified, file2.txt added, file3.txt added
			t.Errorf("Expected 3 files changed, got %d", stats.FilesChanged)
		}
	})

	t.Run("returns non-negative additions and deletions", func(t *testing.T) {
		stats, err := client.GetCommitRangeStats(firstCommit, lastCommit)
		if err != nil {
			t.Fatalf("GetCommitRangeStats() failed: %v", err)
		}
		if stats.Additions < 0 {
			t.Errorf("Expected non-negative additions, got %d", stats.Additions)
		}
		if stats.Deletions < 0 {
			t.Errorf("Expected non-negative deletions, got %d", stats.Deletions)
		}
	})
}
