# AKG Phase 1 Public API Review

Task 7 intentionally exposes the smallest useful boundary after the internal
store behavior was tested.

## Package choice

The module root package is `akg` (`github.com/RobertGumeny/akg-format`). This avoids
introducing a broad `sdk` package while still making the file store usable.

## Exported surface

The public API is limited to:

- file lifecycle: `Create`, `Open`, `Validate`, `Compact`;
- store lifecycle: `Commit`, `Close`, `(*Store).Compact`;
- current-state writes: `PutNode`, `PutEdge`, `DeleteNode`, `DeleteEdge`;
- current-state reads: `GetNode`, `GetEdge`, `ListNodes`, `ListEdges`;
- simple public value types: `Node`, `NodeRecord`, and `Edge`.

Reads are explicitly current logical state only. They do not expose Data-section
records, derived keys, WAL records, tombstones, or superseded values.

## Intentionally not exported

The Phase 1 API does not expose raw WAL internals, derived index mutation,
recovery/salvage, merge, query language, traversal, background services,
multi-writer behavior, automatic flush controls, or a public flush API.

## CLI boundary

`cmd/akg` is limited to operational hooks:

- `akg validate PATH`;
- `akg inspect PATH`;
- `akg compact PATH`.

Inspection opens the file through ordinary store semantics and prints only live
nodes and edges plus counts, so stale WAL records and tombstones are not shown as
current state.
