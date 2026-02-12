package manifest

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/whatsmynameidontknow/git-de/internal/git"
)

func Generate(changes []git.FileChange) string {
	var newFiles, modified, renamed, deleted []string

	for _, change := range changes {
		switch change.Status {
		case "A":
			newFiles = append(newFiles, change.Path)
		case "M":
			modified = append(modified, change.Path)
		case "R":
			renamed = append(renamed, fmt.Sprintf("%s (previously %s)", change.Path, change.OldPath))
		case "D":
			deleted = append(deleted, change.Path)
		}
	}

	sort.Strings(newFiles)
	sort.Strings(modified)
	sort.Strings(renamed)
	sort.Strings(deleted)

	var sb strings.Builder

	if len(newFiles) > 0 {
		sb.WriteString("new files:\n")
		for _, f := range newFiles {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	if len(modified) > 0 {
		sb.WriteString("modified:\n")
		for _, f := range modified {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	if len(renamed) > 0 {
		sb.WriteString("renamed:\n")
		for _, f := range renamed {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	if len(deleted) > 0 {
		sb.WriteString("deleted:\n")
		for _, f := range deleted {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

func WriteToFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
