# git-de Roadmap

## Version History

- **v0.1.0** - Initial release with basic export functionality
- **v0.2.0** - Preview mode (run without `-o` flag)
- **v0.3.0** - Verbose mode, progress bar, ignore patterns
- **v0.4.0** - Include patterns, file size limits, archive export, JSON output, TUI ✅ COMPLETE

---

## v0.4.0 Roadmap ✅ COMPLETE

Staggered release with 5 features in priority order.

### Feature 1: Include Patterns ✅ COMPLETE
Add `--include` (`-I`) flag for whitelist filtering of files.

### Feature 2: File Size Limit ✅ COMPLETE
Add `--max-size` flag to skip files exceeding size limit.

### Feature 3: Archive Export ✅ COMPLETE
Add `--archive` (`-a`) flag to export directly to archive file (zip/tar/tar.gz).

### Feature 4: JSON Output ✅ COMPLETE
Add `--json` flag to output results in JSON format.

### Feature 5: TUI (Terminal User Interface) ✅ COMPLETE
Add `--tui` flag for interactive commit and file selection.

---

## Technical Considerations

### Dependencies
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Styling
- `github.com/charmbracelet/bubbles` - UI components

### Testing Strategy
- CLI flag parsing tests: ✅ Pass
- Exporter logic tests: ✅ Pass
- Git logic tests: ✅ Pass
- Build check: ✅ Pass

### Documentation Updates
- Update README.md: ✅ Done

---

## Completed Features Archive

### v0.4.0 Features
- ✅ **Include patterns** (`-I` flag)
- ✅ **File size limit** (`--max-size` flag)
- ✅ **Archive export** (`-a` flag)
- ✅ **JSON output** (`--json` flag)
- ✅ **Interactive TUI** (`--tui` flag)

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
