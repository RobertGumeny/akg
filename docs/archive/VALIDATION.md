# AKG Milestone 3 Validation Plan

Archived validation plans:

- Milestone 1: `docs/archive/milestone-1-validation-2026-05-20.md`
- Milestone 2: `docs/archive/milestone-2-validation-2026-05-20.md`

This document tracks validation for Milestone 3: AKG v1 hardening, conformance, release-candidate preparation, and repository/docs boundary clarification.

## Purpose

Milestone 3 should prove that AKG core is ready to serve as an open-source format home:

- the v1 spec is auditable against implementation and fixtures;
- the conformance corpus is machine-readable and useful to other implementations;
- valid and invalid fixtures exercise release-critical behavior;
- the Go package remains a canonical minimal reference implementation;
- public API and docs are stable enough for a v1 release candidate;
- SDK/product concepts such as memory-file ingestion stay above AKG core.

## Level 1 — Normal test suite

Run after every change:

```bash
go test ./...
```

Before marking Milestone 3 complete, run:

```bash
go test -count=1 ./...
```

Expected result: all packages pass.

## Level 2 — Milestone 3 scope guard

Before implementing any new behavior:

- [x] Re-read `docs/TASKS.md`.
- [x] Re-read `docs/API.md`.
- [x] Re-read the relevant v1 spec files: `docs/spec/01-data-model.md`, `docs/spec/02-binary-layout.md`, `docs/spec/03-encoding.md`, `docs/spec/04-key-layout.md`, `docs/spec/05-wal.md`, `docs/spec/06-compaction.md`, and `docs/spec/07-error-handling.md`.
- [x] Confirm the change belongs in AKG core, not in a future SDK/example layer.
- [x] Confirm no memory-file ingestion behavior is being added to core.
- [x] Confirm no query engine, traversal language, merge system, recovery-by-default, public flush, background service, or multi-writer behavior is introduced.
- [x] Confirm any public API change is intentional and documented.

## Level 3 — Conformance manifest validation

- [x] A machine-readable manifest exists under `testdata/conformance/`.
- [x] Every `testdata/conformance/*.akg` fixture appears exactly once in the manifest.
- [x] Every manifest fixture path exists.
- [x] Each manifest entry includes purpose/description.
- [x] Each manifest entry declares expected result: accept or reject.
- [x] Rejection entries include a stable error category or failure class.
- [x] Go conformance tests consume the manifest or check sync between manifest and test cases.
- [x] `testdata/conformance/README.md` documents the manifest schema and runner expectations.
- [x] The manifest is usable by non-Go implementations without reading Go test source.

## Level 4 — Fixture generation and corpus reproducibility

- [x] Fixture generation/update workflow is documented.
- [x] Generated valid fixtures are deterministic across repeated local runs where practical.
- [x] Hand-corrupted fixtures are labeled as such.
- [x] Corruption method for each rejection fixture is documented or encoded in a generator/helper.
- [x] The workflow protects against accidental silent fixture changes.
- [x] `testdata/conformance/README.md` explains when and how to update fixture bytes.

## Level 5 — Rejection fixture coverage

Add or verify manifest-backed rejection fixtures for:

- [x] Wrong magic bytes.
- [x] Unsupported major version.
- [x] Bad header checksum.
- [x] Bad section checksum.
- [x] Duplicate sections where v1 requires uniqueness.
- [x] Overlapping sections or invalid section ranges.
- [x] Malformed Bloom section.
- [x] Invalid WAL opcode.
- [x] Invalid WAL payload for `PUT_NODE`.
- [x] Invalid WAL payload for `PUT_EDGE`.
- [x] Invalid WAL payload for `DELETE_NODE`.
- [x] Invalid WAL payload for `DELETE_EDGE`.
- [x] Malformed committed WAL that ordinary open must reject.
- [x] Invalid Data/derived-key consistency failure.

For each rejection fixture:

- [x] Public `Validate` or ordinary `Open` rejects it.
- [x] The rejection is represented in the manifest.
- [x] Exact error strings are not required for conformance, but the failure category is stable enough to document.

## Level 6 — Existing accept fixture coverage

Verify the current accept fixtures remain present and manifest-backed:

- [x] Empty graph created by the store create path.
- [x] Minimal node.
- [x] Fully populated node.
- [x] Single edge.
- [x] Small realistic graph with tags and edges.
- [x] File with committed WAL requiring ordinary-open replay.
- [x] File with trailing uncommitted WAL ignored on open.
- [x] Compacted file with no carried-forward WAL.
- [x] File involving logical deletes before compaction.
- [x] File with structurally valid unknown section tolerated by store-level open/validate, if retained as v1 behavior.

