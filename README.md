# dagim

dagim is a text-based editor for simple directed acyclic graphs.

![dagim demo](docs/demo.gif)

Dagim allows users to create nodes, and then specify parents
and children for those nodes, with an interface designed for 
fast entry and navigation. Users can also see the list of 
all unblocked nodes and then mark nodes complete to unblock
their children child nodes and bring them to the top.

*Motive:* People often think in graphs without realizing it, but 
graph tools are complicated. It's often easier to just write a 
bullet list outline even if the ideas we're processing aren't 
tree-based. This can limit our thinking.

One example is recipes. (Check out the [gumbo example](examples/gumbo.dagim).)
Cookbook recipes are a linear tsort projection of a DAG.
Recipes in DAG form can encourage more flexibility and
collaboration.  Generally, almost any process with multiple 
dependencies is better represented as a DAG. But easy tools 
and utilities for graphs lag other text-based tools. 
`dagim` seeks to shrink that gap somewhat.

**Instructions:** Use dagim to edit a `*.dagim` file. Press `a` to add
a node. You can add more to the top layer, or you can select 
a node and then add parents or children. Adding parents or
children has autocomplete in case you'd like to link an
already-created node.

`dagim` prevents cycles by preventing you from adding upstream
nodes as children, or downstream nodes as parents.

An edge `A -> B` means `B` stays blocked until `A` is complete.

You can also hit `Space` to "complete" a top-layer node, 
which will refresh the top list to show the newly parentless
nodes. This is a useful way to process your way through a 
complex todo graph without getting overwhelmed. Toggling a 
node undone will also toggle its completed descendants undone.

`dagim` documents are intended to be source-readable, with 
useful diffs, in case you want to track them in a repository.

## Etymology

A play on vim: "dag improved", as in a small text tool for DAGs.

## Principles

The intent is to keep `dagim` as a fairly primitive, simple tool,
without bloating its UI with distracting sidecar features.

## Install From Source

Requirements:

- Go
- A terminal with enough space for a TUI

Clone the repository and install the command:

```sh
git clone <repo-url>
cd dagim
go install ./cmd/dagim
```

If `dagim` is not found after installation, add Go's bin directory to your `PATH`:

```sh
export PATH="$HOME/go/bin:$PATH"
```

## Usage

Open an example graph:

```sh
dagim examples/gumbo.dagim
```

Check a graph without opening the TUI:

```sh
dagim --check examples/gumbo.dagim
```

## Common Keys

- `a`: add node to top
- `p`: add/link parent to current node
- `c`: add/link child to current node
- `/`: search nodes
- `r`: ready/top view; nodes with no uncompleted parents
- `l`: view leaves; nodes with no children
- `Space`: toggle done/undone
- `u`: undo the last graph edit in the current session
- `o`: order remaining nodes for an ephemeral sequential list
- `v`: view completed nodes
- `C`: check diagnostics
- `W`: rewrite IDs/canonical format
- `q`: quit

## Status

Experimental and actively dogfooded. File format and UI details may still change.

## License

dagim is licensed under the GNU General Public License v3.0 or later. See [LICENSE](LICENSE).
