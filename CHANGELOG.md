# Changelog

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
