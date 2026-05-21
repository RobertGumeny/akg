# AKG Milestone 2 Validation Plan

Archived Milestone 1 validation plan: `docs/archive/milestone-1-validation-2026-05-20.md`

This document tracks validation for Milestone 2: logical state, store/open/commit, WAL lifecycle behavior, and explicit compaction built on the completed Milestone 1 binary/container layer.

## Purpose

Milestone 2 should prove that AKG files can be used as a current-state graph store while preserving the v1 format rules:

- authoritative mutable state is nodes + edges only;
- derived keys and Bloom are regenerated from live state;
- ordinary open validates strictly, applies committed WAL through the last valid `COMMIT`, and ignores trailing uncommitted WAL;
- ordinary commit appends WAL records and leaves committed WAL in place until compaction;
- compaction rewrites live state only and discards prior WAL.

## Level 1 — Normal test suite

Run after every change:

```bash
go test ./...
```

Expected result: all packages pass.

## Level 2 — Milestone 2 design/API audit

Before adding exported API surface or large store behavior:

- [x] Re-read `docs/TASKS.md`.
- [x] Re-read `docs/spec/01-data-model.md` through `docs/spec/07-error-handling.md`.
- [x] Confirm Milestone 1 binary/container tests still pass unchanged.
- [x] Decide whether public API is needed for the current task; prefer internal implementation until behavior is locked.
- [x] Confirm no query engine, merge behavior, recovery-by-default, or public flush API is introduced.
- [x] Confirm any API exported in Milestone 2 is intentionally reviewed and minimal.

## Level 3 — State and materialization validation

### Authoritative state

- [x] Node upsert by `(type, id)` creates and updates the same identity.
- [x] Node type change is identity change, not in-place mutation.
- [x] Edge upsert by `(from_node, relation, to_node)` creates and updates the same identity.
- [x] Delete existing node succeeds.
- [x] Delete missing node returns not-found.
- [x] Delete existing edge succeeds.
- [x] Delete missing edge returns not-found.
- [x] Dangling edges are accepted.
- [x] Node version increments on mutation.
- [x] Edge version increments on mutation.
- [x] Writer-owned timestamps are set/updated consistently.
- [x] Generated node IDs are 16 lowercase hex characters.
- [x] Invalid caller-provided node IDs are rejected.

### Tags and key constraints

- [x] Duplicate tags are rejected.
- [x] More than 32 tags are rejected.
- [x] Uppercase tags are rejected.
- [x] Tags with spaces are rejected.
- [x] Malformed key components are rejected before write.

### Materialization

- [x] Live nodes produce `n:{type}:{id}` entries with node payload values.
- [x] Live edges produce `e:{from}:{relation}:{to}` entries with edge payload values.
- [x] Live edges produce `ei:{to}:{relation}:{from}` empty-value entries.
- [x] Node tags produce `t:{tag}:{node_id}` empty-value entries.
- [x] Nodes produce `ts:{updated_at}:n:{type}:{id}` empty-value entries.
- [x] Edges produce `ts:{updated_at}:e:{from}:{relation}:{to}` empty-value entries.
- [x] Materialized entries are sorted by raw bytewise key order.
- [x] Duplicate materialized keys are rejected.
- [x] Deleted/superseded records do not appear in materialized output.
- [x] Unknown MessagePack fields are dropped on rewrite.

## Level 4 — Hydration/open/replay validation

### Data hydration

- [x] Data primary node entries decode into authoritative state.
- [x] Data primary edge entries decode into authoritative state.
- [x] Node key/payload consistency is validated where applicable.
- [x] Edge key/payload identity mismatch is rejected.
- [x] Missing or malformed primary payloads are rejected.
- [x] Required derived index omissions/inconsistencies are rejected.
- [x] `materialize -> hydrate -> materialize` preserves logical state.
- [x] Unknown Data keys are rejected in Task 3 rather than ignored.

### Ordinary open

- [x] Clean compacted file opens to current logical state.
- [x] File with committed WAL applies mutations through the last valid `COMMIT`.
- [x] File with no valid `COMMIT` applies no WAL mutations.
- [x] Trailing uncommitted WAL records are ignored.
- [x] Trailing malformed WAL bytes after the last valid `COMMIT` are ignored as uncommitted tail only if the WAL helper semantics permit it.
- [x] Malformed committed WAL causes open failure.
- [x] Bad container/header/section checksum causes open failure.
- [x] Malformed known sections cause open failure.
- [x] Unknown structurally valid sections remain tolerated.
- [x] Next WAL sequence number after reopen is greater than every existing record sequence.
- [x] WAL entry/byte counters are recomputed or persisted accurately enough for threshold checks.

