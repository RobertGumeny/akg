---
title: AKG SDK Backlog
status: active
---

# AKG SDK Backlog

Three epics, in order. Don't start epic 2 until epic 1's conformance tests pass. Don't start epic 3 until epic 2's conformance tests pass.

---

## Epic 1: Go SDK

**Goal:** A standalone Go AKG SDK at `sdk/akg-go/`. Independent implementation — does not import the Go Reference SDK. Conformance tests pass. NodeRef shape locked down for both SDKs.

**Architecture note:** The Go Reference SDK (`akg.go`) is the spec made executable, not a library. The Go SDK reads it as reference material but does not import it. The conformance test fixtures are the shared contract.

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

- [x] **1.8 Expose DeleteNode and DeleteEdge**
  - The WAL already has `walOpDeleteNode` and `walOpDeleteEdge` with full replay logic in `inspectAndReplayWAL`, but neither is reachable from the public API. The codec already has `encodeNodeDeletePayload`/`decodeNodeDeletePayload` and `encodeEdgeDeletePayload`/`decodeEdgeDeletePayload`.
  - Implement `DeleteNode(typeName, id string) error` and `DeleteEdge(fromRef NodeRef, relation string, toRef NodeRef) error` on `*Store`, following the same pending-WAL pattern as `PutNode`/`PutEdge` (update `s.state` immediately, append to `s.pending`).
  - **Deletion semantics (SDK-level decisions, not spec-level):**
    - `DeleteNode` must error if the node has any live inbound or outbound edges. Callers must delete edges first. This is intentionally strict for now; cascading delete is a future extension.
    - `DeleteNode` must error if the node does not exist (`errNotFound`).
    - `DeleteEdge` must error if the edge does not exist (`errNotFound`).
  - Add internal `deleteNode` and `deleteEdge` helpers on `*Store` mirroring the existing `putNode`/`putEdge` pattern.
  - Add conformance fixtures covering delete round-trips. Generate each fixture file programmatically using the SDK (create store, write mutations, commit, close — the resulting `.akg` file is the fixture). Copy the file to `testdata/conformance/`, compute its SHA256, and add an entry to `testdata/conformance/manifest.json`. Cover at minimum: node deleted before commit, node deletion reflected after reopen, edge deleted before commit, edge deletion reflected after reopen.
  - **Done when:** both methods exist on `*Store`, deletion semantics above are enforced, pending WAL writes correctly, reopening a store reflects deletions, and conformance fixtures cover the new cases.

- [x] **1.9a Amend spec: carry full node identity in edge payloads and keys**
  - Root cause of the `OutboundEdges`/`InboundEdges` contamination bug: edge payloads and keys reference nodes by bare `id` string, but the spec defines node identity as `(type, id)`. Edges cannot fully qualify which node they connect to. The fix is a breaking format change — carry `from_node_type` and `to_node_type` explicitly in edge payloads and keys.
  - **Spec changes (all in `docs/spec/`):**
    - `01-data-model.md`: Add `from_node_type: string` (required) and `to_node_type: string` (required) to the edge payload schema. Update the edge identity definition to `(from_node_type, from_node, relation, to_node_type, to_node)`.
    - `03-encoding.md`: Mark `from_node_type` and `to_node_type` as required fields in edge payload encoding.
    - `04-key-layout.md`: Update edge primary key to `e:{fromType}:{fromID}:{relation}:{toType}:{toID}`. Update edge index key to `ei:{toType}:{toID}:{relation}:{fromType}:{fromID}`. Update temporal edge key accordingly.
    - `05-wal.md`: Update `PUT_EDGE` payload to require `from_node_type` and `to_node_type`. Update `DELETE_EDGE` payload to require `from_node_type` and `to_node_type` (in addition to existing identity fields).
    - `09-appendix.md`: Update the WAL delete payload table for `DELETE_EDGE` to include the two new required type fields.
  - No code changes in this task — spec only.
  - **Done when:** all five spec files are updated consistently and the new edge format is unambiguous.

