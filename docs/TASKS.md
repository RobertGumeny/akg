# Milestone 3 Tasks

Source documents:

- `docs/spec/01-data-model.md` through `docs/spec/07-error-handling.md`
- `docs/API.md`
- `docs/akg-reference-implementation-plan.md`
- `docs/akg-comprehensive-test-plan.md`
- `docs/akg-go-sdk-execution-tracker.md`
- `testdata/conformance/README.md`

Archived milestone plans:

- Milestone 1 tasks: `docs/archive/milestone-1-tasks-2026-05-20.md`
- Milestone 1 validation: `docs/archive/milestone-1-validation-2026-05-20.md`
- Milestone 2 tasks: `docs/archive/milestone-2-tasks-2026-05-20.md`
- Milestone 2 validation: `docs/archive/milestone-2-validation-2026-05-20.md`

## Milestone 3 Goal

Milestone 3 hardens AKG v1 toward a release candidate. The repo should become the AKG core/open-source home, not merely a Go package: versioned spec, release-quality docs, reference implementation, conformance corpus, and clear boundaries for future SDKs/examples.

The Milestone 3 focus is **v1 hardening, conformance, and release-candidate preparation**. It is not memory-file ingestion and not a higher-level product SDK milestone.

## Strategic Scope Rules

- Keep AKG core focused on the file format, spec, reference implementation, and conformance corpus.
- Treat the Go implementation as a canonical minimal reference implementation and conformance oracle.
- Do not require downstream SDKs to import the reference implementation directly.
- Keep SDK/product concepts, including memory-file ingestion, above the format layer in future SDKs or examples.
- Do not add a query engine, planner, graph traversal language, merge system, background service, or multi-writer behavior.
- Freeze or minimize the public API in `akg.go`; prefer internal hardening over new exports.

## Task 1 — Add a machine-readable conformance manifest

Create a manifest for `testdata/conformance/` that describes every fixture and its expected behavior.

### Scope

- Add a manifest file, likely `testdata/conformance/manifest.json` or `manifest.yaml`.
- Include fixture path, purpose, expected result (`accept` or `reject`), and expected error category for rejection fixtures.
- Include enough metadata for other implementations to run the corpus without reading Go tests.
- Update `internal/store/conformance_test.go` to consume the manifest or verify that the manifest and test cases remain in sync.
- Update `testdata/conformance/README.md` with manifest format and usage.

### Acceptance criteria

- Every `testdata/conformance/*.akg` fixture is represented exactly once in the manifest.
- Accept/reject expectations are explicit and machine-readable.
- Go conformance tests fail if a fixture is missing from the manifest or the manifest references a missing fixture.
- The README explains how SDK authors and alternate implementations should use the manifest.

## Task 2 — Make fixture generation reproducible and documented

Ensure conformance fixtures can be regenerated or audited intentionally.

### Scope

- Identify current fixture generation path and document it.
- Add a small generator command, test helper, or documented `go test` flow if fixtures are generated from code.
- Make generated fixture bytes deterministic where practical.
- Document which fixtures are hand-corrupted and how they were corrupted.
- Provide a safe update workflow that prevents accidental silent fixture changes.

### Acceptance criteria

- A contributor can regenerate or audit fixtures from documented commands.
- Generated fixtures are stable across repeated local runs.
- Intentionally corrupt fixtures are clearly labeled and reproducible enough for review.
- Fixture update instructions live in `testdata/conformance/README.md` or an adjacent document.

## Task 3 — Expand rejection fixtures for v1 fail-closed behavior

Add rejection fixtures that exercise important format and validation failures.

### Required rejection cases

- Wrong magic bytes.
- Unsupported major version.
- Bad header checksum.
- Bad section checksum.
- Duplicate sections where v1 requires uniqueness.
- Overlapping sections or invalid section ranges.
- Malformed Bloom section.
- Invalid WAL opcode.
- Invalid WAL payloads for each mutation type where practical.
- Malformed committed WAL that must fail ordinary open.
- Invalid Data/derived-key consistency failures not already covered.

### Acceptance criteria

- Each rejection fixture has a manifest entry and README description.
- Tests assert rejection through the public validation/open path, not only through low-level decoders.
- Error categories are stable enough for conformance consumers, even if exact error strings remain implementation-specific.

## Task 4 — Audit v1 spec MUST/SHOULD requirements

Create a traceability audit from `docs/spec/01-data-model.md` through `docs/spec/07-error-handling.md` to implementation, tests, fixtures, or documented deferrals.

### Scope

- Extract every normative `MUST`, `MUST NOT`, `SHOULD`, and `SHOULD NOT` requirement.
- Map each requirement to one of:
  - implemented and tested,
  - covered by conformance fixture,
  - documentation-only requirement,
  - intentionally deferred or out of scope for v1 RC.
- Fix obvious gaps in implementation/tests/fixtures discovered by the audit.
- Clarify ambiguous spec language before adding broad behavior.

### Acceptance criteria

- A traceability document exists, for example `docs/spec/v1-requirements-audit.md`.
- No v1 `MUST` lacks either implementation/test coverage or an explicit release-blocking issue.
- Any changed spec wording remains compatible with existing valid fixtures or is called out clearly.

## Task 5 — Decide and document minimal read-helper stance

Clarify what the core reference API should expose for reads.

