---
title: AKG v1 specification introduction
status: v1 draft
---

# Introduction

AKG is a structured, single-file knowledge graph format for AI agents. It defines a portable binary representation for graph-shaped working memory: nodes that capture durable facts or artifacts, edges that capture explicit relationships between them, and the file structures required to store, validate, and recover that data reliably.

Knowledge graphs are already useful for agent context, but they are often bound to graph servers, framework-specific stores, or app-specific schemas. AKG makes the knowledge graph a portable file an agent can carry with it.

The format exists to provide persistent, portable, and inspectable working memory for agents. AKG is intended for data that should survive process boundaries, model switches, host changes, and implementation changes. An AKG file can be written by one implementation, inspected independently, and read by another implementation without requiring shared infrastructure or a running service.

This specification is written for SDK authors implementing AKG in any language. It defines the on-disk format, encoding rules, validation requirements, and interoperability constraints required for conformant readers and writers. Application APIs, higher-level retrieval strategies, and product-specific memory policies are outside the scope of the format unless stated otherwise.

AKG is designed around a small set of principles:

- Portable graph state is the core abstraction. AKG stores explicit records and explicit relations in a file that can move across tools and hosts.
- Documents are preferred over triples. Nodes are substantial units of memory with typed fields, not atomized subject-predicate-object fragments.
- Exact structure is a first-class access path. The format supports identifiers, typed scans, tags, and graph traversal; higher-level systems may add semantic recall or ranking above it.
- Embeddings are not required for format compatibility.
- Vector indexes are not part of the core on-disk format.

These constraints are intentional. AKG is a format for durable agent memory, not a general-purpose similarity engine. It can be used alongside RAG, embeddings, vector search, graph servers, or application databases when those are the right tools for a larger system.

Accordingly, AKG is not any of the following:

- a vector database
- a conversation store
- a replacement for application databases
- an MCP server

A conformant implementation may be used alongside those systems, but the AKG format does not attempt to subsume them.

The Go Reference SDK for AKG lives alongside this specification. In Phase 1, its scope is limited to the format layer: reading, writing, compaction, WAL replay during ordinary open, and explicit recovery tooling. AKG v1 uses an accumulating WAL between compactions rather than rewriting the full file on every commit. The Reference SDK lives alongside the conformance test suite, which is the cross-implementation test set for format behavior.