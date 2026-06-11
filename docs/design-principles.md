# AKG Design Principles

This is the north star for the AKG format. It defines what AKG is, the line between what
the format and official SDKs own and what an implementer owns, and the core design principles
every spec and SDK decision are judged against. If a proposed change cannot be
traced to an explicit design principle or a specific spec section, it does not belong in AKG.
Any changes or updates to these design principles must be earned.

## What AKG is

AKG (Agent Knowledge Graph) is a single-file storage format for knowledge an
agent can carry and query locally, plus official SDKs that give implementers and agents
simple access to it. It descends from three lineages:

- **A progressively-disclosed knowledge base.** Agents reason well about the
  context a task needs and pull only that. AKG keeps this read model and sheds
  the weaknesses of a markdown knowledge base: drift, the lack of typed nodes,
  edges, and relationships, and the dependence on a filesystem of loose files
  (and on git to move them).
- **A native graph.** Typed nodes, typed edges, and relationships are
  first-class data — not relations bolted onto tables.
- **A single file.** The whole graph is one portable file. No server is
  required to read or write it.

AKG is the combination: a native graph, progressively disclosed, in one file.

**Memory is one use, not the only one.** The same primitives serve any knowledge
an agent reads progressively, whether the agent *accumulates* it (memory: writing
its own observations and recalling them later) or *consumes* knowledge *authored
for* it ahead of time. The clearest proof is the official SDKs themselves: each
ships its API documentation as an `.akg`, and its CLI reads that documentation
straight off the file — no network hop and no web search. Accumulated memory and
shipped reference are the same format wearing two hats.

## Operating model

An `.akg` file lives wherever agents live — on a device, at the edge, beside a
small local model. The agent reads and writes it **locally**. A server, when
present, exists only to **persist, sync, and merge** files; it is never in the
live read/write path. The moment a server mediates ordinary reads and writes,
AKG has become a worse version of a database that still requires a server.

Two consequences follow and are load-bearing:

- **Offline-first.** A file is usable with no network and no server.
- **Single-writer-per-file by construction.** Divergence between copies is
  reconciled by **merge**, not prevented by locking.

AKG is to an agent's knowledge what git is to source code: it defines the file
and the merge. It does not define the transport or the host.

## The scope line

AKG provides **primitives, not products.**

| In scope — the format + official SDKs | Out of scope — implementer / orchestrator |
|---|---|
| The single-file binary format (sections, encoding, keys, identity, WAL) | Sync / transport — how a file reaches a device or the merge process |
| Native graph model: typed nodes, typed edges, relationships, versions | Multi-file composition — querying several `.akg` at once |
| Storage primitives: put / get / delete node and edge | Compaction *policy* — when and how often to compact or archive |
| Read primitives for progressive disclosure: traversal, enumeration, recency | The "index node" — an application convention, not a format feature |
| The compaction *operation* (reclaim space, rebuild the bloom filter) | Server architecture, multi-tenancy, query-at-scale, auth, PII handling |
| Crash-atomic durability of a local write | Merge *triggering* — when and how files are sent to merge |
| Merge *semantics* — how two divergent files reconcile | Any product built on AKG |
| SDKs: simple tool / function-call access to all of the above | |

### Why the right-hand column is out of scope

Sync, multi-file composition, and compaction *policy* are all deployment
decisions. They vary per product and are expressible entirely through how an
implementer designs their tool calls and orchestration. Pulling any of them into
the format would force one product's choices onto every other product.

AKG's potential real-world applications are too diverse to design for directly. The format
earns its longevity by staying small and foundational. Everything above the
primitives is the implementer's to build — including patterns that are genuinely
appealing, such as an agent reading several scoped files at once (e.g. a user
file overlaid on a project file). That is a tool-call design choice, not a format
feature.

## Principles

1. **Primitives, not products.** If it is a policy, a transport, or a deployment
   shape, it belongs to the implementer. AKG draws the line at the file and the
   operations on it.
2. **The file lives with the agent; the server only syncs and merges.** A server
   in the live read/write path is a worse database that still needs a server.
3. **One writer per file, by construction.** Offline-first. Reconcile with merge,
   not locks.
4. **Native graph, not relational-graph.** Typed nodes, edges, and relationships
   are first-class.
5. **Reads are traversal-shaped.** Progressive disclosure is the read model:
   enter at a node (or enumerate to build an index), then traverse only the
   neighborhood the task needs. Enumeration exists so an agent can build its own
   index; the format imposes none.
6. **Merge is the headline problem.** It is where AKG's deep design attention
   belongs.
7. **Additive-only, honestly justified.** The on-disk format changes additively
   so an older SDK on one device can still read a file written by a newer SDK on
   another. Old readers skip unknown sections.
8. **Documents over triples.** A node is a substantial unit of memory with typed
   fields, not an atomized subject-predicate-object fragment. AKG models knowledge
   as documents with relationships, not an RDF-style triple store.

## Merge

Merge semantics — how two divergent `.akg` files reconcile into one — is the
deepest design problem in AKG and gets its own focused design pass. The scope
line for it is fixed: **how merge works is in scope; when and how files reach
merge is not.** AKG ships a merge primitive; the orchestrator decides when to
call it and how the two files arrived.

## A note on tone

The specification is a technical document. It describes bytes and operations. It
does not position AKG against RAG, vector databases, graph servers, or any other
tool, except to state plainly where AKG's scope ends and those tools' begins.
There is no thesis to prove, no benchmark to win, and no contrarian framing. A
reader should finish the spec knowing exactly what the format is and how to
operate on it, and nothing about what it is "better" than.
