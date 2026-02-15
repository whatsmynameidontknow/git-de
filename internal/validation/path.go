package validation

import (
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// ValidatePath checks if a path is valid for the current OS.
// Returns an error if the path contains invalid characters or is reserved.
func ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Check for null bytes (invalid on all platforms)
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("path cannot contain null bytes")
	}

	// Clean the path
	path = filepath.Clean(path)

	if runtime.GOOS == "windows" {
		return validateWindowsPath(path)
	}

	return nil
}

// Windows invalid characters: < > : " | ? *
const windowsInvalidChars = `<>:"|?*`

func validateWindowsPath(path string) error {
	normalized := strings.ReplaceAll(path, "/", "\\")
	for idx, char := range normalized {
		if !strings.ContainsRune(windowsInvalidChars, char) {
			continue
		}

		if char == ':' && idx == 1 && isASCIIAlpha(normalized[0]) {
			continue
		}

		return fmt.Errorf("path contains invalid character: %q", char)
	}

	volume := filepath.VolumeName(path)
	remainder := strings.TrimPrefix(path, volume)
	remainder = strings.ReplaceAll(remainder, "/", "\\")

	for segment := range strings.SplitSeq(remainder, "\\") {
		if segment == "" {
			continue
		}

		if strings.HasSuffix(segment, " ") || strings.HasSuffix(segment, ".") {
			return fmt.Errorf("path segment cannot end with space or period on Windows")
		}

		for _, char := range windowsInvalidChars {
			if strings.ContainsRune(segment, char) {
				return fmt.Errorf("path contains invalid character: %q", char)
			}
		}

		base := segment
		if idx := strings.Index(base, "."); idx != -1 {
			base = base[:idx]
		}
		base = strings.ToUpper(base)

		reservedNames := []string{"CON", "PRN", "AUX", "NUL"}
		if slices.Contains(reservedNames, base) {
			return fmt.Errorf("%q is a reserved name on Windows", base)
		}

		for i := 1; i <= 9; i++ {
			if base == fmt.Sprintf("COM%d", i) || base == fmt.Sprintf("LPT%d", i) {
				return fmt.Errorf("%q is a reserved name on Windows", base)
			}
		}
	}

	return nil
}

func isASCIIAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}
