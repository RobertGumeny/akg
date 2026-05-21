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

- [ ] Re-read `docs/TASKS.md`.
- [ ] Re-read `docs/spec/01-data-model.md` through `docs/spec/07-error-handling.md`.
- [ ] Confirm Milestone 1 binary/container tests still pass unchanged.
- [ ] Decide whether public API is needed for the current task; prefer internal implementation until behavior is locked.
- [ ] Confirm no query engine, merge behavior, recovery-by-default, or public flush API is introduced.
- [ ] Confirm any API exported in Milestone 2 is intentionally reviewed and minimal.

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

- [ ] Live nodes produce `n:{type}:{id}` entries with node payload values.
- [ ] Live edges produce `e:{from}:{relation}:{to}` entries with edge payload values.
- [ ] Live edges produce `ei:{to}:{relation}:{from}` empty-value entries.
- [ ] Node tags produce `t:{tag}:{node_id}` empty-value entries.
- [ ] Nodes produce `ts:{updated_at}:n:{type}:{id}` empty-value entries.
- [ ] Edges produce `ts:{updated_at}:e:{from}:{relation}:{to}` empty-value entries.
- [ ] Materialized entries are sorted by raw bytewise key order.
- [ ] Duplicate materialized keys are rejected.
- [ ] Deleted/superseded records do not appear in materialized output.
- [ ] Unknown MessagePack fields are dropped on rewrite.

## Level 4 — Hydration/open/replay validation

### Data hydration

- [ ] Data primary node entries decode into authoritative state.
- [ ] Data primary edge entries decode into authoritative state.
- [ ] Node key/payload consistency is validated where applicable.
- [ ] Edge key/payload identity mismatch is rejected.
- [ ] Missing or malformed primary payloads are rejected.
- [ ] Required derived index omissions/inconsistencies are either rejected or explicitly documented as deferred.
- [ ] `materialize -> hydrate -> materialize` preserves logical state.

### Ordinary open

- [ ] Clean compacted file opens to current logical state.
- [ ] File with committed WAL applies mutations through the last valid `COMMIT`.
- [ ] File with no valid `COMMIT` applies no WAL mutations.
- [ ] Trailing uncommitted WAL records are ignored.
- [ ] Trailing malformed WAL bytes after the last valid `COMMIT` are ignored as uncommitted tail only if the WAL helper semantics permit it.
- [ ] Malformed committed WAL causes open failure.
- [ ] Bad container/header/section checksum causes open failure.
- [ ] Malformed known sections cause open failure.
- [ ] Unknown structurally valid sections remain tolerated.
- [ ] Next WAL sequence number after reopen is greater than every existing record sequence.
- [ ] WAL entry/byte counters are recomputed or persisted accurately enough for threshold checks.

## Level 5 — Commit and compaction validation

### Commit

- [ ] Commit appends mutation WAL records.
- [ ] Commit appends a `COMMIT` record with empty payload.
- [ ] Commit fsyncs the file state required for durable recovery.
- [ ] Committed mutation survives close/reopen.
- [ ] Uncommitted mutation is not applied after reopen.
- [ ] Multiple committed batches replay in sequence order.
- [ ] WAL sequence numbers are monotonic and never reused across sessions.
- [ ] Committed WAL remains present until compaction.
- [ ] Clean close commits outstanding mutations unless already committed.
- [ ] Internal threshold detection fires at `>= 1,000` WAL entries.
- [ ] Internal threshold detection fires at `>= 10 MB` WAL data.
- [ ] No public flush API is exposed.

### Compaction

- [ ] Compaction performs ordinary open semantics before rewriting.
- [ ] Compaction writes only current live nodes, edges, and derived index keys.
- [ ] Compaction drops tombstones and superseded records.
- [ ] Compaction rebuilds Bloom from the live key set.
- [ ] Compaction discards prior WAL.
- [ ] Compacted file validates and reopens without WAL replay dependency.
- [ ] Compaction preserves logical graph state.
- [ ] Atomic rename replacement path is tested where practical.

## Level 6 — Fixture/conformance expansion

Add or update fixtures under `testdata/conformance/` for:

- [ ] empty graph created by Milestone 2 create path;
- [ ] minimal node;
- [ ] fully populated node;
- [ ] single edge;
- [ ] small realistic graph with tags and edges;
- [ ] file with committed WAL requiring ordinary-open replay;
- [ ] file with trailing uncommitted WAL ignored on open;
- [ ] compacted file with no carried-forward WAL;
- [ ] file involving logical deletes before compaction;
- [ ] rejection fixture for malformed committed WAL;
- [ ] rejection fixture for invalid Data/derived-key consistency if enforced in Milestone 2.

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

- [ ] `go test ./...` passes.
- [ ] State mutation semantics are complete and tested.
- [ ] Materialize/hydrate round trips preserve logical state.
- [ ] Ordinary open applies committed WAL and ignores trailing uncommitted WAL.
- [ ] Commit durability behavior is tested across reopen.
- [ ] Compaction preserves logical state and discards old WAL.
- [ ] Fixtures cover committed WAL replay and compaction.
- [ ] No accidental broad public SDK/API design has been introduced.