- [x] **1.9b Implement spec change from 1.9a in the Go SDK**
  - Implement the breaking edge format change defined in 1.9a. All five spec files in `docs/spec/` are the authoritative source — read them before writing code.
  - **`core_types.go`:** Add `FromType string` and `ToType string` to `coreEdge`. Expand `edgeIdentity` to `{fromType, from, relation, toType, to}`. Add `FromType`/`ToType` to `edgeDelete`. Update `coreEdge.validateForWrite()` to require non-empty `FromType` and `ToType`. Update `storeState.putEdge()` and `loadEdgeRecord()` to key on the new 5-field `edgeIdentity`.
  - **`keys_internal.go`:** Update `parsedEdgeKey` and `parsedEdgeIndexKey` to include type fields. Update `buildEdgeKey` signature and format to `e:{fromType}:{fromID}:{relation}:{toType}:{toID}` (6 parts). Update `parseEdgeKey` to parse 6 parts. Update `buildEdgeIndexKey` and `parseEdgeIndexKey` similarly. Update `buildTemporalEdgeKey` to use the new `buildEdgeKey` signature.
  - **`codec_internal.go`:** Add `from_node_type` and `to_node_type` as required fields in edge payload MessagePack encode/decode. Add them as required fields in `DELETE_EDGE` payload encode/decode.
  - **`edge.go`:** Update `coreEdgeFromFields` to populate `FromType`/`ToType` from `fromRef.Type`/`toRef.Type`. Update `edgeFromRecord` to read types directly from the edge record instead of calling `resolveNodeType`. Delete `resolveNodeType` — it is no longer needed.
  - **`store.go`:** Update `OutboundEdges` scan to filter on `rec.FromType == nodeRef.Type && rec.FromNode == nodeID(nodeRef.ID)`. Update `InboundEdges` to filter on `rec.ToType == nodeRef.Type && rec.ToNode == nodeID(nodeRef.ID)`. Update all `buildEdgeKey`/`buildEdgeIndexKey` callsites to pass type arguments. Update WAL `DELETE_EDGE` replay to use the new 5-field `edgeIdentity`.
  - **Fixture regeneration:** Regenerate all conformance fixtures that contain edges — `m2-single-edge.akg`, `m2-small-graph.akg`, `m1-data-bloom-wal.akg`, `m2-deletes-before-compaction.akg`. Update their SHA256 values in `testdata/conformance/manifest.json`. Generate each file programmatically using the SDK (same pattern as prior fixture tasks).
  - **New test:** Add a test that creates two nodes of different types sharing the same ID string, connects one via an edge, and verifies that `OutboundEdges` on the other type returns empty rather than the wrong edge.
  - **Done when:** `go test ./...` passes, all affected conformance fixtures are regenerated with correct SHA256s in `manifest.json`, and the cross-type collision test passes.

- [x] **1.10 Add ListNodes (enumerate all nodes, optionally by type)**
  - There is currently no way to enumerate all live nodes without knowing their tags. Agents iterating the full graph context have no entry point.
  - Implement `ListNodes(typeName string) ([]Node, error)` on `*Store`. If `typeName` is empty, return all live nodes. If non-empty, return only nodes of that type. Results sorted by node key (consistent with `ListNodesByTag`).
  - If `typeName` is non-empty, validate it with `validateComponent` before scanning — consistent with how other methods validate their inputs.
  - A non-existent type returns an empty slice and `nil` error (not `errNotFound`) — consistent with `ListNodesByTag` returning empty for a tag with no matches.
  - **Done when:** `go test ./...` passes with test cases covering: all-nodes (empty typeName), type-filtered, empty typeName returns all nodes, unknown type returns empty slice, invalid typeName returns error.

- [x] **1.11 Document Close and Commit semantics**
  - `Close()` commits any pending mutations before closing — this is the intended, idiomatic behavior. `Commit()` is a no-op (returns `nil`) when there is nothing pending — also intentional. Calling `Close()` on an already-closed store returns `nil` silently — also intentional. All three behaviors are undocumented.
  - Update the doc comment on `Close` to state: (1) it commits pending mutations before closing, (2) calling it on an already-closed store is a no-op. Update the doc comment on `Commit` to state that it is a no-op when there are no pending mutations.
  - **Done when:** both doc comments are updated and tests cover: commit-on-close (mutations written after last `Commit` survive a `Close` + `Open` round-trip), no-op on empty pending (`Commit` called twice in a row returns `nil` and does not corrupt state), and close-on-already-closed (returns `nil`).

- [x] **1.12 Conformance fixture housekeeping (three small items from 1.9b)**
  - Three friction points surfaced during 1.9b that are worth fixing before the next format change.
  - **1.12a — Fix the `m2-reject-malformed-committed-wal` corruption note.** The manifest `corruption` field currently says "one byte in the committed WAL record is flipped", which is too vague. A reader has to trace `decodeWALRecord` to understand why the fixture yields `invalid_wal_record` and not `wal_checksum_mismatch`. Update the note to state: "the length field (bytes 9–12) is overwritten to `0x7FFFFFFF`, triggering the buffer-size check in `decodeWALRecord` before the CRC check, yielding `errInvalidWALRecord`." No file or code changes — manifest only.
  - **1.12b — Consolidate fixture generators.** `cmd/gen_delete_fixtures` is now a strict subset of `gen_conformance_fixtures_test.go` — the internal test file covers all four of its fixtures and has access to internals when needed. Delete `cmd/gen_delete_fixtures/main.go` and update `manifest.json` `generated_by` fields for the four affected fixtures to point at `Go SDK TestGenEdgeConformanceFixtures`.
  - **1.12c — Tag fixtures with what they touch.** The blast radius of the 1.9b edge format change had to be discovered by running tests and reading failures; the manifest gave no upfront signal. Add a `"features"` array field to each manifest entry listing the logical capabilities exercised (e.g. `["edges"]`, `["wal"]`, `["bloom"]`, `["edge_wal"]`). No test changes — this field is informational and ignored by the conformance runner. Update `README.md` to document the field. The goal is that the next format change can `grep "edges"` the manifest and know its fixture blast radius before touching code.
  - **Done when:** (a) the manifest corruption note is updated; (b) `cmd/gen_delete_fixtures` is deleted and manifest `generated_by` fields reflect the new generator; (c) every manifest entry has a `features` field and the README documents it.

