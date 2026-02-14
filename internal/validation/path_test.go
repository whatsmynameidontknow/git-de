package validation

import (
	"runtime"
	"testing"
)

func TestValidatePath_Empty(t *testing.T) {
	err := ValidatePath("")
	if err == nil {
		t.Error("Expected error for empty path")
	}
}

func TestValidatePath_NullByte(t *testing.T) {
	err := ValidatePath("test\x00path")
	if err == nil {
		t.Error("Expected error for path with null byte")
	}
}

func TestValidatePath_UnixValid(t *testing.T) {
	paths := []string{
		"./export",
		"/home/user/export",
		"export",
		"my-export-dir",
		".hidden",
		"file with spaces",
	}

	for _, path := range paths {
		err := ValidatePath(path)
		if err != nil {
			t.Errorf("Expected no error for %q, got: %v", path, err)
		}
	}
}

func TestValidatePath_WindowsInvalidChars(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	invalidPaths := []string{
		"test<path",
		"test>path",
		"test:path",
		`test"path`,
		"test|path",
		"test?path",
		"test*path",
		`\\inval*id\a:\b`,
	}

	for _, path := range invalidPaths {
		err := ValidatePath(path)
		if err == nil {
			t.Errorf("Expected error for %q on Windows", path)
		}
	}
}

func TestValidatePath_WindowsReservedNames(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	reservedPaths := []string{
		"CON",
		"PRN",
		"AUX",
		"NUL",
		"COM1",
		"COM9",
		"LPT1",
		"LPT9",
		"CON.txt",
		"com1.exe",
	}

	for _, path := range reservedPaths {
		err := ValidatePath(path)
		if err == nil {
			t.Errorf("Expected error for reserved name %q on Windows", path)
		}
	}
}

func TestValidatePath_WindowsTrailingSpace(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	err := ValidatePath("export ")
	if err == nil {
		t.Error("Expected error for path ending with space on Windows")
	}
}

func TestValidatePath_WindowsTrailingPeriod(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	err := ValidatePath("export.")
	if err == nil {
		t.Error("Expected error for path ending with period on Windows")
	}
}

func TestValidatePath_WindowsValid(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	paths := []string{
		`C:\Users\test\export`,
		`\\server\share\export`,
		`export`,
		`my-export-dir`,
		`file with spaces`,
	}

	for _, path := range paths {
		err := ValidatePath(path)
		if err != nil {
			t.Errorf("Expected no error for %q, got: %v", path, err)
		}
	}
}
