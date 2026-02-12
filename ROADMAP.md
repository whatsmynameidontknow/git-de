# git-de Roadmap

## Version History

- **v0.1.0** - Initial release with basic export functionality
- **v0.2.0** - Preview mode (run without `-o` flag)
- **v0.3.0** - Verbose mode, progress bar, ignore patterns
- **v0.4.0** - Include patterns, file size limits, archive export, TUI (planned)

---

## v0.4.0 Roadmap

Staggered release with 4 features in priority order.

### Feature 1: Include Patterns

**Priority:** 1 (highest)  
**Release:** v0.4.0-alpha1

#### Specification

Add `--include` (`-I`) flag for whitelist filtering of files.

**Behavior:**
- Works similarly to `--ignore` but opposite logic
- Only files matching include patterns are considered
- If both `--include` and `--ignore` specified, apply include FIRST, then ignore
- **Conflict resolution:** Ignore wins (safer)
- Supports comma-separated patterns and multiple flags

**CLI Changes:**
```bash
# Only export .go files
git-de HEAD~5 -I "*.go"

# Include multiple patterns
git-de HEAD~5 -I "*.go,*.md" -I "Makefile"

# Include + ignore combined (ignore wins conflicts)
git-de HEAD~5 -I "*.go" -i "*_test.go"
```

**Implementation Notes:**
- Add `IncludePatterns []string` to `cli.Config` and `exporter.Options`
- In `filterAndProcess()`: check include patterns first (if any)
- Add test cases for:
  - Basic include pattern
  - Multiple include patterns
  - Comma-separated includes
  - Include + ignore conflict (ignore wins)
  - No include patterns (export all, current behavior)

**Precedence Order:**
```
1. Must match --include (if any specified)
2. Must NOT match --ignore
3. Must NOT be deleted
4. Must NOT be outside repo
5. Must NOT exceed size limit (if specified)
```

---

### Feature 2: File Size Limit

**Priority:** 3  
**Release:** v0.4.0-alpha2

#### Specification

Add `--max-size` flag to skip files exceeding size limit.

**Behavior:**
- Skip files larger than specified size
- Show warning for skipped files
- Support human-readable formats (10MB, 1GB, 500KB)
- Applied after include/ignore filtering

**CLI Changes:**
```bash
# Skip files > 10MB
git-de HEAD~5 -o ./export --max-size 10MB

# Combine with other flags
git-de HEAD~5 -I "*.go" --max-size 5MB -v
```

**Implementation Notes:**
- Add `MaxSize int64` to config (bytes internally)
- Parse human-readable sizes (KB, MB, GB, TB)
- Check file size before reading content (avoid loading large files)
- Warning format: `⚠ Skipped (too large): filename (25MB > 10MB)`
- Add test cases for:
  - Size limit parsing
  - Files under limit (exported)
  - Files over limit (skipped with warning)
  - Edge cases (exactly at limit, zero limit)

**Size Parsing:**
- `10` → 10 bytes
- `10B` → 10 bytes
- `10K` or `10KB` → 10,240 bytes
- `10M` or `10MB` → 10,485,760 bytes
- `1G` or `1GB` → 1,073,741,824 bytes

---

### Feature 3: Archive Export

**Priority:** 2  
**Release:** v0.4.0-alpha3

#### Specification

Add `--archive` (`-a`) flag to export directly to archive file (zip/tar/tar.gz).

**Behavior:**
- Export files directly to archive without creating temp folder
- Auto-detect format from extension (.zip, .tar, .tar.gz, .tgz)
- Preserve directory structure inside archive
- Summary.txt included in archive

**CLI Changes:**
```bash
# Export to zip
git-de HEAD~5 -a export.zip

# Export to tar.gz
git-de HEAD~5 -a export.tar.gz

# Combine with other flags
git-de HEAD~5 -I "*.go" -a code.zip --max-size 5MB
```

**Implementation Notes:**
- Add `ArchivePath string` to config
- Mutually exclusive with `--output` (error if both provided)
- Support formats:
  - `.zip` → ZIP archive
  - `.tar` → Uncompressed tar
  - `.tar.gz`, `.tgz` → Gzipped tar
- Stream files directly into archive (no temp files)
- Progress bar works same as folder export
- Add test cases for:
  - ZIP creation
  - Tar.gz creation
  - Directory structure preservation
  - Error on invalid extension
  - Error when both --output and --archive provided

**Conflict with --output:**
```bash
# ERROR: cannot use both
git-de HEAD~5 -o ./export -a export.zip
```

---

### Feature 4: TUI (Terminal User Interface)

**Priority:** 4 (lowest)  
**Release:** v0.4.0 (final)

#### Specification

Add `--tui` flag for interactive commit and file selection.

