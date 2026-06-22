# Behavior fixtures

This directory contains the shared behavioral contract for all AKG SDKs.

## Files

- **`parity-graph.akg`** — a pre-built AKG file containing a small graph used as the test fixture
- **`parity-spec.json`** — expected query results against that graph; both files are the source of truth for cross-SDK behavioral *read* parity
- **`parity-commit-append.akg`** — the golden output of the canonical commit-append sequence, the source of truth for cross-SDK *write* parity (CONF-3)

## How to use

Write a `behavior_parity` test in your SDK that:

1. Opens `parity-graph.akg` using your SDK's normal open path
2. Runs the queries described in `parity-spec.json`
3. Asserts your results match the expected values

The Go (`sdk/akg-go/behavior_parity_test.go`) and TypeScript (`sdk/akg-ts/test/behavior_parity.test.ts`) tests are reference implementations of this pattern.

## Write parity (`parity-commit-append.akg`)

All three first-party implementations must produce **byte-identical** output on
the write path, not merely logically-equivalent files (CONF-3). The golden
`parity-commit-append.akg` is the result of one canonical sequence applied to a
fresh store with a *constant* clock (every record stamped `1_000_000`):

```
putNode("note","n1",{title:"One"}) ; commit()
putNode("note","n2",{title:"Two"}) ; commit()
```

which must leave the Data and Bloom sections empty and grow the WAL to four
records (two `PUT_NODE` + two `COMMIT`). Each SDK reproduces the sequence and
asserts byte-equality with this golden:

- reference — `internal/store/commit_parity_test.go`
- akg-go — `sdk/akg-go/commit_parity_test.go`
- akg-ts — `sdk/akg-ts/test/commit_parity.test.ts`

Because all three compare against the same golden, matching it proves they are
byte-identical to each other. The same files also assert CONF-3's
no-re-materialization contract directly: a single-record commit must leave a
non-empty Data section unchanged and only append to the WAL.

To regenerate the golden after an intentional write-format change, run any one
SDK's parity test with `WRITE_PARITY_GOLDEN=1` set, then re-run the other two to
confirm they still match.

## Release threshold

A new SDK must pass ≥80% of its behavior parity test cases before official release (v1.0.0). See [`docs/sdk-author-guide.md`](../../docs/sdk-author-guide.md) for details.

## Updating the fixtures

`parity-graph.akg` is built by a reproducible generator,
`sdk/akg-go/gen_behavior_fixture_test.go` (`TestGenBehaviorParityGraph`), which runs through the
akg-go SDK with a pinned clock. Because all first-party SDKs write byte-identically (the
uniform-write-path rule), an akg-go-generated file is exactly what the other SDKs would produce,
so it is a valid shared fixture.

To update, edit all three together:

1. Edit the generator to add/modify nodes or edges, then regenerate:
   `GEN_BEHAVIOR_GRAPH=1 go test -run TestGenBehaviorParityGraph .` (from `sdk/akg-go`)
2. Add the corresponding expected values to `parity-spec.json`
3. Update the `behavior_parity` tests in all SDKs to cover the new assertions, and re-run them in
   every SDK to confirm they still agree
