# git-de

Git Diff Export - A CLI tool to export files changed between Git commits while preserving directory structure.

## Installation

```bash
go install github.com/whatsmynameidontknow/git-de/cmd/git-de@latest
```

Or download from [releases](https://github.com/whatsmynameidontknow/git-de/releases).

## Usage

```bash
git-de [options] <from-commit> [<to-commit>]
```

### Arguments

- `from-commit` - Starting commit (branch, tag, or SHA) **(required unless using --tui)**
- `to-commit` - Ending commit (defaults to `HEAD`)

### Options

- `-f, --from` - Starting commit (alternative to positional arg)
- `-t, --to` - Ending commit (defaults to HEAD)
- `-o, --output` - Output directory (required for CLI mode, skips TUI)
- `-w, --overwrite` - Overwrite existing output directory
- `-c, --concurrent` - Copy files concurrently
- `-v, --verbose` - Enable verbose output
- `-i, --ignore` - Ignore patterns (comma-separated or multiple flags)
- `-I, --include` - Include patterns - only export files matching these
- `--max-size` - Maximum file size to export (e.g., 10MB, 500KB)
- `-a, --archive` - Export directly to archive (.zip, .tar, .tar.gz)
- `--no-tui` - Force CLI mode even in interactive terminal
- `-h, --help` - Show help

### Examples

```bash
# Launch TUI (default in terminal)
git-de

# Launch TUI with initial commit range
git-de HEAD~5

# CLI mode - export directly
git-de HEAD~5 HEAD -o ./export

# Export .go files only to a zip archive
git-de HEAD~5 HEAD -I "*.go" -a export.zip

# Concurrent export with ignore patterns
git-de main develop -o ./export -c -i "*.log,node_modules/"

# Force CLI mode in terminal
git-de --no-tui HEAD~5 HEAD -o ./export
```

## Features

- ✅ **Interactive TUI** - Select commits and files visually
- ✅ **Archive Export** - Direct to ZIP or Tar.gz
- ✅ **Size Limits** - Prevent exporting accidental large blobs
- ✅ **Include/Ignore Patterns** - Powerful whitelist/blacklist filtering
- ✅ **Preview mode** - See changes without copying files
- ✅ **Concurrent copying** - High performance for large diffs
- ✅ **Cross-platform** - Works on Linux and Windows

## Requirements

- Go 1.24+ (for building from source)
- Git installed and in PATH

## License

MIT
