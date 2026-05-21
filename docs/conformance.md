---
title: AKG conformance guide
status: release-candidate docs
---

# AKG conformance guide

The conformance corpus is the shared test set for AKG implementations. It lives
in [`../testdata/conformance`](../testdata/conformance) and is described by a
machine-readable `manifest.json`.

Use it when writing a reader, writer, validator, SDK, or independent tooling in
another language. You should not need to read Go test source to understand the
expected behavior of each fixture.

## How to use the corpus

For each manifest entry:

1. read the fixture named by `path`;
2. run your ordinary AKG open/validate path;
3. check `expected_result`:
   - `accept`: the file should open normally;
   - `reject`: the file should be refused;
4. for rejections, compare the broad `expected_error_category`, not the exact Go
   error string.

The fixture README documents the manifest fields and safe update workflow:
[`../testdata/conformance/README.md`](../testdata/conformance/README.md).

## Reference checks

From this repository, run:

```sh
go test -count=1 ./internal/format ./internal/store
go run ./internal/cmd/conformance-fixtures -dir testdata/conformance
```

Before sharing a release candidate, run the full suite:

```sh
go test -count=1 ./...
```

## What conformance proves

The corpus checks that implementations agree on release-critical format behavior:
accepted examples open, malformed files fail closed, committed WAL is handled
correctly, invalid checksums are rejected, and derived-key consistency is
validated.

Conformance does not require another SDK to import the Go package or expose the
same public API. The contract is the AKG spec plus the fixture expectations.
