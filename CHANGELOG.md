# Changelog

## 1.4.0 (2026-08-29)

### Added

- JSON schema version 2 uses a uniform success/failure envelope with structured
  diagnostics and deterministic stdout, stderr, and exit-status behavior.
- Selectable text views now use persistent edge-only scrolling and
  rendered-height-aware selection paging, including Search and link matches.

### Changed

- Text screens separate body content from responsive command footers with a
  blank row; the graph-map canvas remains tightly fitted.
- Terminal wrapping, truncation, and command-grid measurement are now
  grapheme-safe and display-cell-aware.
- Atomic saves preserve permissions and sync the containing directory; first
  creation refuses to overwrite a concurrently created file.
- CLI and TUI reads share ordered query projections for node state, relations,
  search, frontiers, and graph transitions.

### Compatibility

- The file format remains `# dagim v1`; existing files require no migration.
- JSON schema version 1 is intentionally replaced without a compatibility mode
  as a one-time pre-adoption contract reset.

## 1.3.1 (2026-08-02)

### Fixed

- Selectable TUI views now keep the highlighted Node or Order item inside the
  visible viewport during navigation, ordering picks, undo/reset, terminal
  resize, reload, and manual page scrolling.

### Compatibility

- The `.dagim` file format and CLI JSON schema remain unchanged.

## 1.3.0 (2026-08-02)

### Added

- Read-only TUI graph map on `g`, with ready nodes aligned at the left of the
  remaining-work view, parent/child and within-column navigation, and optional
  completed history.
- Graph maps hide diagnosed transitive edges by default with a `t` toggle,
  highlight inbound and outbound edges incident to the selected node, and
  prefer nearby ranks for horizontal navigation.
- Opposite horizontal navigation retraces exact traversal history, and mixed
  junctions remain neutral instead of coloring unrelated edge arms while
  selected collinear continuations resume highlighting past the junction.
- A full-width selected-node inspector wraps long descriptions above the graph
  viewport while vertical navigation keeps the selected card visible.
- Horizontal graph navigation retains edge context on both sides of the
  selected card instead of aligning its border directly with the viewport.

### Compatibility

- The `.dagim` file format and CLI JSON schema remain unchanged.

## 1.2.0 (2026-08-01)

### Added

- Automatic TUI reloads when another process changes the open Dagim file.

### Changed

- TUI autosaves now refuse to overwrite unexpected external file changes.
- External reloads preserve the focused node when possible, clear stale undo
  history, and leave invalid external contents untouched with an error message.

### Compatibility

- The `.dagim` file format remains `# dagim v1`.
- The CLI JSON schema remains version 1 with no output contract changes.

## 1.1.0 (2026-07-17)

### Added

- Scriptable read commands: `check`, `ready`, `list`, and `show`.
- Versioned JSON output for every scriptable command.
- Completion commands: `complete` and cascading `reopen`, including
  `reopen --dry-run`.
- Graph editing commands: `add`, `edit`, `link`, and `unlink`.
- Node deletion with incident-edge and frontier reporting, including
  `delete --dry-run`.
- Atomic multi-link node creation with repeatable `--parent` and `--child`
  flags.
- Transition reporting for newly ready, newly blocked, and reopened nodes.

### Compatibility

- The `.dagim` file format remains `# dagim v1`.
- `dagim FILE` remains the default TUI invocation.
- `dagim --check FILE` remains available as a compatibility alias for
  `dagim check FILE`.
- CLI JSON schema version 1 permits additive fields; breaking output changes
  require a schema version bump.

## 1.0.0

- Initial stable release of the single-file DAG terminal editor and v1 file
  format.