### Scope

- Review current exact lookup and list helpers in `akg.go`/`docs/API.md`.
- Decide whether Milestone 3 keeps exact/list only or adds small helpers for tags, outbound edges, and inbound edges.
- Avoid query engine, planner, traversal language, or SDK convenience sprawl.
- Document why any helper belongs in core rather than in an SDK/example layer.

### Acceptance criteria

- `docs/API.md` states the v1 public read-helper policy.
- `akg.go` exposes no accidental broad query surface.
- Any newly added helper has focused tests and maps directly to existing v1 derived keys/state.
- If no helpers are added, the decision is explicit and documented.

## Task 6 — Freeze and minimize public API for v1 RC

Prepare the root Go package for release-candidate stability.

### Scope

- Audit exported identifiers in `akg.go` and any public subpackages.
- Remove, unexport, or mark experimental anything not required for v1 core usage.
- Ensure exported errors/options are intentional and documented.
- Confirm public API does not expose WAL internals, mutable derived indexes, recovery-by-default, merge, query language, public flush, or product SDK concepts.

### Acceptance criteria

- `docs/API.md` matches the actual exported API.
- Public API tests cover create/open/validate, mutation, commit/close, compaction, and allowed reads.
- The release-candidate API boundary is clear enough for downstream SDK authors.

## Task 7 — Dogfood the v1 lifecycle with a tiny real example

Test the reference implementation as a user would, without turning Milestone 3 into SDK/product work.

### Scope

- Add or document a tiny lifecycle example that:
  - creates an AKG file,
  - adds a few realistic nodes, edges, and tags,
  - commits,
  - reopens,
  - reads records back through the public API,
  - compacts,
  - validates the result.
- Prefer an `examples/` program, a documented walkthrough, or a focused integration test that exercises only core primitives.
- Use the example as an API/docs usability probe: note confusing names, missing lifecycle docs, or overly broad/narrow helpers.
- Keep memory-file ingestion, agent workflows, and product SDK behavior out of this example.

### Acceptance criteria

- A contributor can run or follow the lifecycle example from a clean checkout.
- The example uses only the documented public API and CLI/core behavior.
- Any usability findings are fed back into `docs/API.md`, release docs, or narrowly scoped API cleanup.
- The example remains a core AKG lifecycle demonstration, not a memory ingestion prototype.

## Task 8 — Add release-quality core documentation

Make the repository understandable as the AKG core project.

### Scope

Add or update docs covering:

- What AKG is and is not.
- Format lifecycle example: create, mutate, commit, reopen, compact, validate.
- Conformance corpus usage for alternative implementations.
- SDK author guidance: what to build above core and what not to put in the file format.
- Embedding AKG in an SDK or product as an example architecture, without making ingestion part of core.
- Repository layer boundaries: spec vs conformance vs reference implementation vs SDKs vs examples.

### Acceptance criteria

- New contributors can identify the spec, conformance corpus, reference implementation, and API docs quickly.
- Docs explicitly state that memory-file ingestion belongs in SDKs/examples, not AKG core.
- Docs present the reference implementation as canonical/minimal, not as the only acceptable downstream architecture.

## Task 9 — Clarify repository structure for future growth

Prepare the repo layout and docs for future official/community SDKs and examples without implementing them prematurely.

### Scope

- Decide whether to add placeholder directories or just document future areas.
- Clarify naming and ownership expectations for possible future `sdks/`, `examples/`, or `docs/guides/` areas.
- Ensure current Go package remains clearly the reference implementation.
- Avoid adding product-specific SDK code in Milestone 3.

### Acceptance criteria

- Repository boundary documentation exists and is linked from primary docs.
- Future SDK/example areas are described without becoming release blockers.
- No memory ingestion or product harness code is added to AKG core.

## Out of Scope for Milestone 3

- Memory-file ingestion.
- Product SDK behavior or agent memory workflow design.
- Query engine, planner, traversal language, or general graph analytics.
- Merge implementation and conflict resolution.
- Automatic salvage/recovery during ordinary open.
- Background services, daemon behavior, or multi-writer support.
- Large API expansion beyond the minimal v1 core.

## Recommended Implementation Order

1. Add conformance manifest and manifest/test sync checks.
2. Document or implement reproducible fixture generation.
3. Add rejection fixtures and update conformance tests/README.
4. Perform the spec MUST/SHOULD traceability audit and close release-blocking gaps.
5. Review and freeze the public API/read-helper stance.
6. Dogfood the public API with a tiny lifecycle example and feed back usability findings.
7. Add release-quality repository, conformance, lifecycle, and SDK-author docs.
8. Run full validation and update `docs/VALIDATION.md` checkboxes only when directly covered.

## Milestone 3 Definition of Done

Milestone 3 is complete when:

- `go test -count=1 ./...` passes;
- conformance fixtures have a machine-readable manifest;
- fixture generation/update workflow is documented and reproducible enough for review;
- rejection fixtures cover the listed v1 failure classes;
- spec requirements are audited against implementation/tests/fixtures;
- the public API is intentionally minimal and documented for v1 RC;
- a tiny lifecycle example proves the public API/docs can be used as a real core format workflow;
- release-quality docs explain AKG core, conformance, lifecycle usage, SDK boundaries, and repo structure;
- no memory ingestion or product SDK scope has entered AKG core.
