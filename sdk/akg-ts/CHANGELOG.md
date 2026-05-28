# Changelog

## v0.1.1

### Added

- **Filtering** — `store.listNodesFiltered(NodeFilter)` and `store.listEdges(EdgeFilter)` filter live nodes/edges by type, tag, relation, and metadata key-value pairs. Multiple fields combine with AND semantics.
- **Snapshot** — `store.snapshot()` returns a `Snapshot` (`{ nodes, edges }`) of all live records at a point in time.
- **Batch get** — `store.getNodes(NodeRef[])` fetches multiple nodes in one call; missing refs return null slots (no error).
- **Recency queries** — `store.recentNodes(RecencyFilter)` and `store.recentEdges(EdgeRecencyFilter)` return records ordered by `updatedAt` descending. Support time-window bounds (`sinceUpdatedAt`, `untilUpdatedAt`) and `limit` (negative limit throws `InvalidInputError`).
- **Compaction** — `store.compact()` rewrites the `.akg` file to a minimal snapshot, removing superseded WAL entries.
- **Reconcile** — `store.reconcileOutboundEdges(source, relation, desired, fields)` atomically syncs the outbound edge set for a source node to exactly `desired`. Returns a `ReconcileResult` with `added`, `removed`, and `unchanged` counts.
- **Cascade delete** — `store.deleteNodeCascade(type, id)` deletes a node and all its inbound/outbound edges. Returns a `CascadeDeleteResult` with counts.
- **Behavioral parity test suite** — shared fixtures in `testdata/behavior/` and a `behavior_parity.test.ts` that asserts TypeScript SDK behavior against the spec.
- **New exported types** — `RecencyFilter`, `EdgeRecencyFilter`, `ReconcileResult`, `CascadeDeleteResult`, `Snapshot`.

## v0.1.0

Initial release. Core store operations: `open`, `putNode`, `getNode`, `deleteNode`, `putEdge`, `getEdge`, `deleteEdge`, `listNodes`, `listEdgesFrom`, `listEdgesTo`, `addTag`, `removeTag`, `close`.
