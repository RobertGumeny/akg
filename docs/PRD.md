# AKG TypeScript SDK — Execution PRD

## Goal prompt

> Please execute all steps outlined in `docs/PRD.md` and report back when done.

---

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

_Status:_ not started

_Decisions:_

_Spec ambiguities or open questions:_

_Notes for 2.2a:_

---

### 2.2a MessagePack codec

_Status:_ not started

_Decisions:_

_Spec ambiguities or open questions:_

_Notes for 2.2b:_

---

### 2.2b Key layout

_Status:_ not started

_Decisions:_

_Spec ambiguities or open questions:_

_Notes for 2.2c:_

---

### 2.2c Container format

_Status:_ not started

_Decisions:_

_Spec ambiguities or open questions:_

_Notes for 2.2d:_

---

### 2.2d WAL

_Status:_ not started

_Decisions:_

_Spec ambiguities or open questions:_

_Notes for 2.2e:_

---

### 2.2e Store hydration + conformance tests

_Status:_ not started

_Decisions:_

_Spec ambiguities or open questions:_

_Checkpoint: full test suite result:_

_Notes for 3.1:_

---

### 3.1 Store lifecycle

_Status:_ not started

_Decisions:_

_Spec ambiguities or open questions:_

_Notes for 3.2:_

---

### 3.2 Node operations

_Status:_ not started

_Decisions:_

_Spec ambiguities or open questions:_

_Notes for 3.3:_

---

### 3.3 Edge operations

_Status:_ not started

_Decisions:_

_Spec ambiguities or open questions:_

_Notes for 3.4:_

---

### 3.4 Delete operations

_Status:_ not started

_Decisions:_

_Spec ambiguities or open questions:_

_Notes for 3.5:_

---

### 3.5 Validation and error classes

_Status:_ not started

_Decisions:_

_Spec ambiguities or open questions:_

_Notes for 3.6:_

---

### 3.6 NodeRef shape + example program

_Status:_ not started

_Decisions:_

_Spec ambiguities or open questions:_

_Checkpoint: full test suite result:_

_Notes for human review:_
