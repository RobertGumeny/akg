# AKG — Agent Knowledge Graph File Format

[![CI](https://github.com/RobertGumeny/akg/actions/workflows/ci.yml/badge.svg)](https://github.com/RobertGumeny/akg/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/RobertGumeny/akg/sdk/akg-go.svg)](https://pkg.go.dev/github.com/RobertGumeny/akg/sdk/akg-go)
[![npm](https://img.shields.io/npm/v/akg-ts)](https://www.npmjs.com/package/akg-ts)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

AKG is a file format for a knowledge graph — open it, read and write nodes and edges, close it. No server, no query language, no setup.

Think of it as SQLite for agent memory: a portable, single-file graph your agent can open, update, and carry across tools and hosts.

## Non-goals

AKG is deliberately narrow. It does not provide:

- **Query language or traversal planner** — there is no AQL, Cypher, or graph analytics layer. Traversal is done in your application code.
- **Server or background service** — AKG is a file, not a daemon. One writer at a time.
- **Semantic search or embeddings** — no vector index, no ranking, no retrieval pipeline.
- **Multi-writer sync or conflict resolution** — AKG is single-writer. Merging concurrent mutations is out of scope.

If any of those are hard requirements, AKG is not the right tool.

## Quick start

**Go**

```go
import akg "github.com/RobertGumeny/akg/sdk/akg-go"

store, err := akg.Open("memory.akg")
if err != nil { log.Fatal(err) }
defer store.Close()

alice, _ := store.PutNode("person", "alice", akg.NodeFields{Title: "Alice"}, nil)
bob, _ := store.PutNode("person", "bob", akg.NodeFields{Title: "Bob"}, nil)
store.PutEdge(alice, "knows", bob, akg.EdgeFields{})
store.Commit()

node, _ := store.GetNode("person", "alice")
fmt.Println(node.Title) // Alice
```

**TypeScript**

```typescript
import { open } from 'akg-ts';

const store = await open('memory.akg');
const alice = store.putNode('person', 'alice', { title: 'Alice' }, []);
const bob = store.putNode('person', 'bob', { title: 'Bob' }, []);
store.putEdge(alice, 'knows', bob, {});
await store.commit();

const node = store.getNode('person', 'alice');
console.log(node.title); // Alice
await store.close();
```

## Who this is for

**Building an app in Go?** Use the [akg-go SDK](sdk/akg-go/README.md). It is the production Go library with the full public API — tag lookup, edge traversal, and everything you need to build on top of AKG.

**Building an app in TypeScript?** Use the [akg-ts SDK](sdk/akg-ts/README.md). It exposes an identical graph API with `async/await` I/O and full TypeScript types.

**Implementing AKG in another language?** Start with the [v1 specification](docs/spec/00-introduction.md) and the [conformance guide](docs/conformance.md). The conformance fixtures in `testdata/conformance/` are your compatibility contract — you do not need to import or copy any Go code. The Go Reference SDK in this repo exists to prove the spec works and to give you a concrete behavior target; study it, but do not treat it as a blueprint for your own internal architecture.

## Repo contents

| Path | What it contains |
|---|---|
| `docs/spec/` | v1 binary format specification |
| `docs/lifecycle.md` | File lifecycle: create, mutate, commit, compact |
| `docs/conformance.md` | How to run conformance tests from another implementation |
| `docs/sdk-author-guide.md` | Implementing AKG support in a new language |
| `sdk/akg-go/` | Go SDK — full public API for application development |
| `sdk/akg-ts/` | TypeScript SDK — full public API for application development |
| `testdata/conformance/` | Conformance fixtures (accepted and rejected files) |
| `examples/` | Runnable format lifecycle examples |

## Contributing

Run the full test suite (conformance fixtures included):

```sh
go test -count=1 ./...
```

Run the Go lifecycle example:

```sh
go run ./examples/lifecycle
```

See [`examples/lifecycle/README.md`](examples/lifecycle/README.md) for expected output.
