---
title: AKG lifecycle example
status: release-candidate docs
---

# AKG lifecycle example

This is a tiny core-format example, not an SDK or product workflow. It uses only
the documented root package API to:

1. create an AKG file;
2. add three nodes and two edges;
3. commit and reopen the file;
4. read records back with exact lookups and whole-state lists;
5. compact and validate the result.

Run it from a clean checkout:

```sh
go run ./examples/lifecycle
```

By default the program writes a temporary `.akg` file and prints a readable node
summary. To keep the output file, pass a path that does not already exist:

```sh
go run ./examples/lifecycle /tmp/akg-lifecycle.akg
```
