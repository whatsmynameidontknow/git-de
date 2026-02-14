package validation

import (
	"errors"
	"fmt"
	"io/fs"
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

	if !fs.ValidPath(path) {
		return errors.New("invalid path")
	}

	if runtime.GOOS == "windows" {
		return validateWindowsPath(path)
	}

	return nil
}

// Windows invalid characters: < > : " | ? *
const windowsInvalidChars = `<>:"|?*`

func validateWindowsPath(path string) error {
	volume := filepath.VolumeName(path)
	if volume != "" && filepath.Dir(volume) == volume+"." {
		fmt.Println(volume)
		goto reserved_names_check
	}
	for _, char := range windowsInvalidChars {
		if strings.ContainsRune(path, char) {
			return fmt.Errorf("path contains invalid character: %q", char)
		}
	}

reserved_names_check:
	// Check for reserved names (CON, PRN, AUX, NUL, COM1-9, LPT1-9)
	// Get the base name without extension
	base := filepath.Base(path)
	if idx := strings.LastIndex(base, "."); idx != -1 {
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

	// Check for trailing spaces or periods (invalid in Windows filenames)
	if strings.HasSuffix(path, " ") || strings.HasSuffix(path, ".") {
		return fmt.Errorf("path cannot end with space or period on Windows")
	}

	return nil
}
