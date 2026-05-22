# basic

A minimal end-to-end example of the AKG Go SDK. Copy this as a starting point for your own programs.

## What it shows

- Opening (or creating) a store at a file path
- Writing nodes with fields, tags, and metadata
- Writing typed edges between nodes
- Closing and reopening the store to confirm durability
- Reading a node by type + ID (`GetNode`)
- Listing all nodes that carry a tag (`ListNodesByTag`)
- Walking outbound edges from a node (`OutboundEdges`)

## Run it

From `sdk/akg-go/`:

```
go run ./examples/basic
```

Expected output:

```
Node: Person/alice — "Alice"
  body: A researcher in knowledge graphs.
  tags: [active, researcher]
  meta: map[role:lead]

Active people:
  Person/alice — "Alice"
  Person/bob — "Bob"

Outbound edges from Person/alice:
  -[authored]-> Paper/paper-001 (strength 1.0)
  -[collaborates-with]-> Person/bob (strength 0.5)
```

The store is written to a temp file (`$TMPDIR/akg-basic-example.akg`) and cleaned up at the start of each run, so repeated runs always produce the same output.
