# AKG Developer Experience — Execution PRD

## Objective

Make AKG discoverable and immediately usable. A developer landing on the repo for the first time should be able to determine within 30 seconds whether AKG is for them, and have a working `.akg` file written and read back in under 5 minutes.

The TS SDK (Epic 2 & 3) is complete. Both SDKs produce byte-identical `.akg` files and pass all 36 conformance fixtures. The remaining gap is documentation — neither SDK has a first-time onboarding path, and the root README is structured for maintainers rather than adopters.

---

## Context and reference material

| Source | What it contains |
|---|---|
| `docs/PRD.md` | This file. Objective, execution rules, per-task handoff notes. |
| `docs/backlog.md` | Full task list with acceptance criteria. Epic 4 = developer experience. |
| `docs/spec/` | Authoritative format specification. |
| `sdk/akg-go/README.md` | Go SDK public API documentation — the reference for 4.2 and 4.4. |
| `sdk/akg-go/examples/basic/main.go` | Go quick-start example to port for README snippets. |
| `sdk/akg-ts/` | TypeScript SDK source. Read for accurate API signatures and package name. |
| `README.md` | Root README — primary target for 4.1. |

---

## Execution rules

Follow these throughout. They apply across all tasks.

1. **Execute tasks in order.** 4.1 → 4.2 → 4.3 → 4.4. 4.4 depends on 4.2 existing; complete 4.2 before starting 4.4.

2. **Do not invent API behavior.** Read the SDK source or existing README before writing any code snippet or method description. Every snippet must be runnable as written.

3. **Write for the first-time reader.** Assume zero prior AKG knowledge. The reader has 30 seconds before they bounce — structure serves that, not internal repo organization.

4. **Update handoff notes before moving to the next task.** Record decisions, surprises, and anything the next task needs to know. These are the only communication channel between tasks.

---

## Per-task handoff notes

Agents: fill in your task's section before marking it done. Keep entries concise — decisions and surprises only.

---

### 4.1 Rewrite root README as a "SQLite for agents" landing page

_Status:_ done

_Decisions:_ Pitch is "AKG is a file format for a knowledge graph — open it, read and write nodes and edges, close it. No server, no query language, no setup." Non-goals section placed immediately after pitch, before snippets. "Try the example" and "Validate the repo" collapsed into a single Contributing section at the bottom.

_Notes for 4.2:_ Quick-start snippets use `import { open } from 'akg-ts'` — that's the package name.

---

### 4.2 Add TypeScript SDK README

_Status:_ done

_Decisions:_ Created `sdk/akg-ts/README.md`. Matched Go README section structure exactly (Install, Quick start, Getting started, Naming rules, API, Error handling, NodeRef, Non-goals, Run the example). Key TS differences: `open` / `commit` / `close` are async; `putNode` / `putEdge` / read methods are synchronous; `getNode` returns `Node | null`, not an error. Error handling uses `instanceof`. Included Getting Started section here (satisfies 4.4 requirements in the same file).

_Notes for 4.3:_ Go README Getting Started section added above Naming rules.

---

### 4.3 Getting Started: Go — new project from scratch

_Status:_ done

_Decisions:_ Added Getting Started section to `sdk/akg-go/README.md` above Naming rules. Steps: `go mod init`, `go get`, write `main.go`, `go run .`, expected output shown. Example is ~20 lines including package/import boilerplate.

_Notes for 4.4:_ TS Getting Started was already included in the 4.2 TS README creation.

---

### 4.4 Getting Started: TypeScript — new project from scratch

_Status:_ done

_Decisions:_ Getting Started section included in `sdk/akg-ts/README.md` (created as part of 4.2). Steps: `npm init -y`, `npm install akg-ts`, write `index.ts`, `npx tsx index.ts`, expected output shown.

_Notes for human review:_ `akg-ts` is not yet published to npm — the install step (`npm install akg-ts`) will fail until it is published. The Getting Started section is accurate for when the package is published.
