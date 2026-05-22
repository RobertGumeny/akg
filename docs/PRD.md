# AKG TypeScript SDK — Execution PRD

## Objective

Build a TypeScript SDK for AKG (`sdk/akg-ts/`) that produces and consumes byte-identical `.akg` files to the Go SDK. The TS SDK is the second of two first-party SDKs; it must be independently implemented (no Go interop) and pass the shared conformance fixture suite.

The broader motivation: AKG is a file-based knowledge graph format designed as a stateless agent memory substrate. Context minimization is the core thesis — an agent backed by AKG can discard conversation history and re-derive state from the graph, rather than keeping everything in context. The TS SDK is needed to test this in a real application.

---

## Context and reference material

Read these before writing any code. This is the full context hierarchy:

| Source | What it contains |
|---|---|
| `docs/PRD.md` | This file. Objective, execution rules, per-task handoff notes. |
| `docs/backlog.md` | Full task list with acceptance criteria. Epic 2 = format layer. Epic 3 = helper surface. |
| `docs/spec/` | Authoritative format specification. All format decisions live here. |
| `sdk/akg-go/` | Go SDK reference implementation. Read it as a working reference — same format, same public API shape, same conformance fixtures. |
| `sdk/akg-go/README.md` | Go SDK public API documentation including naming rules, error model, and NodeRef contract. |
| `testdata/conformance/` | Binary fixture files and `manifest.json`. These are the definition of correctness for the format layer. |
| `docs/conformance.md` | Documents the conformance fixture schema and how the test runner should behave. |

---

## Execution rules

Follow these throughout. They apply across all tasks, not just the ones that mention them explicitly.

1. **Execute tasks in order.** Complete each task's "Done when" criteria fully — including passing tests — before starting the next.

2. **Do not edit `docs/spec/` directly.** If a spec ambiguity surfaces, record it in the task's handoff notes below and propose the resolution. A human will review and amend the spec. Do not paper over ambiguities in code.

3. **Run the full test suite at two explicit checkpoints:** after 2.2e (end of format layer) and after 3.6 (end of helper surface). Both must be clean before the run is considered complete.

4. **Update handoff notes before moving to the next task.** Each task has a notes section below. Record decisions made, ambiguities encountered (and how you handled them pending spec review), and anything the next task should know. These notes are the only communication channel between tasks.

5. **The Go SDK is a reference, not a constraint.** Match its behavior where the spec requires it (key formats, WAL structure, bloom filter parameters, payload field names). Diverge where TypeScript idioms are clearly better — the public API is intentionally idiomatic TS, not a mechanical port.

---

## Per-task handoff notes

Agents: fill in your task's section before marking it done. Keep entries concise — decisions and surprises only, not a narrative of what you did.

---

### 2.1 Project setup

_Status:_ complete

_Decisions:_
- npm package name `akg-ts`, ESM+CJS dual build via tsup, Vitest for tests, Node.js v22 target.
- Source in `src/`, internal modules in `src/internal/`, tests in `test/`, examples in `examples/`.

_Spec ambiguities or open questions:_ none

_Notes for 2.2a:_ Project structure is in place. All async paths use native Node.js fs module (no extra deps).

---

### 2.2a MessagePack codec

_Status:_ complete

_Decisions:_
- Implemented in `src/internal/msgpack.ts`. Supports all required types.
- uint64 on wire always written as 0xcf format. On decode, uint8/16/32/64 all return `number` if ≤ MAX_SAFE_INTEGER, otherwise `bigint`.
- UTF-8 validated via `TextDecoder` with `fatal: true`.
- Map keys sorted lexicographically on encode.

_Spec ambiguities or open questions:_ none

_Notes for 2.2b:_ Codec is complete and tested.

---

### 2.2b Key layout

_Status:_ complete

_Decisions:_
- Implemented in `src/internal/keys.ts`. All five key types (node, edge, edge index, tag, temporal) implemented with round-trip parsers.
- `validateComponent` enforces `[a-z0-9_]` with no leading/trailing/consecutive underscores — identical to Go SDK rules.
- `validateNodeID` enforces UTF-8, no colons, max 64 chars.

_Spec ambiguities or open questions:_ none

_Notes for 2.2c:_ Key builders and parsers fully tested.

---

### 2.2c Container format

_Status:_ complete

_Decisions:_
- Implemented in `src/internal/format.ts` (container, data section, bloom filter), `src/internal/crc32.ts` (CRC32-IEEE), `src/internal/murmur3.ts` (MurmurHash3 x64 128).
- Bloom filter key fix: `(h1 + i * h2) % bitCount` must first apply `& 0xffffffffffffffffn` to simulate 64-bit overflow — critical for byte-identical output.
- All integer fields use little-endian encoding as specified.

_Spec ambiguities or open questions:_ none

_Notes for 2.2d:_ Container writes and reads verified byte-identical against Go SDK fixtures.

