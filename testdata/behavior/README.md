# Behavior fixtures

This directory contains the shared behavioral contract for all AKG SDKs.

## Files

- **`parity-graph.akg`** — a pre-built AKG file containing a small graph used as the test fixture
- **`parity-spec.json`** — expected query results against that graph; both files are the source of truth for cross-SDK behavioral parity

## How to use

Write a `behavior_parity` test in your SDK that:

1. Opens `parity-graph.akg` using your SDK's normal open path
2. Runs the queries described in `parity-spec.json`
3. Asserts your results match the expected values

The Go (`sdk/akg-go/behavior_parity_test.go`) and TypeScript (`sdk/akg-ts/test/behavior_parity.test.ts`) tests are reference implementations of this pattern.

## Release threshold

A new SDK must pass ≥80% of its behavior parity test cases before official release (v1.0.0). See [`docs/sdk-author-guide.md`](../../docs/sdk-author-guide.md) for details.

## Updating the fixtures

If you add a new feature that requires new assertions, update both files together:

1. Modify `parity-graph.akg` to include any new nodes/edges needed
2. Add the corresponding expected values to `parity-spec.json`
3. Update the behavior parity tests in all existing SDKs to cover the new assertions
