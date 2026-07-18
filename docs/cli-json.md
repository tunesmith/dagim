# CLI JSON Output

Status: public interface beginning with dagim 1.1.0

Every scriptable dagim command accepts `--json`. JSON output is intended for
shell scripts, editor integrations, and agent/tool sessions.

## Process Contract

On success:

- Exit status is zero.
- Standard output contains exactly one JSON object followed by a newline.
- Standard error is empty.

On failure:

- Exit status is nonzero.
- Standard output is empty.
- Standard error contains a human-readable diagnostic. Errors are not JSON in
  schema version 1.

Node and edge arrays use the graph's stable file order. Empty arrays are encoded
as `[]`, not `null`.

## Schema Compatibility

`schema_version` identifies the CLI JSON schema, independently of the
`# dagim v1` file format.

Schema version 1 may gain new object fields without a version bump. Consumers
should ignore fields they do not recognize. Removing or renaming a field,
changing its JSON type, or changing its documented meaning requires a schema
version bump.

## Shared Objects

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

`state` is `complete`, `ready`, or `blocked`. `blocked_by` contains the stable
IDs of immediate incomplete parents.

An edge object is:

```json
{
  "parent": "prepare-ingredients",
  "child": "cook-dinner"
}
```

Statistics contain `nodes`, `edges`, `complete`, `ready`, `roots`, and `leaves`.

## Response Families

Read commands:

- `check`: `schema_version`, `ok`, `stats`, `canonical`, and
  `transitive_edges`.
- `ready` and `list`: `schema_version` and `nodes`.
- `show`: `schema_version`, `node`, `parents`, and `children`.

Completion mutations (`complete` and `reopen`) return:

- `schema_version`, `action`, `dry_run`, and `node`.
- `changed`: nodes whose completion boolean changed.
- `newly_ready` and `newly_blocked`: existing nodes whose state changed.
- `stats`: statistics after the operation, or after the preview for a dry run.

Graph edits (`add`, `edit`, `link`, `unlink`, and `delete`) return:

- `schema_version`, `action`, `dry_run`, and `changed`.
- `node`: the added, edited, or deleted node; otherwise `null`.
- `previous_text`: the prior label for `edit`; otherwise `null`.
- `edges_added` and `edges_removed`.
- `completion_changed`: nodes reopened to preserve completion validity.
- `newly_ready` and `newly_blocked`.
- `stats`: statistics after the operation, or after the preview for a dry run.

State-changing mutation output is written only after a successful atomic save.
No-op output requires no save. Dry-run output describes the validated in-memory
result without saving it.