- [x] **1.13 Tighten validateComponent to match validateTag character rules**
  - `validateComponent` currently accepts any non-empty, valid UTF-8 string that does not contain `:`. This means type names and relation names can contain uppercase letters, spaces, punctuation, and other characters that would be rejected by `validateTag`. The TS SDK will inherit whatever contract is locked in here, so tighten it now before that work begins.
  - Update `validateComponent` in `keys_internal.go` to enforce the same character rules as `validateTag`: lowercase `a–z`, digits `0–9`, and underscores — with underscores disallowed at the start, the end, or consecutively.
  - `validateTag` calls `validateComponent` as its first check and then applies the same character loop — after this change, `validateTag` can delegate entirely to `validateComponent` and remove its own duplicate loop, or the two can be collapsed into one function. Either way, there should be no duplicated validation logic.
  - Update the existing `TestListNodes` test: replace `"bad:type"` with a value that is invalid under the new rules but valid under the old ones (e.g. `"BadType"`) to ensure the tightened check is actually exercised.
  - **Done when:** `validateComponent` enforces lowercase-alphanumeric-underscore, `validateTag` contains no duplicate character logic, and `go test ./...` passes with updated test coverage.

- [x] **1.14 Export public error sentinels**
  - All SDK error values (`errNotFound`, `errInvalidInput`, `errMissingRequiredField`, etc.) are unexported. Callers cannot write `errors.Is(err, akg.ErrNotFound)` — they must treat all errors as opaque, which makes correct error handling in application code impractical.
  - Export the three sentinels callers realistically need to branch on: `ErrNotFound`, `ErrInvalidInput`, and `ErrMissingRequiredField`. Rename them in place (capitalise the existing unexported vars) — no new symbols, no new files. Update all internal call sites.
  - Add a section to `README.md` documenting what each exported error means and when it is returned.
  - **Done when:** `ErrNotFound`, `ErrInvalidInput`, and `ErrMissingRequiredField` are exported, all internal call sites compile, and the README documents the three values.

- [x] **1.15 Wrap error causes in data and WAL decode paths**
  - Three places in `store.go` swallow the original decode error, returning a bare sentinel with no cause attached. This makes corrupt-file diagnostics useless — the caller sees `"invalid data payload"` with no indication of which field failed or why.
  - In `hydrateDataEntries`: the two `return nil, errInvalidDataPayload` lines after `decodeNodePayload` and `decodeEdgePayload` fail. In `inspectAndReplayWAL`: the `return nil, 0, errInvalidWALPayload` line after `validateWALPayload` fails.
  - Change each to `fmt.Errorf("…: %w", err)` using the existing sentinel as the message prefix. The outer sentinel type is preserved (callers using `errors.Is` still match), the inner cause becomes inspectable via `errors.Unwrap`.
  - No API changes, no new tests required — this is a diagnostics-only improvement.
  - **Done when:** all three sites use `%w` wrapping and `go test ./...` passes.

- [ ] **1.16 Allow Strength 0.0 on edges**
  - `applyReadDefaults` silently converts a zero `Strength` to `0.5`, making it impossible to write an edge with explicit strength `0.0`. `EdgeFields{}` also silently produces `0.5` rather than a true zero.
  - Remove the `if e.Strength == 0 { e.Strength = 0.5 }` line from `applyReadDefaults`. `EdgeFields{}` now produces strength `0.0`. Update the `EdgeFields` table in `README.md` — the `Strength` default column should reflect that `0.0` is the zero value (remove the "defaults to 0.5" note). Update any tests that assert the old `0.5` default behaviour.
  - **Done when:** `EdgeFields{}` produces an edge with `Strength == 0.0`, `go test ./...` passes, and the README is updated.

- [ ] **1.17 Fix double msgpack decode in `decodeNodePutPayload`**
  - `decodeNodePutPayload` in `codec_internal.go` currently decodes the full MessagePack payload twice: once via `decodeNodePayload` to build the `coreNode`, then again via `decodeMsgpack` directly to extract the `id` field.
  - Refactor so the payload is decoded once. Extract both the `id` field and the node fields from the single decoded map, then call the existing `decodeNodePayload` field-extraction logic or inline it as appropriate.
  - **Done when:** the payload is decoded exactly once, behaviour is unchanged, and `go test ./...` passes.

---

## Epic 2: TypeScript SDK

**Goal:** From-scratch TS implementation of AKG at `sdk/akg-ts/`. Produces and consumes byte-identical `.akg` files. Conformance tests pass.

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
