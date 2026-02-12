# git-de

Git Diff Export - A CLI tool to export files changed between Git commits.

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

- `from-commit` - Starting commit (branch, tag, or SHA) **(required)**
- `to-commit` - Ending commit (defaults to `HEAD`)

### Options

- `-f, --from` - Starting commit (alternative to positional arg)
- `-t, --to` - Ending commit (defaults to HEAD)
- `-o, --output` - Output directory (optional, runs in preview mode if not set)
- `-w, --overwrite` - Overwrite existing output directory
- `-c, --concurrent` - Copy files concurrently
- `-v, --verbose` - Enable verbose output
- `-i, --ignore` - Ignore patterns (comma-separated or multiple flags)
- `-I, --include` - Include patterns - only export files matching these (comma-separated or multiple flags)
- `-h, --help` - Show help

### Examples

```bash
# Preview changes (no files copied)
git-de HEAD~5 HEAD

# Export changes to directory
git-de HEAD~5 HEAD -o ./export

# Export using flags
git-de --from v1.0.0 --to v2.0.0 --output ./export

# Export with concurrent copying
git-de main develop -o ./export -c

# Overwrite existing output directory
git-de HEAD~10 HEAD -o ./export --overwrite

# Only export .go files (ignore wins in conflicts)
git-de HEAD~5 HEAD -I "*.go" -i "*_test.go" -o ./export

# Include only specific directories
git-de HEAD~5 HEAD -I "cmd/*,pkg/*" -o ./export

# Verbose mode with ignore patterns
git-de HEAD~5 HEAD -o ./export -v -i "*.log,node_modules/"
```

### Preview Mode

When you don't specify an output directory (`-o`), `git-de` runs in **preview mode**:

```
=== PREVIEW MODE (no files will be copied) ===

Files that would be exported (3):
  → A: new.go
  → M: main.go
  → R: pkg/utils.go

=== Summary ===
new files:
- new.go
modified:
- main.go
renamed:
- pkg/utils.go (previously pkg/helpers.go)
```

## Output

When an output directory is specified:

```
export/
├── summary.txt
├── src/
│   └── main.go
└── README.md
```

### summary.txt Format

```
new files:
- cmd/new.go

modified:
- src/main.go

renamed:
- pkg/utils.go (previously pkg/helpers.go)

deleted:
- old/file.txt
```

## Features

- ✅ **Preview mode** - See changes without copying files
- ✅ Copy new, modified, renamed, and copied files
- ✅ Preserve directory structure
- ✅ Concurrent file copying (`-c` flag)
- ✅ Plain text summary report
- ✅ Overwrite protection (`-w` flag)
- ✅ Verbose mode (`-v` flag)
- ✅ Ignore patterns (`-i` flag, comma-separated)
- ✅ Include patterns (`-I` flag, whitelist filtering)
- ✅ Ignores `.git` directory automatically
- ✅ Warns about files outside repo root

## Requirements

- Go 1.24+ (for building from source)
- Git installed and in PATH

## License

MIT
