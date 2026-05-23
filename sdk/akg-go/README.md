# akg-go

Go SDK for reading and writing AKG knowledge graph files.

- Module: `github.com/RobertGumeny/akg/sdk/akg-go`
- Implements AKG v1 independently, without importing the Go Reference SDK — this
  keeps the public API free to expose the full surface an application needs
  (tag lookup, edge traversal, etc.) without being constrained by the Reference
  SDK's intentionally minimal scope.

## Install

```sh
go get github.com/RobertGumeny/akg/sdk/akg-go
```

## Quick start

```go
import akg "github.com/RobertGumeny/akg/sdk/akg-go"

store, err := akg.Open("memory.akg")
if err != nil { ... }
defer store.Close()

alice, err := store.PutNode("person", "alice", akg.NodeFields{
    Title: "Alice",
    Body:  "A researcher.",
}, []string{"active"})

bob, err := store.PutNode("person", "bob", akg.NodeFields{
    Title: "Bob",
}, nil)

err = store.PutEdge(alice, "knows", bob, akg.EdgeFields{})
```

## Getting started

Create a project, write a graph, run it.

**1. Create a project directory and initialize a module:**

```sh
mkdir mygraph && cd mygraph
go mod init mygraph
```

**2. Install akg-go:**

```sh
go get github.com/RobertGumeny/akg/sdk/akg-go
```

**3. Create `main.go`:**

```go
package main

import (
	"fmt"
	akg "github.com/RobertGumeny/akg/sdk/akg-go"
)

func main() {
	store, _ := akg.Open("mygraph.akg")
	defer store.Close()

	alice, _ := store.PutNode("person", "alice", akg.NodeFields{Title: "Alice"}, nil)
	bob, _ := store.PutNode("person", "bob", akg.NodeFields{Title: "Bob"}, nil)
	store.PutEdge(alice, "knows", bob, akg.EdgeFields{})
	store.Commit()

	node, _ := store.GetNode("person", "alice")
	fmt.Printf("node: %s/%s — %q\n", node.Type, node.ID, node.Title)

	edges, _ := store.OutboundEdges(alice, "")
	for _, e := range edges {
		fmt.Printf("  -[%s]-> %s/%s\n", e.Relation, e.To.Type, e.To.ID)
	}
}
```

**4. Run it:**

```sh
go run .
```

**Expected output:**

```
node: person/alice — "Alice"
  -[knows]-> person/bob
```

A `mygraph.akg` file is now in your directory. Open it again later — the graph persists.

## Naming rules

AKG enforces naming constraints on type names, relation names, and tags. Node IDs
follow a separate, more permissive rule.

| Component | Rule |
|---|---|
| Type names | Lowercase `[a-z0-9_]`; no leading, trailing, or consecutive underscores |
| Relation names | Same as type names |
| Tags | Same as type names |
| Node IDs | Any valid UTF-8 string up to 64 characters; colons (`:`) are not allowed |

Node IDs are deliberately more permissive — they may be user-supplied slugs, hex
strings, UUIDs, or anything else that avoids `:`, which is reserved as a key
delimiter. Type names, relation names, and tags share the same stricter rule because
they are used as structural labels in the graph's key space.

Invalid values are rejected at write time with `ErrInvalidInput`.

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

See [Naming rules](#naming-rules) for the constraints on `typeName` and tags.

### Writing edges

```go
err := store.PutEdge(fromRef NodeRef, relation string, toRef NodeRef, fields EdgeFields) error
```

Writes or replaces the edge at `(fromRef, relation, toRef)`. Both referenced
nodes must already exist. See [Naming rules](#naming-rules) for the constraints
on `relation`.

`EdgeFields`:

| Field        | Required | Type            | Default |
|--------------|----------|-----------------|---------|
| `Strength`   | no       | `float64`       | `0.0`   |
| `Confidence` | no       | `*float64`      | nil     |
| `Meta`       | no       | `map[string]any`| nil     |

### Reading

```go
// Returns nil (not an error) if the node does not exist.
node, err := store.GetNode(typeName, id string) (*Node, error)

// Returns all nodes carrying the given tag, sorted by key.
nodes, err := store.ListNodesByTag(tag string) ([]Node, error)

// Returns all nodes, optionally filtered to typeName. Pass "" to return all types.
// An unknown type returns an empty slice and nil error. Results are sorted by key.
nodes, err := store.ListNodes(typeName string) ([]Node, error)

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

## Error handling

Three sentinel errors are exported for callers that need to branch on error type:

| Sentinel | Returned when |
|---|---|
| `akg.ErrNotFound` | A `GetNode`, `DeleteNode`, or `DeleteEdge` call targets a node or edge that does not exist. |
| `akg.ErrInvalidInput` | A caller passes an argument that violates a format or semantic constraint — invalid type name, missing required field, or an operation that would leave the graph inconsistent (e.g. deleting a node that still has live edges). |
| `akg.ErrMissingRequiredField` | A required field is absent. Returned in two situations: (1) a `PutNode` call omits `Title`, or a `PutEdge` call omits a required identity field; (2) a decoded record in a file is structurally valid but missing a required field — callers see this when opening a malformed file written by a buggy writer. |

Use `errors.Is` to test:

```go
node, err := store.GetNode("Person", "alice")
if errors.Is(err, akg.ErrNotFound) {
    // node does not exist
}
```

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
