# git-de Roadmap

Focus: Transitioning to a TUI-first interactive Git export experience.

## Phase 1: TUI as the Default Experience (High Priority)
Goal: Make `git-de` launch into the TUI by default when run in a terminal.

- [ ] Implement TTY detection (launch TUI only if run in a terminal).
- [ ] Auto-bypass TUI if positional arguments (from/to) are provided.
- [ ] Add `--no-tui` flag to explicitly use the classic CLI interface.
- [ ] Update CLI help to reflect the new default behavior.
- [ ] Ensure non-interactive environments (CI/CD) default to CLI.

## Phase 2: Enhanced TUI Functionality
Goal: Build out the feature set of the new TUI.

- [ ] Integrated Help Menu: Press `?` to show all keyboard shortcuts.
- [ ] Commit Search: Add `/` to search/filter through the commit list.
- [ ] File Diff Preview: Allow "peeking" at file changes before selecting for export.
- [ ] Recent Paths: Save and suggest recently used output directories.
- [ ] Custom Styling: Refine the UI with Lip Gloss (borders, status bars, colors).

## Phase 3: Core Engine Improvements
Goal: Optimize the underlying git logic and exporter.

- [ ] Shallow Clone Support: Better handling of limited history repos.
- [ ] Submodule Support: Option to include/exclude submodule changes.
- [ ] Custom Summary Formats: Allow template-based `summary.txt` generation.
- [ ] Parallel Performance: Optimize concurrent copying for huge diffs.

## Phase 4: Distribution & DevOps
Goal: Make git-de easy to install and update.

- [ ] GoReleaser Integration: Automated multi-arch builds for every tag.
- [ ] Semantic Versioning: Switch to strict semver for stability.
- [ ] Automated Testing: Expand integration tests for complex git scenarios.

---
*Roadmap updated 2026-02-13. Let's build the best Git export tool. 🦞*
