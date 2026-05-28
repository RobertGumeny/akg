# Changelog

## v0.1.3

### Fixed

- **Edge payload decoding** — `strength` and `confidence` fields encoded as integers (e.g. `1` or `0`) are now correctly decoded. MessagePack encodes whole-number floats as integers, so a value like `1.0` round-trips as `uint64(1)`. The decoder now accepts both `float64` and `uint64` for these fields.

## v0.1.2

### Added

- **Docs CLI** — `akg-go-docs` binary with four sub-commands: `overview` (type-grouped summary of the API), `explain <Name>` (full detail for a symbol with its relations), `search <query>` (substring match across titles), and `dump [--format markdown|json]` (full graph export). The CLI reads from an embedded, pre-built AKG graph so no external files are needed at runtime.
- **Embedded docs graph** — `docs` sub-package exposes the compiled `akg-go-docs.akg` graph as `docs.Graph ([]byte)`, enabling programmatic access to the documentation graph via `akg.OpenBytes`.
- **`akg.OpenBytes`** — opens a store from an in-memory byte slice rather than a file path; used by the docs CLI and useful for testing or embedding pre-built graphs.

## v0.1.1

### Added

- **Filtering** — `ListNodesFiltered(NodeFilter)` and `ListEdges(EdgeFilter)` let callers filter live nodes/edges by type, tag, relation, and metadata key-value pairs. Multiple fields combine with AND semantics.
- **Snapshot** — `Snapshot()` returns a point-in-time `Snapshot{Nodes, Edges}` of all live records; the struct is JSON-serializable.
- **Batch get** — `GetNodes([]NodeRef)` fetches multiple nodes in one call; missing refs return nil slots (no error).
- **Recency queries** — `RecentNodes(RecencyFilter)` and `RecentEdges(EdgeRecencyFilter)` return records ordered by `updatedAt` descending. Support time-window bounds (`SinceUpdatedAt`, `UntilUpdatedAt`) and `Limit` (negative limit returns `ErrInvalidInput`).
- **Compaction** — `Compact()` rewrites the `.akg` file to a minimal snapshot, removing superseded WAL entries and reducing file size.
- **Reconcile** — `ReconcileOutboundEdges(source, relation, desired, fields)` atomically syncs the outbound edge set for a source node to exactly `desired`, adding and removing as needed. Returns a `ReconcileResult{Added, Removed, Unchanged}`.
- **Cascade delete** — `DeleteNodeCascade(type, id)` deletes a node and all its inbound/outbound edges in one operation. Returns a `CascadeDeleteResult` with counts.
- **Behavioral parity test suite** — shared fixtures in `testdata/behavior/` (`parity-graph.akg`, `parity-spec.json`) and a `behavior_parity_test.go` that asserts Go SDK behavior against the spec.

## v0.1.0

Initial release. Core store operations: `Open`, `PutNode`, `GetNode`, `DeleteNode`, `PutEdge`, `GetEdge`, `DeleteEdge`, `ListNodes`, `ListEdgesFrom`, `ListEdgesTo`, `AddTag`, `RemoveTag`, `Close`.
