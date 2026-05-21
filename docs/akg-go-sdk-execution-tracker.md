# AKG Go SDK — Execution Tracker

Purpose: durable working tracker for short, handoff-driven implementation sessions after the spec reconciliation pass. This is the operational source of truth for execution status. Keep `docs/akg-reference-implementation-plan.md` as the higher-level reference.

## Current Implementation Target

Phase 1 Go reference SDK for the AKG format layer:
- create/open/validate `.akg` files
- apply `PutNode`, `PutEdge`, `DeleteNode`, `DeleteEdge`
- perform ordinary open by validating the file, loading Data, replaying committed WAL through the last valid `COMMIT`, and ignoring trailing uncommitted WAL
- expose current logical state only
- compact files by rewriting live state into a fresh file, rebuilding Bloom, and discarding prior WAL
- build conformance fixtures alongside implementation

## Reconciled Decisions

- **Write model:** conventional accumulating WAL between compactions; not rewrite-on-every-commit.
- **Commit boundary:** committed state is all WAL records through the last valid `COMMIT`.
- **Ordinary open:** validate the container strictly, read the Data section, replay committed WAL, and ignore trailing uncommitted WAL.
- **Corruption policy:** ordinary open is fail-closed; malformed known sections, checksum failures, invalid committed WAL, and section-table violations are rejected. Salvage/recovery is explicit and separate.
- **Compaction:** explicit whole-file rewrite from current logical state; discard prior WAL; rebuild bloom filter from the live key set.
- **Data section model:** repeated flat KV entries with `key_len:uint32`, `value_len:uint32`, little-endian fixed-width integers, no padding, empty values allowed, duplicate keys invalid, sorted by raw UTF-8 bytewise lexicographic order.
- **Bloom section:** implement the deterministic wire format and fixed v1 parameters from the propagated spec.
- **Temporal keys:** canonical self-describing forms are `ts:{timestamp}:n:{type}:{id}` and `ts:{timestamp}:e:{from}:{relation}:{to}`.
- **WAL delete payloads:** `DELETE_NODE` and `DELETE_EDGE` use explicit MessagePack map identities with required fields; unknown extras are tolerated on read.
- **Section rules:** exactly one Data section required; at most one Bloom section; at most one WAL section; zero-length Data/Bloom invalid; zero-length WAL allowed; unknown section types skipped if structurally valid.
- **Architecture rule:** authoritative mutable state is nodes + edges only; `ei:`, `t:`, `ts:`, and Bloom are derived.
- **Scope control:** implement format-layer behavior first; no query engine, traversal layer, merge engine, or automatic recovery in this phase.

## Chosen Architecture

- `internal/record` — node/edge structs, validation, MessagePack encode/decode, write-time required-field enforcement, read-time defaults
- `internal/keys` — canonical key builders/parsers for `n:`, `e:`, `ei:`, `t:`, and `ts:`
- `internal/format` — header, section table, checksum handling, section cardinality validation, Data-section entry encoding/decoding, Bloom-section wire format
- `internal/wal` — WAL record encode/decode, append, replay through last valid `COMMIT`, committed-tail detection, sequence tracking, committed WAL size/count accounting
- `internal/state` — authoritative logical nodes + edges only, mutation application, delete semantics, derivation of secondary keys at materialization time
- `internal/store` — sorted KV assembly/helpers, duplicate-key checks, live key materialization, bloom construction helpers
- `sdk` or `akg` — small public Go API surface for Phase 1 (`Commit()`, `Compact()`, no public flush API)
- `cmd/akg` — tiny validation/inspection/compaction CLI once core format pieces exist
- `testdata/conformance` — golden files and fixtures

## Repo Layout

Current repo state:
- `docs/spec/` — normative spec
- `docs/akg-go-sdk-execution-tracker.md` — working execution tracker
- `docs/akg-reference-implementation-plan.md` — high-level implementation reference
- `docs/akg-comprehensive-test-plan.md` — test planning reference
- `docs/archive/` — decision/history/archive material retained for reference

Planned implementation additions:
- `go.mod`
- `internal/record/`
- `internal/keys/`
- `internal/format/`
- `internal/wal/`
- `internal/state/`
- `internal/store/`
- `cmd/akg/`
- `testdata/conformance/`

## Current Milestone

Milestone 2: build the first logical store layer on top of the completed Milestone 1 binary/container primitives.

Definition of done for this milestone:
- authoritative in-memory state is implemented as nodes + edges only
- `PutNode`, `PutEdge`, `DeleteNode`, and `DeleteEdge` semantics are tested, including strict delete-not-found behavior
- live state materializes into sorted Data entries with derived `ei:`, `t:`, `ts:`, and Bloom output
- Data entries hydrate back into authoritative state with validation of primary payloads and key identity
- ordinary open validates the container, loads Data, replays committed WAL through the last valid `COMMIT`, and ignores trailing uncommitted WAL
- ordinary commit appends WAL mutation records plus `COMMIT`, fsyncs, and leaves committed WAL in place until compaction
- explicit compaction rewrites only live state, rebuilds derived keys/Bloom, discards prior WAL, and atomically replaces the file
- any public API or CLI surface is intentionally minimal and reviewed before export

## Next 3–5 Tasks

1. Implement file create/open/validate using existing format and WAL primitives, including committed-WAL replay, ignored uncommitted tails, next-sequence tracking, and WAL threshold counters.
2. Implement ordinary commit and WAL lifecycle behavior, including threshold detection.
3. Implement explicit compaction using whole-file rewrite from live state.
4. Perform the minimal public API/CLI review before adding exported surface.
5. Add fixture-backed conformance files once create/open/compact behavior exists.

## Test / Conformance Checkpoints

