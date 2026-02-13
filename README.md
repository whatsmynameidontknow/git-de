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

| Flag | Description | TUI Mode | CLI Mode |
|------|-------------|----------|----------|
| `-f, --from` | Starting commit (alternative to positional arg) | ✅ Used | ✅ Used |
| `-t, --to` | Ending commit (defaults to HEAD) | ✅ Used | ✅ Used |
| `-o, --output` | Output directory | ❌ Ignored (TUI asks interactively) | ✅ Required* |
| `-w, --overwrite` | Overwrite existing output directory | ❌ Ignored | ✅ Used |
| `-c, --concurrent` | Copy files concurrently | ❌ Ignored | ✅ Used |
| `-v, --verbose` | Enable verbose output | ❌ Ignored | ✅ Used |
| `-i, --ignore` | Ignore patterns (comma-separated or multiple flags) | ❌ Ignored | ✅ Used |
| `-I, --include` | Include patterns - only export files matching these | ❌ Ignored | ✅ Used |
| `--max-size` | Maximum file size to export (e.g., 10MB, 500KB) | ❌ Ignored | ✅ Used |
| `-a, --archive` | Export directly to archive (.zip, .tar, .tar.gz) | ❌ Ignored (skips TUI) | ✅ Used* |
| `--no-tui` | Force CLI mode even in interactive terminal | — | — |
| `-h, --help` | Show help | — | — |

**Legend:** ✅ = Used, ❌ = Ignored

**Note:** `-o` and `-a` are mutually exclusive — use one or the other. Both skip the TUI and run in CLI mode.

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