## Level 7 — Spec requirements audit

- [x] Create/update a traceability document, for example `docs/spec/v1-requirements-audit.md`.
- [x] Audit `docs/spec/01-data-model.md` normative requirements.
- [x] Audit `docs/spec/02-binary-layout.md` normative requirements.
- [x] Audit `docs/spec/03-encoding.md` normative requirements.
- [x] Audit `docs/spec/04-key-layout.md` normative requirements.
- [x] Audit `docs/spec/05-wal.md` normative requirements.
- [x] Audit `docs/spec/06-compaction.md` normative requirements.
- [x] Audit `docs/spec/07-error-handling.md` normative requirements.
- [x] Every v1 `MUST`/`MUST NOT` maps to implementation, tests, fixtures, docs-only rationale, or a release-blocking gap.
- [x] Every v1 `SHOULD`/`SHOULD NOT` maps to implementation, tests, fixtures, docs-only rationale, or an explicit decision.
- [x] Any ambiguous spec wording discovered during audit is clarified before v1 RC.

## Level 8 — Public API/read-helper validation

- [x] `docs/API.md` documents the v1 public read-helper stance.
- [x] Exact lookup/list helper policy is explicitly accepted or changed.
- [x] Any tag/outbound/inbound helper decision is documented with rationale.
- [x] No query engine, planner, traversal language, or broad SDK helper surface is exported.
- [x] Exported identifiers in `akg.go` are audited for v1 necessity.
- [x] Public API tests cover create/open/validate.
- [x] Public API tests cover put/delete node and edge mutations.
- [x] Public API tests cover commit/close behavior.
- [x] Public API tests cover compaction.
- [x] Public API tests cover the allowed read helpers.
- [x] Public API does not expose raw WAL internals or mutable derived indexes.

## Level 9 — Dogfood lifecycle validation

- [x] A tiny lifecycle example or walkthrough exists.
- [x] The example creates an AKG file.
- [x] The example adds realistic nodes, edges, and tags.
- [x] The example commits changes.
- [x] The example reopens the file and reads records through the public API.
- [x] The example compacts the file.
- [x] The example validates the final file.
- [x] The example is runnable or followable from a clean checkout.
- [x] The example does not implement memory-file ingestion, agent workflow behavior, or product SDK logic.
- [x] Any public API/docs usability findings from dogfooding are resolved or explicitly tracked.

## Level 10 — Release-quality documentation validation

- [x] Docs explain what AKG is.
- [x] Docs explain what AKG is not.
- [x] Docs include a lifecycle example: create, mutate, commit, reopen, compact, validate.
- [x] Docs explain conformance corpus usage for alternative implementations.
- [x] Docs include SDK author guidance.
- [x] Docs explain that memory-file ingestion belongs in SDKs/examples, not AKG core.
- [x] Docs describe repository layer boundaries: spec, conformance, reference implementation, SDKs, examples.
- [x] Docs describe the reference implementation as canonical/minimal, not as a required dependency for downstream SDKs.
- [x] Primary docs link to spec, API docs, conformance README, and repository boundary guidance.

## Suggested Agent Workflow

When asking an agent to execute Milestone 3 work, use a request like:

> Continue from `docs/TASKS.md` and `docs/VALIDATION.md`. Implement the next Milestone 3 task only. Keep AKG core focused on v1 hardening/conformance/release-candidate prep, avoid memory ingestion or SDK product scope, and run `gofmt` plus `go test -count=1 ./...` when relevant.

Recommended sequence:

1. Read `docs/TASKS.md`, this file, and the relevant spec/API/conformance docs.
2. Add or update tests/fixtures/docs for one task at a time.
3. Keep public API changes rare and documented before expanding exports.
4. Run `gofmt` on changed Go files.
5. Run `go test ./...` during iteration and `go test -count=1 ./...` before completion.
6. Update checklist items only when directly covered by tests, docs, fixtures, or an explicit decision note.

## Milestone 3 completion checklist

Milestone 3 should not be considered complete until:

- [x] `go test -count=1 ./...` passes.
- [x] Conformance manifest exists and is checked by tests.
- [x] Fixture generation/update workflow is documented.
- [x] Required rejection fixtures are present and manifest-backed.
- [x] Spec requirements audit is complete.
- [x] Public API/read-helper stance is documented and implemented.
- [x] A tiny lifecycle example dogfoods the public API without adding SDK/product scope.
- [x] Release-quality docs clarify AKG core, conformance, lifecycle, SDK author guidance, and repo boundaries.
- [x] No memory ingestion or product SDK scope has entered AKG core.
