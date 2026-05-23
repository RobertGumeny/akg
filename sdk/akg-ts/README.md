# akg-ts

TypeScript SDK for reading and writing AKG knowledge graph files.

- Package: `akg-ts`
- Implements AKG v1 with a full public API — tag lookup, edge traversal, and everything you need to build on top of AKG.

## Install

```sh
npm install akg-ts
```

## Quick start

```typescript
import { open } from 'akg-ts';

const store = await open('memory.akg');

const alice = store.putNode('person', 'alice', {
  title: 'Alice',
  body: 'A researcher.',
}, ['active', 'researcher']);

const bob = store.putNode('person', 'bob', {
  title: 'Bob',
}, []);

store.putEdge(alice, 'knows', bob, {});
await store.close();
```

## Getting started

Create a project, write a graph, run it.

**1. Create a project directory and initialize npm:**

```sh
mkdir mygraph && cd mygraph
npm init -y
```

**2. Install akg-ts:**

```sh
npm install akg-ts
```

**3. Create `index.ts`:**

```typescript
import { open } from 'akg-ts';

const store = await open('mygraph.akg');
const alice = store.putNode('person', 'alice', { title: 'Alice' }, []);
const bob = store.putNode('person', 'bob', { title: 'Bob' }, []);
store.putEdge(alice, 'knows', bob, {});
await store.commit();

const node = store.getNode('person', 'alice')!;
console.log(`node: ${node.type}/${node.id} — "${node.title}"`);

const edges = store.outboundEdges(alice);
for (const e of edges) {
  console.log(`  -[${e.relation}]-> ${e.to.type}/${e.to.id}`);
}
await store.close();
```

**4. Run it:**

```sh
npx tsx index.ts
```

**Expected output:**

```
node: person/alice — "Alice"
  -[knows]-> person/bob
```

A `mygraph.akg` file is now in your directory. Open it again later — the graph persists.

## Naming rules

AKG enforces naming constraints on type names, relation names, and tags. Node IDs follow a separate, more permissive rule.

| Component | Rule |
|---|---|
| Type names | Lowercase `[a-z0-9_]`; no leading, trailing, or consecutive underscores |
| Relation names | Same as type names |
| Tags | Same as type names |
| Node IDs | Any valid UTF-8 string up to 64 characters; colons (`:`) are not allowed |

Node IDs are deliberately more permissive — they may be user-supplied slugs, hex strings, UUIDs, or anything else that avoids `:`, which is reserved as a key delimiter. Type names, relation names, and tags share the same stricter rule because they are used as structural labels in the graph's key space.

Invalid values are rejected at write time with `InvalidInputError`.

## API

### Opening a store

```typescript
import { open } from 'akg-ts';

const store = await open(path: string): Promise<Store>
```

Opens an existing `.akg` file or creates a new empty one if the path does not exist. Returns an error if the file exists but is malformed.

### Writing nodes

```typescript
const ref: NodeRef = store.putNode(typeName: string, id: string, fields: NodeFields, tags: string[]): NodeRef
```

Writes or replaces the node at `(typeName, id)`. If `id` is empty, a new ID is generated. Returns a `NodeRef` you can pass directly to `putEdge`. Throws synchronously on validation errors.

`NodeFields`:

| Field   | Required | Type                        |
|---------|----------|-----------------------------|
| `title` | yes      | `string`                    |
| `body`  | no       | `string`                    |
| `meta`  | no       | `Record<string, unknown>`   |

See [Naming rules](#naming-rules) for the constraints on `typeName` and tags.

### Writing edges

```typescript
store.putEdge(fromRef: NodeRef, relation: string, toRef: NodeRef, fields: EdgeFields): void
```

Writes or replaces the edge at `(fromRef, relation, toRef)`. Both referenced nodes must already exist. See [Naming rules](#naming-rules) for the constraints on `relation`.

`EdgeFields`:

| Field        | Required | Type                      | Default |
|--------------|----------|---------------------------|---------|
| `strength`   | no       | `number`                  | `0`     |
| `confidence` | no       | `number \| null`          | `null`  |
| `meta`       | no       | `Record<string, unknown>` | `{}`    |

### Reading

```typescript
// Returns null (not an error) if the node does not exist.
const node: Node | null = store.getNode(typeName: string, id: string)

// Returns all nodes carrying the given tag, sorted by key.
const nodes: Node[] = store.listNodesByTag(tag: string)

// Returns all nodes, optionally filtered to typeName. Pass no argument to return all types.
// An unknown type returns an empty array. Results are sorted by key.
const nodes: Node[] = store.listNodes(typeName?: string)

// Pass no relation to return all edges regardless of relation.
const edges: Edge[] = store.outboundEdges(nodeRef: NodeRef, relation?: string)
const edges: Edge[] = store.inboundEdges(nodeRef: NodeRef, relation?: string)
```

### Committing and closing

```typescript
await store.commit(): Promise<void>  // durably writes all pending mutations
await store.close(): Promise<void>   // commits outstanding mutations and closes the store
```

Always close a store when done. `close` is safe to call on a store with no pending mutations.

## Error handling

Three error classes are exported for callers that need to branch on error type:

| Class | Thrown when |
|---|---|
| `NotFoundError` | A `deleteNode` or `deleteEdge` call targets a node or edge that does not exist. |
| `InvalidInputError` | A caller passes an argument that violates a format or semantic constraint — invalid type name, missing required field, or an operation that would leave the graph inconsistent (e.g. deleting a node that still has live edges). |
| `MissingRequiredFieldError` | A required field is absent. Returned in two situations: (1) a `putNode` call omits `title`; (2) a decoded record in a file is structurally valid but missing a required field — callers see this when opening a malformed file written by a buggy writer. |

Use `instanceof` to test:

```typescript
import { NotFoundError } from 'akg-ts';

try {
  store.deleteNode('person', 'alice');
} catch (err) {
  if (err instanceof NotFoundError) {
    // node does not exist
  }
}
```

Note: `getNode` returns `null` rather than throwing `NotFoundError` for missing nodes.

## NodeRef

`putNode` returns a `NodeRef`:

```typescript
interface NodeRef {
  type: string;
  id: string;
}
```

This shape is part of the public SDK contract and is identical across the Go and TypeScript SDKs, including field names and JSON keys. `NodeRef` values are safe to serialize and pass between systems.

## Non-goals

AKG does not provide a query language, server, semantic search, or multi-writer sync. See the [root README](../../README.md#non-goals) for the full list.

## Run the example

```sh
npx tsx examples/basic.ts
```
