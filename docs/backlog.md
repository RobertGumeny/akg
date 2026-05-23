---
title: AKG Backlog
status: active
---

# AKG Backlog

---

## Epic 4: Developer Experience

**Goal:** A developer can find the AKG repo and be interacting with an AKG file in a demo project of their own within 5 minutes. A developer can find the repo and understand what AKG is for, why it is better for specific use cases, and what it is NOT for, within ~30 seconds. Reading the README is enough to start using the official SDKs without getting bogged down in details.

---

- [x] **4.1 Rewrite root README as a "SQLite for agents" landing page**
  - The current README opens with a definition and three audience paths. A developer landing here for the first time needs to make a go/no-go decision in ~30 seconds — the structure should serve that, not the internal repo map.
  - **Structure (in order):**
    - One-sentence pitch: what AKG is, in concrete terms. Model it on the SQLite tagline — functional, not marketing.
    - **Non-goals section** (explicit, named, first-class): no query language, no server, no semantic search, no multi-writer sync. This is the most valuable signal for a developer evaluating adoption — they need to know if they're in the wrong place as fast as possible.
    - Quick-start snippets for both Go and TypeScript — minimal, self-contained, runnable. ~10 lines each. Not a pointer to another file. The code itself.
    - Audience paths (Building in Go / Building in TypeScript / Implementing in another language) — these stay but come after the pitch and snippets.
    - Repo contents table — keep it, move it after the above.
  - Strip the "Try the example" and "Validate the repo" sections from prominence — they're maintainer-facing, not adopter-facing. Move to the bottom or a Contributing section.
  - **Done when:** a cold reader can determine within 30 seconds whether AKG is for them, and can copy a working snippet directly from the README.

