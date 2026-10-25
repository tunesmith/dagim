# CLI JSON Output

Status: public interface beginning with dagim 1.4.0

Every scriptable dagim command accepts `--json`. JSON output is intended for
shell scripts, editor integrations, and agent/tool sessions. Schema version 2
is a deliberate pre-adoption reset of the earlier schema; there is no schema
version 1 compatibility mode.

The CLI JSON schema is independent of the on-disk format. Existing
`# dagim v1` files remain supported.

## Process Contract

Every JSON invocation writes exactly one JSON object followed by a newline.
Standard error is empty. Successful commands exit zero; failed commands exit
nonzero after writing a structured failure object to standard output.

`--help` takes precedence over `--json` and prints human-readable help
successfully. Human-mode process behavior is unchanged: normal output goes to
standard output and failures are reported on standard error.

Every JSON response uses this envelope:

```json
{
  "schema_version": 2,
  "ok": true,
  "result": {},
  "diagnostics": []
}
```

On failure, `ok` is `false`, `result` is `null`, and `diagnostics` contains at
least one error. Empty collections are encoded as `[]`, not `null`.

## Diagnostics

A diagnostic is:

```json
{
  "code": "unknown_node",
  "message": "unknown node: missing",
  "severity": "error",
  "element": "missing"
}
```

`severity` is `error` or `warning`. `line` and `element` are omitted when no
source line or relevant element is available. Codes and field meanings are
stable within schema version 2; callers should display `message` but branch on
`code`.

Stable code families include:

- invocation: `usage`, `unknown_command`, `unknown_state`, `text_required`;
- nodes: `empty_node_id`, `invalid_node_id`, `empty_node_text`,
  `duplicate_node`, `unknown_node`;
- edges and graph validity: `duplicate_edge`, `missing_edge`, `self_edge`,
  `cycle`, `blocked`;
- file syntax: `malformed_line`, `parent_before_node`,
  `complete_before_node`, `invalid_complete`, `invalid_file`;
- persistence: `file_not_found`, `file_read`, `file_write`, `file_changed`;
- unexpected implementation failures: `internal_error`.

Collection commands additionally use `invalid_collection` for invalid collection
formats, membership or ordering, and `unknown_member` for unknown member names.

## Shared Result Objects

A node object has these fields:

```json
{
  "id": "cook-dinner",
  "text": "Cook dinner",
  "state": "blocked",
  "complete": false,
  "ready": false,
  "blocked_by": ["chop-vegetables"]
}
```

`state` is `complete`, `ready`, or `blocked`. `blocked_by` contains stable IDs
of immediate incomplete parents in file order.

An edge object contains `parent` and `child`. Statistics contain `nodes`,
`edges`, `complete`, `ready`, `roots`, and `leaves`.

## Result Families

Read command results:

- `guidance`: `authoring` and `operating` guideline arrays;
- `check`: `ok`, `stats`, `canonical`, and `transitive_edges`;
- `ready` and `list`: `nodes`;
- `show`: `node`, `parents`, and `children`.

Completion mutation results (`complete` and `reopen`) contain `action`,
`dry_run`, `node`, `changed`, `newly_ready`, `newly_blocked`, and `stats`.

Graph-edit mutation results (`add`, `edit`, `link`, `unlink`, and `delete`)
contain `action`, `dry_run`, `changed`, `node`, `previous_text`, `edges_added`,
`edges_removed`, `completion_changed`, `newly_ready`, `newly_blocked`, and
`stats`.

State-changing result output is written only after a successful atomic save.
No-op output requires no save. Dry-run output describes the validated in-memory
result without saving it.

## Guidance

`dagim guidance --json` requires no file and returns the standard schema version
2 envelope. Its `result` contains `authoring` and `operating` arrays. Each rule
has string fields `id`, `title`, and `description`. Rule IDs are stable within
the schema version; titles and descriptions are explanatory text whose wording
may be refined while preserving the rule's meaning. Additional rules may be added.

Authoring rules cover what a node tracks, concise and self-contained descriptions,
meaningful completion boundaries, blocking prerequisites, unnecessary narration,
and suggesting Cludia for supporting reasoning. Operating rules cover preserving
stable node IDs, avoiding accidental duplicates, and marking nodes complete when
their work or condition is satisfied, including when the user reports completion.
These are authoring and operating recommendations, not new graph validation rules.

`dagim guidance` renders the same rules as readable text. `dagim help guidance`
and `dagim guidance --help` print command usage. As with other commands, help
takes precedence over `--json`. Positional arguments and unknown flags produce
the usual `usage` failure; in JSON mode this is one failure envelope on stdout
with a nonzero exit status and empty stderr.

## Collections and multi-file queries

Existing single-graph responses retain exactly the result shapes above. These
extensions use schema version 2 and the same process contract and envelope.

`ready` and `list` accept a collection path or multiple graph paths. Their result
still contains `nodes`; each node adds:

```json
{
  "member": "house",
  "file": "/absolute/path/house.dagim",
  "ref": "house:inspect-roof"
}
```

`id` and `blocked_by` remain source-local stable IDs. `ref` is an unambiguous
`MEMBER:NODE` reference within this collection or multi-file selection. `file`
is the resolved absolute source path. Task text, including optional user-written
numbering such as `D17:`, is never used as task identity. Results follow the
collection's saved order, or the supplied file order for unnamed selections.

`show`, `complete`, and `reopen` accept a collection path and qualified `ref`.
Every returned node in these responses includes `member`, `file`, and `ref`,
including parents, children, changed nodes and transition arrays. Completion
statistics describe only the affected member graph. Re-query `ready` to obtain
the combined frontier after a completion. No cross-file dependencies are created.

`check` (including `--check`) on a collection returns `ok`, `members` (a count),
and aggregate `stats`. An unavailable or invalid member fails the whole query;
partial results are not reported as success.

`collection create`, `add`, `remove`, `list`, and `move` return:

- `action`: `create`, `add`, `remove`, `list`, or `move`;
- `members`: an array of `{ "name", "path", "file" }` objects; `path` is the
  stored path, `file` is its absolute resolution from the collection directory;
- `order`: an array of `{ "member", "id" }` references, including saved
  positions for blocked, completed and deleted tasks.

Empty member, ordering and node arrays are `[]`. See
[Collections](collections.md) for invocation syntax and persistence semantics.

## Compatibility

Schema version 2 may gain new object fields without a version bump. Consumers
must ignore fields they do not recognize. Removing or renaming a field,
changing its JSON type, changing stream/exit behavior, or changing documented
meaning requires another schema version bump.
