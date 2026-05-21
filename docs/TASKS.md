# Milestone 2 Tasks

Source documents:

- `docs/spec/`
- `docs/akg-go-sdk-execution-tracker.md`
- `docs/akg-reference-implementation-plan.md`
- `docs/akg-comprehensive-test-plan.md`

Archived Milestone 1 task plan: `docs/archive/milestone-1-tasks-2026-05-20.md`

## Milestone 2 Goal

Milestone 2 builds on the completed Milestone 1 binary/container primitives by adding the first logical store layer: authoritative in-memory state, live-key materialization, ordinary open with committed-WAL replay, commit durability behavior, and explicit compaction.

The implementation should remain small and spec-shaped. Do not build query planning, traversal beyond minimal exact/list helpers, merge, recovery/salvage, background services, or product SDK abstractions.

## Scope Rules

- Authoritative mutable state is **nodes + edges only**.
- `ei:`, `t:`, `ts:`, and Bloom are derived from live state when materializing files.
- Data sections contain only current live state and required derived index keys; no tombstones.
- Ordinary open is strict and fail-closed, but applies committed WAL through the last valid `COMMIT` and ignores trailing uncommitted WAL records.
- Ordinary commit appends WAL mutation records plus `COMMIT`, fsyncs the required durable state, and leaves WAL in place until compaction.
- Compaction is an explicit whole-file rewrite via atomic replacement; it rebuilds live Data/Bloom and discards prior WAL.
- Design any public API intentionally and minimally. Do not accidentally export broad SDK concepts.

## Task 1 — Implement authoritative logical state and mutation semantics

Create real `internal/state` behavior for current logical nodes and edges.

### Scope

- Represent nodes by `(type, id)` identity, with node ID separate from payload.
- Represent edges by `(from_node, relation, to_node)` identity.
- Implement apply helpers for:
  - `PutNode`
  - `PutEdge`
  - `DeleteNode`
  - `DeleteEdge`
- Enforce strict delete-not-found behavior.
- Preserve dangling-edge tolerance; do not enforce referential integrity or cascade deletes.
- Apply writer-owned field behavior for created/updated timestamps and versions at the state/write boundary.
- Generate 16-character random hex node IDs when a caller omits an ID.
- Validate duplicate tags, maximum 32 tags, and key-component constraints before accepting writes.

### Acceptance criteria

- Node upsert by `(type, id)` works and increments version on mutation.
- Edge upsert by `(from_node, relation, to_node)` works and increments version on mutation.
- Generated node IDs are 16 lowercase hex characters and satisfy key constraints.
- Caller-provided invalid IDs are rejected.
- Duplicate tags, uppercase tags, tags with spaces, malformed tags, and more than 32 tags are rejected.
- Deleting an existing node/edge succeeds; deleting a missing node/edge returns a not-found error.
- Dangling edges are accepted.
- Tests prove type change is identity change, not an in-place node mutation.

## Task 2 — Materialize live state into sorted Data entries and derived indexes

Create `internal/store` helpers that turn authoritative state into the AKG live key set.

### Scope

- Emit primary node entries:
  - `n:{type}:{id}` → node payload
- Emit primary edge entries:
  - `e:{from}:{relation}:{to}` → edge payload
- Emit derived inbound edge entries:
  - `ei:{to}:{relation}:{from}` → empty value
- Emit derived tag entries:
  - `t:{tag}:{node_id}` → empty value
- Emit derived temporal entries:
  - `ts:{updated_at}:n:{type}:{id}` → empty value
  - `ts:{updated_at}:e:{from}:{relation}:{to}` → empty value
- Sort entries by raw bytewise key order.
- Reject duplicate materialized keys.
- Drop unknown MessagePack fields on rewrite by re-encoding through canonical record structs.

### Acceptance criteria

- Materialization is deterministic for the same logical state.
- Output entries are bytewise sorted and accepted by Milestone 1 Data-section decoders.
- Derived keys are regenerated from authoritative nodes/edges and are not stored as mutable state.
- Empty-value index entries use `value_len = 0` after Data encoding.
- Deleted/superseded records do not appear in materialized output.
- Tests cover node, edge, inbound, tag, and temporal key generation.

## Task 3 — Hydrate logical state from Data sections

Implement the reverse path from decoded Data entries to authoritative current state.

### Scope

- Load node primary entries from `n:` keys and decode node payloads.
- Load edge primary entries from `e:` keys and decode edge payloads.
- Validate key/payload identity consistency where the payload repeats identity fields.
- Validate required derived index entries are present and correct, or document and test any intentionally deferred validation.
- Treat malformed known keys/payloads as ordinary-open validation failures.
- Ignore structurally valid unknown future keys only if doing so is explicitly safe and documented.

### Acceptance criteria

- A materialize → hydrate → materialize cycle preserves logical state.
- Missing or malformed primary payloads are rejected.
- Edge key identity and edge payload identity mismatches are rejected.
- Required index-key omissions or inconsistencies are either rejected or explicitly documented as deferred before implementation proceeds beyond Milestone 2.
- Unknown MessagePack fields in payloads are tolerated on read and dropped after rewrite.

