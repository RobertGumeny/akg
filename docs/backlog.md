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
