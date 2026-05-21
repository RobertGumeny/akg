# AKG v1 Public API

The root Go package intentionally exposes the smallest useful boundary for the
AKG v1 reference implementation.

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

`Create` creates and opens a new AKG file. `Open` uses ordinary strict
validation/open semantics. `Validate` checks that a file opens under those
ordinary semantics. `Commit` durably appends pending mutations, `Close` commits
outstanding mutations, and both package-level `Compact` and `(*Store).Compact`
perform explicit compaction without exposing flush or recovery controls.

Reads return only the current live nodes and edges. They hide internal storage
records, indexes, write logs, deleted records, and older replaced versions.

## v1 read-helper policy

Milestone 3 keeps the core read surface to exact lookup plus whole-state lists:

- `GetNode(typeName, id)` and `GetEdge(fromNode, relation, toNode)` return one
  current live record by its authoritative identity.
- `ListNodes()` and `ListEdges()` return current live records only, primarily so
  callers can inspect, export, validate, or build their own indexes above core.

The v1 core **does not add** helpers for tag lookup, outbound edge listing, or
inbound edge listing. Those access patterns map to existing v1 derived keys
(`t:`, `e:`, and `ei:`), but exposing them now would start turning the reference
implementation into a convenience/query layer. SDKs and applications can build
those helpers from `ListNodes`/`ListEdges` or maintain their own read indexes
above AKG core without changing the file format.

This keeps the reference API aligned with the conformance role of the Go
implementation: create/open/validate/mutate/commit/compact files and expose the
current logical state, not provide a planner, traversal language, graph query
engine, or SDK convenience surface.

## Intentionally not exported

The v1 API does not expose raw WAL internals, derived index mutation,
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
