# Changelog

## v0.3.0

### Fixed

- **Tag-index key collision** — two nodes sharing the same `id` across different types (e.g. a `counter` and a `tendency` node both with id `preflop__vpip`) plus a common tag collapsed to a single tag-index key. `compact()` then threw `duplicate data key` and the colliding nodes' tag membership was corrupted. Root cause: the tag-index key was `t:{tag}:{id}`, omitting the node type even though node identity is `(type, id)` — the same ambiguity the spec already type-qualifies for edges. The key is now type-qualified as `t:{tag}:{type}:{id}`, so colliding nodes stay distinct and both resolve from `listNodesByTag`.

### Changed

- **⚠️ Binary format major 1 → 2** — the type-qualified tag-index key is an on-disk format change, so the binary major is bumped to 2. **Read-compatible:** a v0.3.0 reader transparently reads existing major-1 files (3-part tag keys) as well as major-2 files (4-part), distinguished by component count. **Forward-breaking:** files written by v0.3.0 are major 2, which pre-v0.3.0 SDKs reject. **Auto-upgrade:** a major-1 file is rewritten as major 2 (with type-qualified tag keys) on its next `compact()`. No application code changes are required; the public API is unchanged.

## v0.2.0

### Changed

- **⚠️ Potentially breaking — unified key validation** — type, relation, and tag names are no longer restricted to snake_case (`[a-z0-9_]`); casing and word-separation are an application convention, not a format rule (spec `01`/`04`). All key components (type, relation, tag, node id) now share one rule: non-empty, valid UTF-8, no `:` delimiter, at most 64 UTF-8 bytes. The node-id length cap switches from code points to UTF-8 bytes, and the 64-byte cap is newly applied to type/relation/tag (previously uncapped). Oversize or otherwise invalid keys throw `InvalidInputError`. **Migration impact:** keys longer than 64 UTF-8 bytes that were accepted before v0.2.0 now error; the change only adds validation, so previously well-formed short keys are unaffected.
- **Read-side secondary indexes** — `listNodesByTag`, `outboundEdges`, and `inboundEdges` are now O(matches) instead of O(total store size), backed by derived in-memory indexes (tag→nodes, from-node→edges, to-node→edges) rebuilt at load from the primary records. Incident-edge checks on delete and cascade collection are now O(degree). There is no format change — the persisted derived keys remain the load/validation source of truth.

### Fixed

- **Docs-graph version stamp** — the bundled documentation graph stamped a stale hard-coded version (`0.1.1`) into every node. The version is now sourced from `package.json` — the single source of truth for the shipped SDK version — so `akg-ts docs` reports the shipped version.

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
