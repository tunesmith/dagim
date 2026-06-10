# dagim

dagim is a terminal editor for small, single-file directed acyclic graphs.

![dagim demo](docs/demo.gif)

Use dagim for dependency-shaped notes, plans, recipes, and todo graphs that do
not fit cleanly into an outline. It is built for fast keyboard entry, parent and
child linking, graph navigation, and working through the currently unblocked
nodes.

An edge `A -> B` means only that `B` stays blocked until `A` is complete.

## Why

People often think in graphs, but most writing tools make trees cheaper than
graphs. It is easy to reach for a bullet list even when the thing being modeled
has multiple dependencies, shared prerequisites, or several valid paths through
it.

One example is recipes. (Check out the [gumbo example](examples/gumbo.dagim).)
Cookbook recipes are a linear tsort projection of a DAG. Recipes in DAG form can
make parallel prep and dependencies more visible, and collaboration more possible.

dagim is intentionally primitive: one readable text file, one DAG, stable node
IDs, durable completion state, and a small TUI for editing and traversal.

## Install

Requirements:

- Go 1.26 or newer
- A terminal with enough space for a TUI

Install the command:

```sh
go install github.com/tunesmith/dagim/cmd/dagim@latest
```

If `dagim` is not found after installation, add Go's bin directory to your `PATH`:

```sh
export PATH="$HOME/go/bin:$PATH"
```

## Quick Start

Create or open a graph:

```sh
dagim my-plan.dagim
```

Inside the TUI:

1. Press `a` to add a node.
2. Select a node and press `p` or `c` to add or link a parent or child.
3. Press `r` to see ready nodes: incomplete nodes whose parents are complete.
4. Press `Space` to mark a ready node complete and unblock its children.

dagim prevents cycles by rejecting links that would make an upstream node a
child, or a downstream node a parent. Toggling a node undone also toggles its
completed descendants undone, so completion state stays dependency-valid.

Open an example graph:

```sh
dagim examples/gumbo.dagim
```

Check a graph without opening the TUI:

```sh
dagim --check examples/gumbo.dagim
```

## File Format

dagim files are source-readable UTF-8 text with stable IDs and useful diffs:

```text
# dagim v1

node prep-rice: Prep rice
  complete

node cook-gumbo: Cook gumbo
  parent prep-rice  # Prep rice
```

Node IDs are generated from node text when you create nodes in the TUI. The text
can be edited later without changing identity.

## Common Keys

- `a`: add node to top
- `Enter`: focus selected node
- `p`: add/link parent to current node
- `c`: add/link child to current node
- `e`: edit current node text
- `x`: unlink selected parent or child
- `d`: delete current node
- `/`: search nodes
- `r`: ready/top view; nodes with no uncompleted parents
- `l`: view leaves; nodes with no children
- `Space`: toggle done/undone
- `u`: undo the last graph edit in the current session
- `o`: order remaining nodes for an ephemeral sequential list
- `J` / `K`: reorder the selected visible item
- `v`: view completed nodes
- `R`: reset all completion state
- `C`: check diagnostics
- `W`: rewrite IDs/canonical format
- `?`: show help
- `q`: quit

## Etymology

A play on vim: "dag improved", as in a small text tool for DAGs.

## Status

Actively maintained and dogfooded. The `.dagim` file format is versioned
(`# dagim v1`); compatibility should be preserved for v1 files unless a future
format version is introduced.

## License

dagim is licensed under the GNU General Public License v3.0 or later. See [LICENSE](LICENSE).
