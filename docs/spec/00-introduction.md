# Introduction

AKG is a structured, single-file knowledge graph format for AI agents. It defines a portable binary representation for graph-shaped working memory: nodes that capture durable facts or artifacts, edges that capture explicit relationships between them, and the file structures required to store, validate, and recover that data reliably.

The format exists to provide persistent, portable, and inspectable working memory for agents. AKG is intended for data that should survive process boundaries, model switches, host changes, and implementation changes. An AKG file can be written by one implementation, inspected independently, and read by another implementation without requiring shared infrastructure or a running service.

This specification is written for SDK authors implementing AKG in any language. It defines the on-disk format, encoding rules, validation requirements, and interoperability constraints required for conformant readers and writers. Application APIs, higher-level retrieval strategies, and product-specific memory policies are outside the scope of the format unless stated otherwise.

AKG is designed around a small set of principles:

- Structure is preferred over fuzzy retrieval. AKG stores explicit records and explicit relations.
- Documents are preferred over triples. Nodes are substantial units of memory with typed fields, not atomized subject-predicate-object fragments.
- Agents name what they want. The format is optimized for exact identifiers, typed scans, tags, and graph traversal rather than approximate semantic recall.
- Embeddings are not part of the format.
- Vector search is not part of the format.

These constraints are intentional. AKG is a format for durable agent memory, not a general-purpose similarity engine.

Accordingly, AKG is not any of the following:

- a vector database
- a conversation store
- a replacement for application databases
- an MCP server

A conformant implementation may be used alongside those systems, but the AKG format does not attempt to subsume them.

The reference implementation for AKG is written in Go. In Phase 1, its scope is limited to the format layer: reading, writing, compaction, WAL replay during ordinary open, and explicit recovery tooling. AKG v1 uses an accumulating WAL between compactions rather than rewriting the full file on every commit. The reference implementation lives alongside the conformance corpus, which serves as the cross-implementation test set for format behavior.