## Level 5 — Commit and compaction validation

### Commit

- [x] Commit appends mutation WAL records.
- [x] Commit appends a `COMMIT` record with empty payload.
- [x] Commit fsyncs the file state required for durable recovery.
- [x] Committed mutation survives close/reopen.
- [x] Uncommitted mutation is not applied after reopen.
- [x] Multiple committed batches replay in sequence order.
- [x] WAL sequence numbers are monotonic and never reused across sessions.
- [x] Committed WAL remains present until compaction.
- [x] Clean close commits outstanding mutations unless already committed.
- [x] Internal threshold detection fires at `>= 1,000` WAL entries.
- [x] Internal threshold detection fires at `>= 10 MB` WAL data.
- [x] No public flush API is exposed.

### Public API and CLI

- [x] Public API review is documented in `docs/API.md` before exports are added.
- [x] Root `akg` package exposes only minimal Phase 1 create/open/validate, mutation, lookup/list, commit/close, and compaction APIs.
- [x] Public API reads expose only current logical nodes and edges.
- [x] Public API does not expose raw WAL internals, mutable derived indexes, recovery-by-default, merge, query language, or public flush.
- [x] CLI `validate` succeeds on valid files and fails clearly on corrupt files.
- [x] CLI `inspect` opens through ordinary store semantics and shows only current logical state.
- [x] CLI `compact` runs explicit compaction and leaves a valid file.

### Compaction

- [x] Compaction performs ordinary open semantics before rewriting.
- [x] Compaction writes only current live nodes, edges, and derived index keys.
- [x] Compaction drops tombstones and superseded records.
- [x] Compaction rebuilds Bloom from the live key set.
- [x] Compaction discards prior WAL.
- [x] Compacted file validates and reopens without WAL replay dependency.
- [x] Compaction preserves logical graph state.
- [x] Atomic rename replacement path is tested where practical.

Crash-safety note: explicit compaction writes a same-directory temporary file,
fsyncs it, renames it over the original path, then fsyncs the directory. A crash
should leave either the old file or the new compacted file at the target path;
at most a removable `.compact-*` temporary file may remain.

## Level 6 — Fixture/conformance expansion

Add or update fixtures under `testdata/conformance/` for:

- [x] empty graph created by Milestone 2 create path;
- [x] minimal node;
- [x] fully populated node;
- [x] single edge;
- [x] small realistic graph with tags and edges;
- [x] file with committed WAL requiring ordinary-open replay;
- [x] file with trailing uncommitted WAL ignored on open;
- [x] compacted file with no carried-forward WAL;
- [x] file involving logical deletes before compaction;
- [x] rejection fixture for malformed committed WAL;
- [x] rejection fixture for invalid Data/derived-key consistency if enforced in Milestone 2.

## Suggested Agent Workflow

When asking an agent to execute Milestone 2 work, use a request like:

> Continue from `docs/TASKS.md` and `docs/VALIDATION.md`. Implement the next Milestone 2 task only. Keep authoritative state as nodes + edges, derive all index keys at materialization, avoid broad public API changes, and run `gofmt` plus `go test ./...`.

Recommended sequence:

1. Read `docs/TASKS.md`, this file, and the relevant spec sections.
2. Inspect existing internal package tests before coding.
3. Add focused tests for one task.
4. Implement the minimal code to satisfy those tests.
5. Run `gofmt` on changed Go files.
6. Run `go test ./...`.
7. Update checklist items only when directly covered by tests or an explicit deferral note.

## Before Milestone 3

Milestone 3 should not start until:

- [x] `go test ./...` passes.
- [x] State mutation semantics are complete and tested.
- [x] Materialize/hydrate round trips preserve logical state.
- [x] Ordinary open applies committed WAL and ignores trailing uncommitted WAL.
- [x] Commit durability behavior is tested across reopen.
- [x] Compaction preserves logical state and discards old WAL.
- [x] Fixtures cover committed WAL replay and compaction.
- [x] No accidental broad public SDK/API design has been introduced.
