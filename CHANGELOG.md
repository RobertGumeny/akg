# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-06-11

This release aligns the authoritative spec with the recovered design vision and
hardens the Go Reference SDK to match the behavior all first-party SDKs now share.

### Changed

- **⚠️ Potentially breaking — key-component validation unified (spec `01`, `04`).** Type, relation, tag, and
  node-id components now reduce to a single key-safety rule: non-empty, valid UTF-8,
  no `:` delimiter, and at most 64 UTF-8 bytes. The previous snake_case
  (`[a-z0-9_]`) restriction on type/relation/tag is dropped — casing and
  word-separation are an SDK-level convention, not a format rule. The node-id length
  cap changes from runes/codepoints to UTF-8 bytes, and the 64-byte cap is newly
  applied to type/relation/tag (previously uncapped). Oversize keys are rejected as
  `malformed_key`. **Migration impact:** any type/relation/tag longer than 64 UTF-8
  bytes, or a node id that fit under the old code-point limit but exceeds 64 UTF-8
  bytes, that was accepted before v0.2.0 will now be rejected. The change only
  *adds* validation, so previously well-formed short keys are unaffected.
- **Merge contract settled (spec `08`).** Edge identity is the full 5-tuple, with
  neutral last-writer resolution and scalar-version detection.
- **WAL section on compacted files (spec `05`).** A compacted file carries a
  zero-length WAL section rather than omitting the section entirely, so an
  incremental commit can append onto it.
- **Reference SDK write path is logical-append.** `Commit` appends only the new
  mutation records plus a `COMMIT` marker, reusing the existing `Data`/`Bloom`
  bytes instead of re-materializing the whole `Data` section every commit;
  `Compact` re-establishes the baseline. This matches both application SDKs.
- **Spec prose realigned** with the recovered design vision — introduction,
  data model, and appendix (`00`, `01`, `09`).

### Fixed

- **Reference SDK commit is crash-atomic.** Durable writes now go through a
  same-directory temp file that is fsynced, renamed over the target, and followed
  by a directory fsync, replacing the in-place `O_TRUNC` rewrite. An interrupted
  commit can no longer tear a previously committed `.akg` store. File permissions
  are preserved across writes.
- **Edge identity corrected to the 5-tuple** in the reference SDK and the merge
  spec section, fixing earlier 3-tuple drift.

### Added

- **Byte-parity and write-path conformance coverage** — a shared
  `parity-commit-append.akg` golden plus a `malformed_key` conformance category and
  UTF-8 key fixtures, asserting the reference SDK and both application SDKs are
  byte-identical on commit.

## [0.1.0] - 2026-05-22

### Added

- The AKG v1 binary format specification lives in `docs/spec/` — this is the authoritative contract for all implementations.
- A Go Reference SDK (`akg.go`, `internal/`) that proves the spec is implementable end-to-end.
- A Go application SDK (`sdk/akg-go/`) with a full public API: `Open`, `PutNode`, `PutEdge`, `GetNode`, `ListNodes`, `ListNodesByTag`, `OutboundEdges`, `InboundEdges`, `DeleteNode`, `DeleteEdge`, `Commit`, `Close`.
- A TypeScript application SDK (`sdk/akg-ts/`) with the same API surface using async/await.
- 36 conformance fixtures in `testdata/conformance/` that cover the full range of valid files, malformed-file rejection, WAL replay, compaction, and edge deletion. These are the correctness contract for any future SDK port.
- A conformance guide (`docs/conformance.md`) and SDK author guide (`docs/sdk-author-guide.md`) for anyone who wants to implement AKG in another language.
- Apache 2.0 license.