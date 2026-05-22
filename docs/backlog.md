---
title: AKG SDK Backlog
status: active
---

# AKG SDK Backlog

Three epics, in order. Don't start epic 2 until epic 1's conformance tests pass. Don't start epic 3 until epic 2's conformance tests pass.

---

## Epic 1: Go SDK

**Goal:** A standalone Go AKG SDK at `sdk/akg-go/`. Independent implementation — does not import the reference package. Conformance tests pass. NodeRef shape locked down for both SDKs.

**Architecture note:** The reference implementation (`akg.go`) is the spec made executable, not a library. The Go SDK reads it as reference material but does not import it. The conformance test fixtures are the shared contract.

- [x] **1.1 Module setup**
  - Scaffold `sdk/akg-go/` with its own `go.mod`.
  - Decided module path: `github.com/RobertGumeny/akg-go`
  - Does not import `github.com/RobertGumeny/akg-format` (the reference).
  - **Done when:** `go mod tidy` runs clean, directory structure is in place, no reference import.

- [x] **1.2 Define and document NodeRef shape**
  - `PutNode` and `PutEdge` return a `NodeRef` — a stable, compact, JSON-serializable identifier (e.g. `{"type": "Hand", "id": "h_47"}`).
  - This shape is public API and must be held identical in the TS SDK. Lock it down here first.
  - Document it in `sdk/akg-go/README.md` or an inline doc comment — wherever downstream SDK authors will find it.
  - **Done when:** `NodeRef` type is defined, its JSON shape is specified and documented, and there's a note that the TS SDK must match it exactly.

- [x] **1.3 Store lifecycle**
  - Implement `Open(path) -> Store`, `Close(store)`, `Commit(store)`.
  - Path-based only, no global state, no implicit config.
  - WAL must be intact after `Close` — a store closed and reopened via `Open` must reflect all committed mutations.
  - **Done when:** a store can be created, written to, committed, closed, and reopened with data intact.

- [x] **1.4 Node operations**
  - Implement `PutNode(typeName, id, fields, tags) -> NodeRef`, `GetNode(typeName, id) -> Node | null`, `ListNodesByTag(tag) -> Node[]`.
  - `ListNodesByTag` maps onto the `t:` derived keys in the AKG format.
  - **Done when:** all three operations work correctly against real `.akg` files and round-trip cleanly through close/reopen.

- [x] **1.5 Edge operations**
  - Implement `PutEdge(fromRef, relation, toRef, fields)`, `OutboundEdges(nodeRef, relation?) -> Edge[]`, `InboundEdges(nodeRef, relation?) -> Edge[]`.
  - `OutboundEdges`/`InboundEdges` map onto `e:` and `ei:` derived keys respectively. `relation` filter is optional — omitting it returns all edges in that direction.
  - **Done when:** all three operations work correctly against real `.akg` files.

- [x] **1.6 Conformance tests**
  - Wire `testdata/conformance/` fixtures (from the repo root) against the SDK's open/validate path.
  - For each fixture: `accept` cases must open without error, `reject` cases must fail. Match on `expected_error_category`, not exact error strings.
  - **Done when:** `go test ./...` in `sdk/akg-go/` passes all fixture cases with no skips.

- [x] **1.7 Example program**
  - ~50-line program at `sdk/akg-go/examples/basic/main.go`.
  - Demonstrates the full helper surface: open a store, write nodes with tags, write edges, read back via `GetNode`, `ListNodesByTag`, `OutboundEdges`. Print human-readable output.
  - This is the template downstream consumers will copy — write it to be readable, not clever.
  - **Done when:** `go run ./examples/basic` runs cleanly from `sdk/akg-go/` and produces legible output.

---

## Epic 2: TypeScript SDK Core

**Goal:** From-scratch TS implementation of AKG core at `sdk/akg-ts/`. Produces and consumes byte-identical `.akg` files. Conformance tests pass.

**Architecture note:** Same boundary as the Go SDK — does not import or wrap any Go code. The spec (`docs/spec/`) and the conformance test fixtures are the implementation contract.

> Task list is intentionally coarse. Expect to add tasks as spec gaps surface — each one is either a spec amendment in `docs/spec/` or a TS bug.

- [ ] **2.1 Project setup**
  - Scaffold `sdk/akg-ts/` with its own `package.json`.
  - npm package name: `akg-ts`.
  - TypeScript config, tsup for build, Vitest for tests.
  - Does not import or shell out to the Go reference.
  - **Done when:** `npm install`, `npm run build`, and `npm test` all run clean against an empty test suite.

- [ ] **2.2 Core port + conformance tests**
  - Implement the following, using `docs/spec/` as the authoritative source. Run the conformance tests continuously as you go — they are the definition of done, not a separate step:
    - Binary container: header, section table, section payloads (spec §2).
    - Payload encoding: MessagePack for node and edge records, UTF-8 validation (spec §3).
    - Key layout: primary node/edge keys, derived tag and edge-index keys (`t:`, `e:`, `ei:`) (spec §4).
    - WAL: record format, committed vs. uncommitted tail, replay on open, sequence validation (spec §5).
    - Error handling: fail-closed reader — reject on bad magic, unsupported version, checksum mismatch, invalid section table, malformed payloads, derived index mismatch (spec §7).
  - Wire `testdata/conformance/manifest.json` as the test driver from the start. For each fixture: `accept` cases must open without error and match `store_expectation` (node count, edge count, WAL state); `reject` cases must fail and match `expected_error_category`. Respect `validation_scope` (`format` vs `store`).
  - Every time a spec ambiguity surfaces, resolve it in `docs/spec/` first, then implement. Do not paper over it in code. Add new fixtures when TS work uncovers gaps not covered by existing ones.
  - **Done when:** all conformance tests pass with no skips, including any newly added ones.

---

## Epic 3: TypeScript SDK Helper Surface

**Goal:** Same `Open / PutNode / PutEdge / etc.` surface as the Go SDK, layered on top of Epic 2. Idiomatic TypeScript — not a mechanical translation of the Go API.

- [ ] **3.1 Store lifecycle**
  - Implement `Open(path): Store`, `Close()`, `Commit()`.
  - Same semantics as Go SDK: path-based, no global state, WAL intact after close.
  - **Done when:** a store can be created, written to, committed, closed, and reopened with data intact.

- [ ] **3.2 Node operations**
  - Implement `PutNode(typeName, id, fields, tags): NodeRef`, `GetNode(typeName, id): Node | null`, `ListNodesByTag(tag): Node[]`.
  - **Done when:** all three operations round-trip correctly through close/reopen against real `.akg` files.

- [ ] **3.3 Edge operations**
  - Implement `PutEdge(fromRef, relation, toRef, fields)`, `OutboundEdges(nodeRef, relation?): Edge[]`, `InboundEdges(nodeRef, relation?): Edge[]`.
  - **Done when:** all three operations work correctly against real `.akg` files.

- [ ] **3.4 NodeRef shape + example program**
  - Verify `NodeRef` JSON shape is byte-identical to the shape locked down in task 1.2.
  - Write a ~50-line example at `sdk/akg-ts/examples/basic.ts` mirroring the Go SDK example: open a store, write nodes with tags, write edges, read back via `GetNode`, `ListNodesByTag`, `OutboundEdges`. Print human-readable output.
  - **Done when:** NodeRef shape matches, `npx tsx examples/basic.ts` runs cleanly and produces legible output.
