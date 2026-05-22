# akg-go

Go SDK for reading and writing AKG knowledge graph files.

- Module: `github.com/RobertGumeny/akg-go`
- Implements AKG v1 independently, without importing the Go Reference SDK — this
  keeps the public API free to expose the full surface an application needs
  (tag lookup, edge traversal, etc.) without being constrained by the Reference
  SDK's intentionally minimal scope.

## Install

```sh
go get github.com/RobertGumeny/akg-go
```

## Quick start

```go
import akg "github.com/RobertGumeny/akg-go"

store, err := akg.Open("memory.akg")
if err != nil { ... }
defer store.Close()

alice, err := store.PutNode("Person", "alice", akg.NodeFields{
    Title: "Alice",
    Body:  "A researcher.",
}, []string{"active"})

bob, err := store.PutNode("Person", "bob", akg.NodeFields{
    Title: "Bob",
}, nil)

err = store.PutEdge(alice, "knows", bob, akg.EdgeFields{})
```

## API

### Opening a store

```go
store, err := akg.Open(path string) (*Store, error)
```

Opens an existing `.akg` file or creates a new empty one if the path does not
exist. Returns an error if the file exists but is malformed.

### Writing nodes

```go
ref, err := store.PutNode(typeName, id string, fields NodeFields, tags []string) (NodeRef, error)
```

Writes or replaces the node at `(typeName, id)`. If `id` is empty, a new ID is
generated. Returns a `NodeRef` you can pass directly to `PutEdge`.

`NodeFields`:

| Field   | Required | Type            |
|---------|----------|-----------------|
| `Title` | yes      | `string`        |
| `Body`  | no       | `string`        |
| `Meta`  | no       | `map[string]any`|

Tags must be lowercase `[a-z0-9_]` with no leading, trailing, or consecutive
underscores. Invalid tags are rejected at write time.

### Writing edges

```go
err := store.PutEdge(fromRef NodeRef, relation string, toRef NodeRef, fields EdgeFields) error
```

Writes or replaces the edge at `(fromRef, relation, toRef)`. Both referenced
nodes must already exist.

`EdgeFields`:

| Field        | Required | Type            | Default |
|--------------|----------|-----------------|---------|
| `Strength`   | no       | `float64`       | `0.5`   |
| `Confidence` | no       | `*float64`      | nil     |
| `Meta`       | no       | `map[string]any`| nil     |

### Reading

```go
// Returns nil (not an error) if the node does not exist.
node, err := store.GetNode(typeName, id string) (*Node, error)

// Returns all nodes carrying the given tag, sorted by key.
nodes, err := store.ListNodesByTag(tag string) ([]Node, error)

// Pass an empty relation to return all edges regardless of relation.
edges, err := store.OutboundEdges(nodeRef NodeRef, relation string) ([]Edge, error)
edges, err := store.InboundEdges(nodeRef NodeRef, relation string) ([]Edge, error)
```

### Committing and closing

```go
err := store.Commit() // durably writes all pending mutations
err := store.Close()  // commits outstanding mutations and closes the store
```

Always close a store when done. `Close` is safe to call on a store with no
pending mutations.

## NodeRef

`PutNode` returns a `NodeRef`:

```go
type NodeRef struct {
    Type string `json:"type"`
    ID   string `json:"id"`
}
```

This shape is part of the public SDK contract and is identical across the Go and
TypeScript SDKs, including field names and JSON keys. `NodeRef` values are safe
to serialize and pass between systems.

## Run the example

```sh
go run ./examples/basic
```

See [`examples/basic/README.md`](examples/basic/README.md) for expected output.
