---
title: AKG overview
status: release-candidate docs
---

# AKG overview

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

## What this repo contains

- the v1 binary file format and validation rules;
- node, edge, key, section, Bloom, WAL, and compaction semantics;
- a conformance test suite of accepted and rejected `.akg` files;
- a Go Reference SDK that lives alongside the spec to prove it works and give
  implementers a concrete behavior target;
- official SDKs in Go and TypeScript (with more planned);
- examples that demonstrate format lifecycle operations.

## What does not belong here

- memory-file ingestion or agent workflow policy;
- vector search, ranking, embeddings, or semantic retrieval pipelines;
- a query planner, traversal language, or graph analytics layer;
- merge/conflict resolution, multi-writer behavior, or background services.

Those features can be valuable and complementary, but they belong in products
built on top of AKG, not in the format or the SDKs.

## Main project pieces

- [Specification](spec/00-introduction.md): the technical contract for the file format.
- [Public API](API.md): the Go Reference SDK API — minimal by design, scoped to format operations.
- [Conformance guide](conformance.md): how to run the conformance tests from another implementation.
- [Lifecycle guide](lifecycle.md): the ordinary file lifecycle shown by the runnable example.
- [SDK author guide](sdk-author-guide.md): implementing AKG support in a new language.
- [Repository boundaries](repository-boundaries.md): ownership and scope for current and future repo areas.

## Go Reference SDK

The Go code in this repo (`akg.go`, `internal/`, `cmd/`) is the Reference SDK.
It is intentionally minimal — its job is to demonstrate what correct format
behavior looks like, not to be the most ergonomic Go library for building apps.

If you are building a Go application, use the
[akg-go SDK](../sdk/akg-go/README.md) instead. It has the full public API.

If you are implementing AKG in another language, use the spec and the conformance
fixtures as your contract. The Reference SDK is there for you to read and run;
you do not need to import it or copy its internal structure. Any implementation
is conformant if it follows the spec and passes the conformance tests.