The implementation is not ready to advance unless fixture-backed tests cover these locked semantics:

- header, section table, checksums, and all fixed-width integers as little-endian
- section cardinality/range validation, including zero-length Data/Bloom rejection and zero-length WAL acceptance
- Data-section entry wire format, duplicate-key rejection, no-padding layout, and bytewise UTF-8 sort order
- deterministic Bloom wire format and fixed v1 parameters
- canonical key emission for `n:`, `e:`, `ei:`, `t:`, and self-describing `ts:` keys
- WAL record encoding/decoding, explicit delete payload maps, and unknown-extra tolerance on delete payload reads
- replay through the last valid `COMMIT`
- ordinary open ignoring trailing uncommitted WAL but rejecting malformed committed WAL/corruption
- compaction rewriting live state only, rebuilding derived keys/Bloom, and discarding prior WAL
- threshold-triggered auto-compaction using the same rewrite path as explicit `Compact()`
- read/rewrite/read logical equivalence with unknown MessagePack fields dropped on rewrite where allowed

## Flush / Auto-Compaction Design Note

Spec WAL "flush" will be implemented as an internal automatic compaction-style rewrite, not as a third persistence mode.

Design decision:
- keep public API limited to `Commit()` and `Compact()`
- do not expose a public flush method
- after each successful committed batch, check WAL thresholds
- when threshold reached (`>= 1,000` WAL entries or `>= 10 MB` WAL data), run `autoCompactIfNeeded()`
- `autoCompactIfNeeded()` rewrites current logical state into a fresh AKG file, rebuilds Bloom, discards prior WAL, atomically renames into place, and resets WAL counters
- explicit `Compact()` and threshold-triggered auto-compaction should share the same rewrite implementation

Key invariant:
- only two durable file shapes should exist: Data/Bloom plus accumulated committed WAL, or a freshly rewritten compacted file with no carried-forward WAL

## Blockers / Unresolved Items

- The main reconciliation blockers are resolved; implementation should now track the propagated spec/docs, not the old checklist.
- Before writing fixtures, verify implementation assumptions directly against the normative spec text for:
  - Data-section wire format and ordering
  - little-endian fixed-width integer handling
  - Bloom wire format and fixed parameters
  - section cardinality/validation rules
  - canonical temporal-key shape
  - WAL delete payload map shapes
  - ordinary-open failure vs trailing-uncommitted-tail tolerance
- Remaining implementation detail: decide exactly where WAL counters live and how they are recomputed on open so post-reopen threshold checks remain correct.
- Merge/persistent deletion-log design remains Phase 2 and is out of scope for current sessions.

## Recent Completions

- Completed Milestone 2 Task 3: `internal/store` now hydrates decoded Data entries into authoritative `internal/state`, loads primary `n:`/`e:` records without writer-owned timestamp/version mutation, validates key/payload identity, rejects malformed primary payloads, validates required `ei:`, `t:`, and `ts:` indexes by regenerating materialized keys, rejects non-empty derived values and unknown Data keys, and drops unknown MessagePack fields after read/rewrite. `go test ./...` passes.
- Completed Milestone 2 Task 2: `internal/store` now materializes authoritative `internal/state` live nodes and edges into sorted AKG Data entries, regenerating `n:`, `e:`, `ei:`, `t:`, and self-describing `ts:` keys, using empty values for derived indexes, rejecting duplicate materialized keys, and re-encoding node/edge payloads through canonical record encoders. Focused tests cover determinism, sorted output, Data-section decoder acceptance, empty derived values, duplicate derived-key rejection, and omitted deleted/superseded records. `go test ./...` passes.
- Updated `docs/spec/01-data-model.md` and `docs/spec/04-key-layout.md` to lock the Task 1 decision that node identity is `(type,id)`, node IDs are unique within a type key space rather than globally, and changing type is identity change.
- Completed Milestone 2 Task 1: `internal/state` now holds authoritative live nodes and edges only, supports `PutNode`, `PutEdge`, `DeleteNode`, and `DeleteEdge`, generates 16-character lowercase hex node IDs, enforces key/tag validation, writer-owned timestamps/versions, strict delete-not-found behavior, dangling-edge tolerance, and `(type,id)` identity semantics. `go test ./...` passes.
- Archived completed Milestone 1 planning docs to `docs/archive/milestone-1-validation-2026-05-20.md` and `docs/archive/milestone-1-tasks-2026-05-20.md`.
- Reset active `docs/TASKS.md` and `docs/VALIDATION.md` for Milestone 2 planning.
- Completed Milestone 1 validation through Level 5: corruption tests, round-trip/property tests, fixture-backed whole-container round trip, and decoder fuzz target compilation.
- Confirmed the Before Milestone 2 gate: `go test ./...` passed, no active M1 binary-format spec deviations are known, and no accidental public SDK/API design was introduced.
- Chose the workflow: spec reconciliation first, then execution tracker, then scaffolding, then small-session implementation.
- Decided not to use `docs/akg-reference-implementation-plan.md` as the working tracker.
- Identified the write-model contradiction as the key blocker before coding.
- Completed the spec propagation pass into the main spec/reference docs.
- Confirmed the propagated docs now lock in accumulating-WAL semantics, strict ordinary open, deterministic Bloom wire format, explicit Data entry format, explicit section rules, and self-describing temporal keys.
- Updated this execution tracker to follow the propagated spec as the implementation source of truth.

## Working Conventions For Short Sessions

- Keep sessions small and implementation-scoped.
- Handoff at roughly 30–35% context usage.
- Update this file at the end of every session:
  - move finished tasks to Recent Completions
  - refresh Current Milestone
  - rewrite Next 3–5 Tasks
  - leave a crisp Handoff Seed
- Prefer fixture-backed progress over broad refactors.
