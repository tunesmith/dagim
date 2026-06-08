# dagim Future Ideas

These ideas are intentionally outside the next focused changeset. They should be revisited after more real use.

## Workflow

1. Add explicit transitive-reduction prune/simplify command with preview.
2. Add undo for graph-editing actions.

## Export

1. Add DOT/Graphviz export.

## Alternate Interfaces

1. Add `dagim --gui FILE` as a lightweight graphical interface over the same `.dagim` file. The Go binary could start a local `127.0.0.1` web server on a random port, serve embedded HTML/JS assets from the binary, render the graph with a browser-side layout engine such as ELK/elkjs, and POST edits back to Go for validation and canonical save to the original file path. This should remain a quick-launch one-file editor/viewer, not a full desktop app or separate project database.
2. Add an outline projection that chooses a primary-parent tree from the DAG and shows secondary parents as crosslinks/aliases. This may be less necessary if a `g` hotkey can launch a read-only web graph view with an ELK layout from inside the TUI; keep the outline idea only if dogfooding shows a tree-shaped projection would add value beyond the graph view.
