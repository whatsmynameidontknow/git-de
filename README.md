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
- `-o, --output` - Output directory **(required)**
- `-w, --overwrite` - Overwrite existing output directory
- `-c, --concurrent` - Copy files concurrently
- `-h, --help` - Show help

### Examples

```bash
# Export changes between HEAD~5 and HEAD
git-de HEAD~5 HEAD -o ./export

# Export using flags
git-de --from v1.0.0 --to v2.0.0 --output ./export

# Export with concurrent copying
git-de main develop -o ./export -c

# Overwrite existing output directory
git-de HEAD~10 HEAD -o ./export --overwrite
```

## Output

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

- ✅ Copy new, modified, renamed, and copied files
- ✅ Preserve directory structure
- ✅ Concurrent file copying (`-c` flag)
- ✅ Plain text summary report
- ✅ Overwrite protection (`-w` flag)
- ✅ Ignores `.git` directory automatically
- ✅ Warns about files outside repo root

## Requirements

- Go 1.24+ (for building from source)
- Git installed and in PATH

## License

MIT
