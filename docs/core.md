---
title: AKG core concepts
status: release-candidate docs
---

# AKG core concepts

AKG is a portable, single-file knowledge graph format. Knowledge graphs already
work well for agent context, but they are often trapped inside graph servers,
framework-specific stores, or app-specific schemas. AKG makes that graph a file
an agent can carry across tools and hosts.

An AKG file stores a current graph state made of typed nodes, explicit edges,
derived indexes, and a write-ahead log for committed mutations between
compactions.

The goal is interoperability: one implementation can write an AKG file, another
can validate it, and an SDK can build product behavior above it without inventing
a private memory format.

## What belongs in core

AKG core covers:

- the v1 binary file format and validation rules;
- node, edge, key, section, Bloom, WAL, and compaction semantics;
- a conformance corpus of accepted and rejected `.akg` files;
- a minimal Go reference implementation;
- examples that demonstrate format lifecycle operations.

## What does not belong in core

AKG core does not include:

- memory-file ingestion or agent workflow policy;
- vector search, ranking, embeddings, or semantic retrieval pipelines;
- a query planner, traversal language, or graph analytics layer;
- merge/conflict resolution, multi-writer behavior, or background services;
- product SDK conveniences that are not required to read and write the format.

Those features can be valuable and complementary, but they should live in SDKs,
applications, indexes, or services above AKG core.

## Main project pieces

- [Specification](spec/00-introduction.md): the technical contract for the file format.
- [Public API](API.md): the minimal Go reference API for creating, opening, validating, mutating, reading, and compacting files.
- [Conformance guide](conformance.md): how alternate implementations use the fixture corpus.
- [Lifecycle guide](lifecycle.md): the ordinary file lifecycle shown by the runnable example.
- [SDK author guide](sdk-author-guide.md): suggested architecture for building product behavior above core.
- [Repository boundaries](repository-boundaries.md): ownership and scope for current and future repo areas.

## Reference implementation stance

The Go implementation is canonical and minimal. It is the reference behavior and
conformance oracle for AKG v1, but downstream SDKs do not have to import it or
copy its internal architecture. A Rust, TypeScript, Python, or product-specific
implementation can be conformant if it follows the spec and agrees with the
conformance corpus.