## Task 4 — Implement ordinary create/open/validate behavior

Add the first file-level store open path using the Milestone 1 container, Data, Bloom, and WAL primitives.

### Scope

- Create a new AKG file with an empty live Data section and deterministic Bloom state.
- Open an existing file by:
  1. decoding and validating the container,
  2. decoding the Data section into authoritative state,
  3. replaying committed WAL records through the last valid `COMMIT`,
  4. ignoring trailing uncommitted WAL records.
- Reject corrupt containers, malformed known sections, checksum failures, malformed committed WAL, and invalid WAL payloads.
- Track the next WAL sequence number so reopened files never reuse sequence numbers.
- Track uncompacted WAL entry count and byte count for threshold decisions.

### Acceptance criteria

- Opening a clean compacted file exposes its live logical state.
- Opening a file with committed WAL applies committed mutations automatically.
- Opening a file with no valid `COMMIT` applies no WAL mutations.
- Opening a file with trailing uncommitted WAL ignores the trailing records.
- Opening a file with malformed committed WAL rejects the file.
- Sequence-number and WAL-counter behavior is tested across reopen.
- `Validate(path)`-style behavior verifies format/state consistency without mutating logical content.

## Task 5 — Implement ordinary commit and WAL lifecycle behavior

Implement the write path that was intentionally deferred from Milestone 1.

### Scope

- Buffer or stage logical mutations until commit according to the chosen minimal store design.
- On `Commit()`:
  1. append WAL mutation records,
  2. append `COMMIT`,
  3. fsync the file state required for durable recovery,
  4. leave committed WAL in place.
- Ensure clean close commits outstanding mutations unless already committed.
- Recompute or persist WAL counters needed for automatic flush thresholds.
- Implement internal threshold detection for:
  - `>= 1,000` uncompacted WAL entries,
  - `>= 10 MB` uncompacted WAL data.
- Keep automatic flush behavior internal; do not expose a public flush API.

### Acceptance criteria

- A committed mutation survives close/reopen through ordinary WAL replay.
- An uncommitted mutation is not applied after reopen.
- Multiple committed batches replay in sequence order through the last `COMMIT`.
- WAL sequence numbers are monotonic and distinct across sessions.
- Committed WAL remains present until compaction.
- Threshold detection is covered by tests, even if the actual threshold action is minimal in Milestone 2.

## Task 6 — Implement explicit compaction

Add explicit whole-file rewrite compaction.

### Scope

- Perform ordinary open semantics to obtain current live state.
- Materialize only current live keys.
- Rebuild Bloom from the live key set using fixed v1 parameters.
- Write a fresh AKG file with a fresh section table.
- Discard prior WAL contents.
- Atomically rename the compacted file over the original.

### Acceptance criteria

- Compaction preserves logical nodes and edges.
- Compaction drops WAL history and tombstones.
- Compaction regenerates `ei:`, `t:`, `ts:`, and Bloom from live state.
- The compacted file validates and reopens without WAL replay.
- Crash-safety behavior is documented and the atomic rename path is tested where practical.

## Task 7 — Define the intentionally minimal Phase 1 API/CLI boundary

Only after internal store behavior is tested, expose the smallest useful API and CLI hooks.

### Scope

- Decide package name and exported API shape intentionally (`sdk` or `akg`, per tracker).
- Keep public surface limited to Phase 1 behavior such as:
  - open/create/validate,
  - put/delete node/edge,
  - commit,
  - compact,
  - exact lookup and minimal list helpers.
- Do not expose raw WAL internals, mutable derived indexes, recovery-by-default, merge, query language, or public flush.
- Extend `cmd/akg` only for small validation/inspection/compaction commands.

### Acceptance criteria

- Public API review is documented before exports are added.
- API tests prove reads expose only current logical state.
- CLI validation fails clearly on corrupt files and succeeds on valid files.
- CLI inspection does not accidentally present tombstones or stale WAL records as current state.

## Out of Scope for Milestone 2

- Query engine or planner.
- General graph traversal language.
- Merge implementation.
- Persistent deletion log after compaction.
- Automatic salvage/recovery during ordinary open.
- Pi SDK harness or agent usability tests, except as future planning.
- Background services or multi-writer behavior.

## Recommended Implementation Order

1. `internal/state` data structures, identity types, and mutation tests.
2. `internal/store` materialization to sorted Data entries and derived key tests.
3. Hydration from Data entries back into state.
4. File create/open/validate using existing format/WAL decoders.
5. Commit/reopen tests for WAL durability behavior.
6. Explicit compaction tests.
7. Minimal API/CLI review and implementation.

## Milestone 2 Definition of Done

Milestone 2 is complete when:

- all tasks above have passing tests or documented deferrals,
- `go test ./...` passes,
- ordinary open applies committed WAL and ignores trailing uncommitted WAL,
- commit leaves WAL in place until compaction,
- compaction rewrites only live state and discards old WAL,
- reads expose only current logical nodes/edges,
- derived keys and Bloom are regenerated from live state,
- no broad public SDK/API is introduced accidentally.
