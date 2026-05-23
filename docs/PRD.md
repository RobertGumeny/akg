# AKG — Execution PRD

## Objective

Ship AKG as a real, production-ready open-source project. Both SDKs are implemented, all 36 conformance fixtures pass, and the developer experience documentation is complete. The remaining work is everything that separates a working implementation from something you can announce: correct module paths, a license, CI, repo infrastructure, test coverage gaps, and a code quality audit.

---

## Context and reference material

| Source | What it contains |
|---|---|
| `docs/PRD.md` | This file. Objective, execution rules, per-task handoff notes. |
| `docs/backlog.md` | Full task list with acceptance criteria. Epics 5–8 = current work. |
| `docs/spec/` | Authoritative format specification. |
| `sdk/akg-go/` | Go SDK source and README. |
| `sdk/akg-ts/` | TypeScript SDK source and README. |
| `testdata/conformance/` | Conformance fixtures — the correctness contract for both SDKs. |

---

## Execution rules

Follow these throughout. They apply across all tasks.

1. **Execute tasks in order within each epic.** 5.1 → 5.2 → 5.3 before moving to Epic 6. Within Epic 8, complete the audits (8.1, 8.2) before fixes (8.3, 8.4) so findings from the audit can inform the fix tasks.

2. **Do not invent API behavior.** Read the SDK source before writing any test or snippet. Every code example must be runnable as written.

3. **Tests must be real.** No mocks of the file system or store internals. Every new test should use a real temp file and make real assertions about state — not just assert that nothing threw.

4. **Update handoff notes before moving to the next task.** Record decisions, surprises, and anything the next task needs to know.

---

## Per-task handoff notes

Agents: fill in your task's section before marking it done. Keep entries concise — decisions and surprises only.

---

### 5.1 Update Go SDK module path

_Status:_ done

_Decisions:_ New module path is `github.com/RobertGumeny/akg/sdk/akg-go`. Updated go.mod, examples/basic/main.go, sdk/akg-go/README.md, and root README.md.

_Notes for 5.2:_ No surprises. `go test ./...` passes after the rename.

---

### 5.2 Release prep

_Status:_ done

_Decisions:_ Apache 2.0 LICENSE added. Added `"files": ["dist", "README.md"]` to akg-ts/package.json.

_Notes for 5.3:_ npm pack --dry-run confirms only dist/ and README.md ship.

---

### 5.3 Docs cleanup

_Status:_ done

_Decisions:_ Deleted docs/akg-sdk-requirements.md, docs/repository-boundaries.md, docs/core.md, docs/API.md. Removed docs/core.md row from README repo contents table.

_Notes for Epic 6:_ docs/ now contains only spec/, conformance.md, sdk-author-guide.md, lifecycle.md (plus PRD.md and backlog.md which are current-work artifacts).

---

### 6.1 GitHub Actions CI workflow

_Status:_ done

_Decisions:_ Workflow file named `ci.yml`. Triggers on push and pull_request. Go job runs root tests, sdk/akg-go tests, and vet. TS job runs npm ci, test, build, tsc --noEmit from sdk/akg-ts/.

_Notes for 6.2:_ Workflow name is "CI" — use this name for the badge URL in 6.4.

---

### 6.2 Issue templates and PR template

_Status:_ done

_Decisions:_ Bug report and feature request templates in .github/ISSUE_TEMPLATE/. Minimal PR template at .github/PULL_REQUEST_TEMPLATE.md.

_Notes for 6.3:_ No surprises.

---

### 6.3 Repo root files

_Status:_ done

_Decisions:_ AGENTS.md describes full repo structure and what is authoritative. CLAUDE.md contains only `@AGENTS.md`. SECURITY.md directs to email. CHANGELOG.md seeded with v0.1.0 entry.

_Notes for 6.4:_ Workflow filename is `ci.yml`, workflow name is "CI". npm package name is `akg-ts`. Go module is `github.com/RobertGumeny/akg/sdk/akg-go`.

---

### 6.4 README badges