- [x] **4.2 Add TypeScript SDK README**
  - The Go SDK has a 174-line README with install, quick start, API reference, naming rules, error handling, and NodeRef contract. The TypeScript SDK has no README at all. Any JS/TS developer landing in `sdk/akg-ts/` must read source code to understand the API.
  - Port the Go SDK README to TypeScript. Adapt for TS idioms: `npm install`, `async/await`, `instanceof` for error handling, `null` return from `getNode`. Keep section structure and depth identical — the two READMEs should be directly comparable.
  - Include the same sections: Install, Quick start, Naming rules, API (with each method's signature and behavior), Error handling, NodeRef, Run the example.
  - Include a Non-goals section (same content as 4.1) so the TS README stands alone.
  - **Done when:** `sdk/akg-ts/README.md` exists, a developer can install and use the TS SDK using only that file, and it matches the Go README in depth.

- [x] **4.3 Getting Started: Go — new project from scratch**
  - The Go SDK README documents the API but skips the "start from zero" arc: create a directory, `go mod init`, `go get`, write the first file, run it. That gap is where "5 minutes" becomes "20 minutes" for a first-time adopter.
  - Add a Getting Started section to `sdk/akg-go/README.md` (above the API reference) that walks through: create a project directory, `go mod init`, `go get github.com/RobertGumeny/akg-go`, write a 15-line `main.go`, `go run .`, see output. Use actual commands the reader can copy-paste. No pointers to other files — the complete path in one place.
  - **Done when:** a Go developer with zero prior AKG knowledge can follow the Getting Started section and have a `.akg` file written and read back in under 5 minutes.

- [x] **4.4 Getting Started: TypeScript — new project from scratch**
  - Same as 4.3 but for TypeScript. Depends on 4.2 (TS SDK README must exist first).
  - Add a Getting Started section to `sdk/akg-ts/README.md` that walks through: create a project directory, `npm init -y`, `npm install akg-ts`, write a 15-line `index.ts`, `npx tsx index.ts`, see output.
  - **Done when:** a TypeScript developer with zero prior AKG knowledge can follow the Getting Started section and have a `.akg` file written and read back in under 5 minutes.

---

## Epic 5: Pre-Release Housekeeping

**Goal:** The repo reflects its new name, both SDKs are ready to publish, and the docs folder contains only content useful to external readers. No internal process artifacts, no stale module paths, no missing license.

---

- [x] **5.1 Update Go SDK module path**
  - The GitHub repo was renamed from `akg-format` to `akg`. The Go SDK module declaration and all internal imports still reference the old path. Every snippet in both SDK READMEs and the root README that shows an install command or import statement needs to reflect the new path.
  - Update `sdk/akg-go/go.mod` to `module github.com/RobertGumeny/akg/sdk/akg-go`. Update all internal imports within `sdk/akg-go/`. Update the `go get` command and import snippet in `sdk/akg-go/README.md` and the Go quick-start snippet in the root `README.md`.
  - **Done when:** every reference to the old module path is gone, `go test ./...` passes in `sdk/akg-go/`, and the install and import snippets in both READMEs match the new path.

- [x] **5.2 Release prep**
  - Two things are missing before either SDK can be published. There is no `LICENSE` file, which means the repo is technically all-rights-reserved. The `akg-ts` `package.json` has no `files` field, so `npm publish` would ship test files, `tsconfig.json`, `tsup.config.ts`, and all source under `src/` — none of which belong in the published package.
  - Add an Apache 2.0 `LICENSE` file to the repo root. Add `"files": ["dist", "README.md"]` to `sdk/akg-ts/package.json`.
  - **Done when:** `LICENSE` exists at the repo root, and `npm pack --dry-run` from `sdk/akg-ts/` lists only `dist/` and `README.md`.

- [x] **5.3 Docs cleanup**
  - `docs/` contains internal process artifacts that have no value to external readers: the original SDK requirements doc, the repository boundaries doc, the overview (`core.md`, which now overlaps entirely with the root README), and the reference SDK API doc (`API.md`, which documents a minimal internal surface that is not the application SDK). Leaving these in place clutters the repo and gives agents and contributors false signals about what is authoritative.
  - Delete `docs/akg-sdk-requirements.md`, `docs/repository-boundaries.md`, `docs/core.md`, and `docs/API.md`. Update the repo contents table in `README.md` to reflect what actually remains in `docs/`.
  - **Done when:** `docs/` contains only `spec/`, `conformance.md`, `sdk-author-guide.md`, and `lifecycle.md`, and the repo contents table in `README.md` matches.

---

## Epic 6: CI/CD & Repo Infrastructure

**Goal:** Every push and PR runs a full test suite automatically. The repo has the standard files a contributor or agent needs to orient themselves. README badges reflect live status.

---

- [x] **6.1 GitHub Actions CI workflow**
  - There is no `.github/workflows/` directory. Contributors have no automated signal that their changes broke something, and the CI badge in the README cannot be added until a workflow exists.
  - Add a workflow file that triggers on push and pull request. Two jobs: a Go job running `go test -count=1 ./...`, `cd sdk/akg-go && go test ./...`, and `go vet ./...`; a TypeScript job running `npm ci && npm test`, `npm run build`, and `npx tsc --noEmit` from `sdk/akg-ts/`.
  - **Done when:** the workflow file exists, both jobs are defined, and the structure is correct enough that they would pass on a clean push.

- [x] **6.2 Issue templates and PR template**
  - Without templates, bug reports arrive with no reproduction steps and feature requests arrive with no context. A minimal PR template sets expectations without being bureaucratic.
  - Add a bug report issue template and a feature request issue template under `.github/ISSUE_TEMPLATE/`. Add a minimal PR template at `.github/PULL_REQUEST_TEMPLATE.md` covering what changed and how to test it — nothing else.
  - **Done when:** both issue templates and the PR template exist and are well-formed GitHub template files.

- [x] **6.3 Repo root files**
  - The repo is missing several standard files that contributors and agents rely on. Agents working in this codebase have no structured map of what is authoritative. There is no guidance on reporting security issues and no changelog to anchor releases to.
  - Add `AGENTS.md` at the repo root explaining the structure: `docs/spec/` is the authoritative format contract, the reference SDK in the root package proves the spec, application SDKs live in `sdk/`, and conformance fixtures are in `testdata/conformance/`. Add `CLAUDE.md` containing only `@AGENTS.md`. Add `SECURITY.md` describing how to report vulnerabilities privately. Add `CHANGELOG.md` seeded with a v0.1.0 entry.
  - **Done when:** all four files exist, `AGENTS.md` accurately describes the repo structure, and `CLAUDE.md` correctly references it.

- [x] **6.4 README badges**
  - The root README has no badges. A CI badge, Go Reference badge, npm badge, and license badge give a landing developer immediate signal about project health and where to find SDK docs. Depends on the workflow name established in 6.1.
  - Add CI status, Go Reference (pkg.go.dev), npm version, and Apache 2.0 license badges to the top of `README.md`. Badge URLs must match the actual workflow filename from 6.1 and the published package name `akg-ts`.
  - **Done when:** all four badges are present in `README.md` and point to the correct URLs.

---

## Epic 7: Test Coverage Gaps

**Goal:** Both SDKs cover all validation boundaries and the cross-type edge case that is a known source of subtle bugs. Test behavior is symmetric across both SDKs wherever the APIs are symmetric.

---

- [x] **7.1 Go SDK test gaps**
  - The Go SDK store tests cover the happy path and lifecycle thoroughly but have no tests for validation errors, no test for referencing a nonexistent node in `PutEdge`, and no test for the auto-ID generation path.
  - Add tests for: `ErrInvalidInput` returned on invalid type name, invalid relation name, and invalid tag; `ErrMissingRequiredField` returned when `Title` is absent from a `PutNode` call; `PutEdge` returning an error when the source node does not exist and when the target node does not exist; `PutNode` with an empty `id` string returning a non-empty generated ID.
  - **Done when:** all cases above have passing tests in `sdk/akg-go/store_test.go`.

- [x] **7.2 TypeScript SDK test gaps**
  - The TypeScript store tests are missing the cross-type contamination case that the Go SDK covers, the inbound-edge variant of the `deleteNode` blocked-by-edges test, and any coverage of the `confidence` field.
  - Add tests for: `outboundEdges` and `inboundEdges` on a node that shares an ID string with a node of a different type returning no cross-type results; `deleteNode` throwing `InvalidInputError` when the node has only inbound edges (not outbound); `confidence` set to a number, read back correctly, surviving a close/reopen cycle, and defaulting to `null` when not provided.
  - **Done when:** all cases above have passing tests in `sdk/akg-ts/test/store.test.ts`.

- [x] **7.3 Shared validation gaps (both SDKs)**
  - Neither SDK has tests for node ID format constraints or tag array constraints. These are specified in the format and enforced at write time but not exercised in any test.
  - For both SDKs, add tests for: `putNode` / `PutNode` rejecting a node ID that contains a colon; rejecting a node ID longer than 64 characters; rejecting a `tags` array with more than 32 entries; rejecting a `tags` array with duplicate values.
  - **Done when:** all four constraint cases are tested and passing in both `sdk/akg-go/store_test.go` and `sdk/akg-ts/test/store.test.ts`.

---

## Epic 8: Code Quality Audit

**Goal:** Both SDKs are production quality. Error chains are intact, behavior is symmetric, and no latent bugs survive a careful read-through.

---

- [x] **8.1 Go SDK audit**
  - The Go SDK was built quickly and has not had a dedicated quality pass. Error handling consistency, correct use of sentinel errors, and format layer edge cases may have gaps that are not caught by the existing tests.
  - Read through all files in `sdk/akg-go/`. Check that every error return is either a sentinel or wraps one with `%w`. Check that no function silently swallows an error. Look for any logic that would produce incorrect results on valid inputs that happen not to appear in the test suite. Fix everything found.
  - **Done when:** the read-through is complete, all identified issues are fixed, and `go test ./...` passes in `sdk/akg-go/`.

- [x] **8.2 TypeScript SDK audit**
  - Same quality pass for the TypeScript SDK. Particular attention to async boundaries — any place where a sync function performs work that should be guarded by the closed-store check, and any error that is caught and re-thrown in a way that loses the original message.
  - Read through all files in `sdk/akg-ts/src/`. Apply the same criteria as 8.1 adapted for TypeScript: consistent use of the three error classes, no swallowed errors, no incorrect behavior on valid edge-case inputs. Fix everything found.
  - **Done when:** the read-through is complete, all identified issues are fixed, and `npm test` passes in `sdk/akg-ts/`.

- [x] **8.3 Fix Go error sentinel debt**
  - WAL and data payload decode errors in the Go SDK are currently wrapped in a way that discards the original error cause. A caller using `errors.Is` to test for `ErrMissingRequiredField` on a file opened from disk will not get the expected result if the error originated in a WAL or data payload decode path.
  - Audit all error wrapping in the WAL replay and data hydration paths. Replace any wrapping that discards the cause with `fmt.Errorf("context: %w", err)` so the full chain is preserved. Add a test that opens a file with a malformed payload and confirms `errors.Is(err, ErrMissingRequiredField)` returns true.
  - **Done when:** `errors.Is` works correctly through all error wrapping paths and the new test passes.

- [x] **8.4 Behavior parity and empty-store edge case**
  - No test currently verifies that both SDKs handle a store that is opened and closed without any mutations ever being committed — a valid and common state for a newly created file. Beyond that, any method present in both SDKs should produce identical results given identical inputs; undocumented divergences are latent bugs.
  - Add a test to both SDKs: open a store, close immediately without writing anything, reopen, assert zero nodes and zero edges. Then cross-check every method available in both SDKs against its counterpart's test suite — any behavioral difference that is not explicitly specified should be treated as a bug in the newer implementation and fixed.
  - **Done when:** the empty-store test passes in both SDKs and no unspecified behavioral divergences remain between them.