**Behavior:**
- Launches interactive UI when `--tui` flag provided
- If commits provided as args, skip to file selection
- File selection: all files start selected, user can toggle
- Deleted files shown but disabled
- Path input with tab completion for output directory
- Final confirmation before export
- In-TUI progress bar during export

**CLI Changes:**
```bash
# Full interactive mode (select commits + files)
git-de --tui

# Pre-fill commits, skip to file selection
git-de HEAD~5 HEAD --tui
```

**UI Flow:**

**Screen 1: Commit Selection** (skip if commits provided)
```
┌─────────────────────────────────────────┐
│  git-de - Select From Commit            │
│                                         │
│  ▸ abc1234 feat: add new feature        │
│    def5678 fix: bug fix                 │
│    ghi9012 docs: update readme          │
│    [Search...]                          │
└─────────────────────────────────────────┘
```

**Screen 2: To Commit** (if not provided)
```
┌─────────────────────────────────────────┐
│  git-de - Select To Commit              │
│                                         │
│  ▸ HEAD (default)                       │
│    abc1234 feat: add new feature        │
│    [Search...]                          │
└─────────────────────────────────────────┘
```

**Screen 3: File Selection**
```
┌─────────────────────────────────────────┐
│  git-de - Select Files (5 files)        │
│                                         │
│  [✓] A: cmd/main.go                     │
│  [✓] M: internal/git/client.go          │
│  [✗] D: old/file.txt   (deleted)        │
│  [✓] R: pkg/utils.go                    │
│                                         │
│  [Space:toggle] [A:all] [N:none]        │
│  [Enter:export] [Q:cancel]              │
└─────────────────────────────────────────┘
```

**Screen 4: Output Directory**
```
┌─────────────────────────────────────────┐
│  Output Directory                       │
│                                         │
│  [./export                ]             │
│  (Tab for path completion)              │
│                                         │
│  [Continue]  [Back]  [Cancel]           │
└─────────────────────────────────────────┘
```

**Screen 5: Confirmation**
```
┌─────────────────────────────────────────┐
│  Confirm Export                         │
│                                         │
│  Export 4 files to ./export?            │
│                                         │
│  [Yes]  [No, go back]  [Cancel]         │
└─────────────────────────────────────────┘
```

**Screen 6: Progress**
```
┌─────────────────────────────────────────┐
│  Exporting...                           │
│                                         │
│  [████████████░░░░░░░░] 60%  3/5 files  │
│                                         │
│  Current: internal/git/client.go        │
└─────────────────────────────────────────┘
```

**Implementation Notes:**
- Use `github.com/charmbracelet/bubbletea` for TUI framework
- Add `TUI bool` to config
- If TUI mode:
  - Skip normal CLI validation
  - Launch bubbletea program
  - Collect all inputs interactively
  - Run export with progress updates
- Keyboard shortcuts:
  - `Space` - toggle file selection
  - `A` - select all
  - `N` - select none
  - `Enter` - proceed/confirm
  - `Q` - cancel/quit
  - `Tab` - path completion
  - Arrow keys - navigate
- Add test cases for:
  - TUI flag parsing
  - Mock TUI interactions (if possible)
  - Integration with existing export logic

**No Args Behavior:**
- Current behavior preserved: show help if no args
- TUI requires explicit `--tui` flag
- Example: `git-de` → show help (not TUI)

---

## Technical Considerations

### Dependencies

**For TUI:**
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Styling
- `github.com/charmbracelet/bubbles` - UI components (list, textinput, progress)

**For Archive:**
- Standard library `archive/zip` and `archive/tar`
- `compress/gzip` for .tar.gz support

### Testing Strategy

Each feature requires:
1. CLI flag parsing tests
2. Exporter logic tests
3. Integration tests (if applicable)
4. TDD approach: tests first, then implementation

### Release Schedule

| Version | Feature | ETA |
|---------|---------|-----|
| v0.4.0-alpha1 | Include patterns | TBD |
| v0.4.0-alpha2 | File size limit | TBD |
| v0.4.0-alpha3 | Archive export | TBD |
| v0.4.0 | TUI + all features | TBD |

### Documentation Updates

Each release requires:
- Update README.md with new flags
- Update examples section
- Add to CHANGELOG

---

## Open Questions

1. Should config file support be added? (e.g., `.git-de.yaml`)
2. Should shell completions be included?
3. JSON/CSV output format for scripting?
4. Git hooks integration?

---

## Completed Features Archive

### v0.3.0 Features
- ✅ Verbose mode (`-v`)
- ✅ Progress bar (default, silent mode)
- ✅ Ignore patterns (`-i`, comma-separated)
- ✅ Better warning messages
- ✅ Renamed/copied file details

### v0.2.0 Features
- ✅ Preview mode (run without `-o`)

### v0.1.0 Features
- ✅ Basic export functionality
- ✅ Concurrent copying
- ✅ Overwrite protection
- ✅ Cross-platform builds