_Status:_ done

_Decisions:_ CI badge points to ci.yml workflow. Go Reference points to pkg.go.dev for new module path. npm badge for `akg-ts`. Apache 2.0 license badge points to LICENSE file.

_Notes for Epic 7:_ Epic 6 complete.

---

### 7.1 Go SDK test gaps

_Status:_ done

_Decisions:_ Had to fix errInvalidComponent and errTooManyTags/errDuplicateTags to wrap ErrInvalidInput (they were opaque errors). Added TestValidationErrorsErrInvalidInput, TestMissingRequiredFieldTitle, TestPutEdgeNonexistentNodes, TestPutNodeAutoID. 7.3 tests added in same file (TestNodeIDConstraints, TestTagArrayConstraints).

_Notes for 7.2:_ TS SDK already has most tests; missing: cross-type contamination for inboundEdges, deleteNode with only inbound edges, confidence field tests.

---

### 7.2 TypeScript SDK test gaps

_Status:_ done

_Decisions:_ Added cross-type contamination tests (outboundEdges and inboundEdges), deleteNode with only inbound edges → InvalidInputError, and three confidence field tests. 7.3 TS tests added in same block (node ID colon/length, tags count/duplicate).

_Notes for 7.3:_ Go 7.3 tests already added in 7.1. TS 7.3 tests already added in 7.2. Task 7.3 is complete.

---

### 7.3 Shared validation gaps (both SDKs)

_Status:_ done

_Decisions:_ Go tests added in 7.1 (TestNodeIDConstraints, TestTagArrayConstraints). TS tests added in 7.2 (node ID and tag constraints describe block). All pass.

_Notes for Epic 8:_ Both SDKs already had the constraint enforcement in place; only tests were missing.

---

### 8.1 Go SDK audit

_Status:_ done

_Decisions:_ No logic bugs found. The errInvalidComponent/errTooManyTags/errDuplicateTags sentinel wrapping (fixed in 7.1) was the main quality issue. Data hydration and WAL replay paths use fmt.Errorf multi-wrap correctly. dir.Sync() discards are intentional. Unreachable decode paths in WAL replay (after validateWALPayload) are not a concern.

_Notes for 8.2:_ TS SDK has two bugs: (1) WAL validateWALPayload errors converted from MissingRequiredFieldError to InvalidInputError — loses the sentinel; (2) decodeEdgeFromMap default strength is 0.5 (should be 0.0).

---

### 8.2 TypeScript SDK audit

_Status:_ done

_Decisions:_ Fixed two bugs found in audit: (1) WAL error wrapping now preserves MissingRequiredFieldError class through the chain; (2) default strength in decodeEdgeFromMap changed from 0.5 to 0.0.

_Notes for 8.3:_ Go SDK data hydration path already uses fmt.Errorf multi-wrap for ErrMissingRequiredField — the chain is intact. Add a test to prove it.

---

### 8.3 Fix Go error sentinel debt

_Status:_ done

_Decisions:_ Data hydration path already uses fmt.Errorf("%w: %w", errInvalidDataPayload, err) multi-wrap — ErrMissingRequiredField is already preserved. WAL replay path uses the same pattern. Added TestOpenMalformedNodePayloadReturnsErrMissingRequiredField which crafts a msgpack payload with {type: "note"} (no title) and confirms errors.Is(err, ErrMissingRequiredField) = true.

_Notes for 8.4:_ Empty-store test needs to verify both zero nodes and zero edges after reopen. No unspecified behavioral divergences remain after 8.2 fixes.

---

### 8.4 Behavior parity and empty-store edge case

_Status:_ done

_Decisions:_ Added TestEmptyStoreRoundTrip to Go SDK and a matching describe block to TS SDK — both open, close without mutations, reopen, assert zero nodes. Cross-checked all methods: no unspecified divergences remain after 8.2 fixes (WAL error class and default strength). Behavioral parity is confirmed.

_Notes for human review:_ Epics 5–8 complete. All tests pass in both SDKs and the root Go package.
