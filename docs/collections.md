# Collections

A collection is a named view over independent Dagim graphs. It stores member
paths, short member names, and a combined task order. It contains no task text,
completion state, or dependencies. The same graph can appear in multiple
collections, each with its own membership and order.

## Creating and opening

```sh
dagim collection create daily
dagim collection add daily house.dagim --as house
dagim collection add daily work.dagim --as work
dagim daily
```

Member files must already exist and be valid graphs. Names may contain letters,
digits, `.`, `_`, and `-`, except for the names `.` and `..`. Without `--as`,
the name defaults to the file's basename with its last extension removed.
Names must be unique within the collection. The same physical file cannot be
included twice, including through symlinks or hard links. Nested collections
and dependencies between files are not supported.

Filenames and extensions are unrestricted. Dagim recognizes a collection by
`# dagim collection v1` as its first nonblank line. A missing or empty file
opened with `dagim FILE` still starts a graph, even if its name looks like a
collection filename. Create a collection explicitly; `collection create`
refuses to overwrite any existing file. Existing headerless graphs remain valid.
Unsupported collection versions produce an error.

The creation and membership commands also accept `--json`.

## Combined TUI

- `j`/`k`, arrows: select a task.
- `PgUp`/`PgDn`, `Home`/`End`: navigate the list.
- `Space`: complete the selected ready task in its source graph.
- `J`/`K`: move the selected task across file boundaries; save collection order.
- `v`: include completed tasks. `Space` on a completed task reopens it and its
  completed descendants, within its source graph.
- `/`: filter by member name, task text, or stable node ID. `Esc` clears the filter.
- `Enter`: focus a task and navigate its parents and children.
- `?`: help. `q`: quit from the ready list.

In node view, `j`/`k` selects a parent or child, and `Enter` follows it, including
blocked tasks that are not in the ready list. Each related task shows whether it
is ready, blocked, or complete. A blocked focus also lists its remaining blockers.
Completed relations are hidden until `v` is enabled, matching the ready list's
completed-node toggle. The focused node stays open when its completion changes.

`Space` completes or reopens the focused node in its source graph. Completing
a blocked task is rejected until all its parents are complete; reopening still
cascades through completed descendants. `Esc` or `Backspace` retraces navigation,
restoring the previous relation selection. `r` or `q` returns directly to the
combined ready list. `PgUp`/`PgDn` and `Home`/`End` navigate relations; help can
be opened with `?` and closed without leaving the focused node.

Navigation preserves qualified identity and selection across external changes.
If a focused node or its member is removed, navigation returns to a surviving
previous node or the ready list. Unavailable or invalid member files retain the
last valid view with mutations disabled until the problem is repaired.

Every row identifies its member. User-written `D17:` labels and stable node IDs
may repeat across members. An individual graph's full TUI continues to work as
before, with its own ordering. Use it or the source-file CLI for graph edits.

The collection and its members reload every half second and before an action.
An invalid, missing, or unreadable member produces a visible error; the last
valid list remains visible with completion and ordering actions disabled until
the problem is repaired. CLI queries fail instead of returning a partial ready
list. `collection list` and `collection remove` remain usable to repair membership.
Saves use atomic replacement and check for changes since the document was read.

## Ordering

The saved order identifies nodes by member name and stable node ID. It can
include blocked and completed nodes, although the ready view hides them.
Completing a node retains its saved position, so reopening it restores that
position. Deleted node references are ignored; restoring the same ID restores
its saved position. New nodes without a saved position follow the saved order,
in member order and then source-file order. Reordering records those nodes too.

Reordering never changes a member graph's order. Removing a member removes
its saved ordering entries, leaving its graph untouched; adding it again starts
its tasks at the end. Member order follows addition order.

## CLI and agent access

```sh
dagim collection list daily --json
dagim collection add daily another.dagim --as another --json
dagim collection remove daily another --json

dagim ready daily --json
dagim list daily --state blocked --json
dagim show daily house:inspect-roof --json
dagim complete daily house:inspect-roof --json
dagim reopen daily house:inspect-roof --dry-run --json
dagim reopen daily house:inspect-roof --json
dagim check daily --json

dagim collection move daily house:inspect-roof --before work:review-parser --json
dagim collection move daily house:inspect-roof --after work:review-parser --json
```

References use `MEMBER:NODE`, with the stable node ID, not a number extracted
from task text. An agent can use each returned node's `ref` directly. It can
alternatively call existing single-file commands using the returned absolute
`file` and local `id`. Mutation statistics describe the affected source graph;
query the collection again for the complete combined frontier after a mutation.

`ready` and `list` also accept multiple graph files for an unnamed read-only
selection:

```sh
dagim ready house.dagim work.dagim --json
```

This selection uses basename-derived member names and the supplied file order.
For colliding basenames, unusual filenames requiring explicit names, or saved
cross-file ordering, create a collection and use `--as`.

Graph edits (`add`, `edit`, `link`, `unlink`, `delete`) use the individual source
file, keeping dependencies within one graph. Collection paths are rejected by
the graph parser, including a header-only collection, preventing accidental
conversion or overwriting through graph commands.

## Format and portability

An empty collection is:

```text
# dagim collection v1
{
  "members": [],
  "order": []
}
```

A populated collection might contain:

```text
# dagim collection v1
{
  "members": [
    {"name": "house", "path": "house.dagim"},
    {"name": "work", "path": "plans/work.dagim"}
  ],
  "order": [
    {"member": "work", "id": "review-parser"},
    {"member": "house", "id": "inspect-roof"}
  ]
}
```

Paths added through the CLI are stored relative to the collection file. They
are resolved from its directory, regardless of the process's working directory.
Moving a folder containing the collection and its members preserves those
relationships. When opening the collection through a symlink, its real location
determines the base directory. Hand-written absolute member paths are accepted.

The collection format version, graph format version (`# dagim v1`), and CLI JSON
schema version are independent.
