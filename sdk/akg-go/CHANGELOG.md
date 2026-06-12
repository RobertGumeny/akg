# Changelog

## v0.2.0

### Changed

- **Logical-append commit** — `Commit` now appends only the new mutation records (plus a `COMMIT` marker) to the WAL, writing the compaction-baseline `Data`/`Bloom` sections back unchanged and growing only the WAL. Previously every commit re-materialized the entire `Data` section from live state, rewriting the whole file and recording each mutation twice (once in `Data`, once in the WAL). `Compact` re-establishes the baseline. The Go SDK is now byte-identical to the TypeScript SDK and the Go Reference SDK on commit (proven by the `parity-commit-append.akg` golden).
- **⚠️ Potentially breaking — unified key validation** — type, relation, and tag names are no longer restricted to snake_case (`[a-z0-9_]`); casing and word-separation are an application convention, not a format rule (spec `01`/`04`). All key components (type, relation, tag, node id) now share one rule: non-empty, valid UTF-8, no `:` delimiter, at most 64 UTF-8 bytes. The node-id length cap switches from runes to UTF-8 bytes, and the 64-byte cap is newly applied to type/relation/tag (previously uncapped). Oversize or otherwise invalid keys raise `ErrInvalidInput`. **Migration impact:** keys longer than 64 UTF-8 bytes that were accepted before v0.2.0 now error; the change only adds validation, so previously well-formed short keys are unaffected.
- **Read-side secondary indexes** — `ListNodesByTag`, `OutboundEdges`, and `InboundEdges` are now O(matches) instead of O(total store size), backed by derived in-memory indexes (tag→nodes, from-node→edges, to-node→edges) rebuilt at load from the primary records. `DeleteNode`'s incident-edge check and `DeleteNodeCascade`'s collection are now O(degree). There is no format change — the persisted derived keys remain the load/validation source of truth.

### Fixed

- **Crash-atomic commit** — `Commit` (and the auto-flush and initial-create paths it shares) now writes through the same atomic temp → fsync → rename → directory-fsync sequence that compaction already used, instead of rewriting the live file in place with `O_TRUNC`. A crash or power loss mid-commit can no longer tear a previously committed `.akg` store; the rename either fully lands or doesn't. The in-place `writeFileSync` writer has been removed. File permissions are preserved across writes (new files honor umask, matching the TypeScript SDK).
- **Docs-graph version stamp** — the embedded documentation graph stamped a stale hard-coded version (`0.1.1`) into every node. The version is now sourced from the latest released `## vX.Y.Z` heading in this CHANGELOG — the single source of truth for the git-tag-versioned Go SDK — so `akg-go docs` reports the shipped version.

## v0.1.4

### Added

- **Automatic flush safety valve** — the store now auto-commits buffered mutations once the pending buffer **or** the uncompacted WAL crosses the spec-recommended thresholds (1,000 entries or 10 MB, whichever comes first; `docs/spec/05-wal.md`). This bounds in-memory and WAL growth in long-running writers without an explicit `Commit()`, matching the TypeScript SDK. It is a durability safeguard only — it appends to the WAL exactly as a manual `Commit()` would and never triggers compaction.
- **WAL introspection accessors** — `Store.UncompactedWALEntries()` and `Store.UncompactedWALBytes()` expose the size of the uncompacted WAL, mirroring the inputs to the flush policy and the TypeScript SDK's `uncompactedWALEntryCount` / `uncompactedWALByteCount`.
- **`akg-go show <PATH>`** — renders a `.akg` file as readable text, grouping nodes by the types an application invented and printing each node's title and body, with edges listed as `from -relation-> to`. High-volume node types are collapsed unless `--all` is passed; `--json` emits the full `Snapshot`. The human-facing companion to the reference CLI's JSON `akg inspect`.

### Fixed

- **`go install` of the `akg-go` CLI** — the `//go:embed`-ed docs graph (`docs/akg-go-docs.akg`) was excluded by the blanket `*.akg` gitignore rule, so a clean checkout or `go install .../cmd/akg-go@latest` failed to compile (`pattern akg-go-docs.akg: no matching files found`). The generated graph is now committed via a gitignore exception, so the CLI builds from a fresh tree.

### Changed

- **Single `akg-go` CLI** — the command-line tools are now one multiplexer binary in the conventional `akg-go <command> [args]` shape. The former `akg-go-docs` and `akg-go-docs-gen` binaries are now the `akg-go docs` and `akg-go gen-docs` subcommands; behavior is unchanged.

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
