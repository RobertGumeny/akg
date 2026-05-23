---
title: AKG SDK author guide
status: release-candidate docs
---

# AKG SDK author guide

This guide is for anyone implementing AKG support in a new language.

## What you are building

An AKG SDK implements the format layer: open, read, write, commit, and compact
`.akg` files. That is the whole job. Product behavior — ingestion pipelines,
retrieval policy, naming conventions, retention rules — belongs in the product,
not the SDK.

## How to implement it

Follow the [v1 specification](spec/00-introduction.md). The spec defines the
on-disk format, encoding rules, validation requirements, and interoperability
constraints. Your internal architecture is your business; the spec only constrains
what you read and write.

## How to verify it

Run your implementation against the conformance fixtures in `testdata/conformance/`.
The `manifest.json` describes which `.akg` files must be accepted and which must
be rejected. Passing those tests is the compatibility contract — you do not need
to import any Go code or match any Go internals.

See the [Conformance guide](conformance.md) for setup details.

## The Go Reference SDK

The Go code in this repo (`github.com/RobertGumeny/akg`) exists to prove
the spec works. Read it as a behavior target — what should happen when you open a
file, replay a WAL, or compact. Do not treat it as a blueprint for your own
internal structure, and do not import it.

If you are building in Go specifically, use [akg-go](../sdk/akg-go/README.md)
instead. It is the production Go SDK with the full public API.
