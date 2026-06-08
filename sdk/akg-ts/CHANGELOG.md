# Changelog

## v0.1.4

### Added

- **`akg-ts show <PATH>`** — renders a `.akg` file as readable text, grouping nodes by the types an application invented and printing each node's title and body, with edges listed as `from -relation-> to`. High-volume node types are collapsed unless `--all` is passed; `--json` emits the full snapshot. The general-purpose companion to the `akg-ts docs` API browser, for reading any store (an agent's memory, the docs graph) without parsing the binary format by hand.
- **tsdoc on the public API** — every exported symbol (the `Store` class and its methods, the WAL accessors, the `open` factory, the exported interfaces, and the three error classes) now carries a doc comment. The comments flow into the bundled `dist/*.d.ts`, so editors show them on hover.

### Fixed

- **`akg-ts docs` after `npm install`** — the published package omitted the bundled `docs/akg-ts-docs.akg` graph that the `docs` command loads, so every `akg-ts docs` subcommand failed with `ENOENT` for installed users. Root cause: the blanket `*.akg` gitignore rule excluded the generated graph and it was missing from the package's `files` list. The graph is now committed (via a gitignore exception) and shipped in `files`, so `docs` works after a plain install.
- **Error-table docs** — the `MissingRequiredFieldError` row now documents the `putEdge` missing-identity-field case, which the SDK already throws.

## v0.1.3

### Added

- **Automatic flush safety valve** — the store now auto-commits buffered mutations once the pending buffer **or** the uncompacted WAL crosses the spec-recommended thresholds (1,000 entries or 10 MB, whichever comes first; `docs/spec/05-wal.md`). This bounds in-memory and WAL growth in long-running writers without an explicit `commit()`. It is a durability safeguard only — it appends to the WAL exactly as a manual `commit()` would and never triggers compaction.
- **WAL introspection accessors** — `store.uncompactedWALEntryCount` and `store.uncompactedWALByteCount` expose the size of the uncompacted WAL, mirroring the inputs to the flush policy.
- **Cross-SDK round-trip coverage** — `npm run generate:roundtrip` writes a deterministic `testdata/roundtrip/ts-written.akg` fixture, and `test/roundtrip.test.ts` exercises the write → commit → close → reopen path, crash-atomicity, and the incremental-commit behavior. The Go SDK reads the same fixture to prove cross-SDK compatibility.

### Changed

- **Incremental `commit()`** — a commit now appends only the new mutation records (plus a `COMMIT` marker) to the file's WAL, reusing the existing `Data`/`Bloom` bytes instead of re-materializing and rewriting the whole file. Reclaiming WAL space still requires an explicit `compact()`.
- **Crash-atomic file replacement** — every durable write now goes to a same-directory temp file that is fsynced, renamed over the target, and followed by a directory fsync. An interrupted write can no longer destroy the previously committed store; on error the temp file is cleaned up.
- **`compact()` WAL section** — a compacted file now carries a zero-length WAL section rather than omitting the WAL section entirely, matching the Go reference SDK so incremental `commit()` can append onto it.

## v0.1.2

### Added

- **Docs CLI** — `akg-ts-docs` binary (via `npx akg-ts docs`) with four sub-commands: `overview` (type-grouped summary of the API), `explain <Name>` (full detail for a symbol with its relations), `search <query>` (substring match across titles), and `dump [--format markdown|json]` (full graph export). The CLI reads from a bundled, pre-built AKG graph so no external files are needed at runtime.
- **Bundled docs graph** — `docs/akg-ts-docs.json` is the compiled documentation graph shipped with the package; the CLI loads it directly.

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