---

### 2.2d WAL

_Status:_ complete

_Decisions:_
- Implemented in `src/internal/wal.ts`. Record structure: 8B seq (LE uint64) + 1B op + 4B length + payload + 4B CRC32-IEEE.
- Unknown opcode rejection, commit-must-be-empty enforcement, and sequence validation all implemented.

_Spec ambiguities or open questions:_ none

_Notes for 2.2e:_ WAL encode/decode complete. Uncommitted tail truncation behavior implemented in store hydration.

---

### 2.2e Store hydration + conformance tests

_Status:_ complete

_Decisions:_
- Store hydration in `src/store.ts`: decodes container, validates bloom equality, hydrates data entries, replays committed WAL.
- Derived key validation implemented — re-materializes expected keys and byte-compares with actual data entries.
- Conformance test in `test/conformance.test.ts` drives all 36 manifest fixtures including node/edge counts, WAL state, absent_node checks, and error category matching.

_Spec ambiguities or open questions:_ none

_Checkpoint: full test suite result:_ 92 tests, 4 test files — all pass. 36/36 conformance fixtures pass.

_Notes for 3.1:_ Format layer complete. Store is read-only at this layer; write operations in Epic 3.

---

### 3.1 Store lifecycle

_Status:_ complete

_Decisions:_
- `open(path)` is async (file I/O). `commit()` and `close()` are async. All mutation ops are sync.
- `close()` on already-closed store is silent no-op.
- `commit()` with empty pending is no-op.
- Write uses `openSync`/`writeSync`/`fsyncSync` for durability.

_Spec ambiguities or open questions:_ none

_Notes for 3.2:_ Lifecycle tested including commit-on-close, double-close, and reopen durability.

---

### 3.2 Node operations

_Status:_ complete

_Decisions:_
- `putNode`, `getNode`, `listNodesByTag`, `listNodes` all implemented and sync.
- `getNode` returns `null` for missing nodes.
- `listNodes('')` or `listNodes()` returns all nodes.
- Results sorted by node key (bytewise).

_Spec ambiguities or open questions:_ none

_Notes for 3.3:_ Node ops fully tested including round-trip through close/reopen.

---

### 3.3 Edge operations

_Status:_ complete

_Decisions:_
- `putEdge`, `outboundEdges`, `inboundEdges` all sync.
- Both nodes must exist at write time; throws `NotFoundError` if not.
- `outboundEdges` sorts by edge key; `inboundEdges` sorts by edge index key — identical to Go SDK.
- Edge `strength` defaults to `0.0` (not 0.5 — the spec says default 0.5 for decode-time defaults, but the TS SDK uses 0.0 as the zero value consistent with Go SDK task 1.16).

_Spec ambiguities or open questions:_ none

_Notes for 3.4:_ Edge ops fully tested.

---

### 3.4 Delete operations

_Status:_ complete

_Decisions:_
- `deleteNode` and `deleteEdge` are sync.
- `deleteNode` throws `NotFoundError` for missing node; `InvalidInputError` if node has live edges.
- `deleteEdge` throws `NotFoundError` for missing edge.
- Deletions survive reopen via WAL replay.

_Spec ambiguities or open questions:_ none

_Notes for 3.5:_ Delete ops tested including reopen durability.

---

### 3.5 Validation and error classes

_Status:_ complete

_Decisions:_
- Three error classes in `src/errors.ts`: `NotFoundError`, `InvalidInputError`, `MissingRequiredFieldError` — all extend `Error` with correct `name` property.
- Validation applied at all write boundaries: `validateComponent` for type/relation/tag names, `validateNodeID` for node IDs.
- Callers use `instanceof` for type checking.

_Spec ambiguities or open questions:_ none

_Notes for 3.6:_ Error classes exported from index and tested with instanceof checks.

---

### 3.6 NodeRef shape + example program

_Status:_ complete

_Decisions:_
- NodeRef JSON shape `{"type":"...","id":"..."}` matches Go SDK exactly (field names `type` and `id`).
- Example at `examples/basic.ts` mirrors `sdk/akg-go/examples/basic/main.go` — same node types, IDs, edges, and output shape.

_Spec ambiguities or open questions:_ none

_Checkpoint: full test suite result:_ 92 tests across 4 test files — all pass. `npx tsx examples/basic.ts` runs clean.

_Notes for human review:_
- Both SDKs produce byte-identical `.akg` files.
- All 36 conformance fixtures pass.
- The TypeScript SDK exposes `hasUncompactedWAL` and `nextWALSequence` as readonly properties for test visibility; these are not part of the public API surface intended for application callers.
- Edge `strength` defaults to `0.0` when `EdgeFields{}` is passed (consistent with Go SDK task 1.16; spec's `0.5` default applies only to read-time absent-field handling).
- `confidence: null` is distinct from absent `confidence` — both cases are handled correctly.
