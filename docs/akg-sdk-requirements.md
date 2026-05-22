---
title: AKG SDK Requirements
status: draft
author: Robert Gumeny
date: 2026-05-21
---

# AKG SDK Requirements

Two SDKs need to be built on top of AKG core: a Go SDK and a TypeScript SDK. Both are general-purpose AKG infrastructure — not tied to any specific consumer. The Agent Poker research demo is the first consumer of both, but neither SDK should reflect that.

## 1. Helper surface

Both SDKs expose the same conceptual surface, language-idiomatically. This is the v0 surface.

```
Open(path)                              -> Store
Close(store)
Commit(store)

PutNode(typeName, id, fields, tags)     -> NodeRef
GetNode(typeName, id)                   -> Node | null
ListNodesByTag(tag)                     -> Node[]

PutEdge(fromRef, relation, toRef, fields)
OutboundEdges(nodeRef, relation?)       -> Edge[]
InboundEdges(nodeRef, relation?)        -> Edge[]
```

Explicitly out of scope for v0:

- Traversal language / query DSL
- Ingestion pipelines
- Schema definitions or validation
- Streaming / observation APIs
- Anything that looks like an ORM

The helpers map directly onto AKG core's existing derived keys (`t:`, `e:`, `ei:`). The SDK is a thin readable wrapper, not a new semantic layer.

`PutNode` must return a stable, compact reference (e.g. `{type: "Document", id: "doc_01"}`). This reference shape is part of the public API, must be identical across both SDKs, and should be documented as such.

## 2. Go SDK

**Location:** `github.com/RobertGumeny/akg-go`

**Definition of done:**
- Helper surface implemented.
- Conformance tests pass.
- An example program writes a few nodes and edges and reads them back.

## 3. TypeScript SDK

**Location:** `sdk/akg-ts/`

This is a from-scratch TypeScript implementation of AKG core — not a binding to the Go reference. It must produce and consume byte-identical `.akg` files, proven by the conformance tests.

The TS port is also a spec-hardening pass. Every ambiguity or implicit assumption in the Go reference will surface during the port. Each one is either a spec amendment or a TS bug. Amendments get folded back into `docs/spec/` — not buried in code comments or commit messages.

Runs in Node. Filesystem access, native crypto, and native buffers are all fair game. Correctness is the priority; performance is not a concern at v0 scale.

**Definition of done:**
- Conformance tests pass against the TS implementation.
- Helper surface implemented, same shape as §1.

Do the conformance tests first. The helper surface is mostly a consequence of that being real.

## 4. Conformance tests

The existing test files are the primary correctness signal for the TS port. Expect to extend them as the TS work uncovers gaps — budget for it.

## 5. Suggested phasing

1. **Go SDK** — helper surface, conformance tests pass, example program runs. NodeRef shape defined and documented.
2. **TS SDK core** — from-scratch port, conformance tests pass. Every failing case is a spec question first, implementation question second.
3. **TS SDK helper surface** — layered on top of TS core, same shape as Go SDK §1.
