---
name: "prep-for-release"
description: "Prepare a hotfix or feature release for one or both SDKs. Identifies which SDK(s) are affected, writes a CHANGELOG entry, commits, and tags. SDKs are versioned independently from the root module."
---

# Release Prep Workflow

This repo has three independently versioned units:

| Unit | Version mechanism | Tag format |
|---|---|---|
| `sdk/akg-go/` | git tag only (no version field in go.mod) | `sdk/akg-go/vX.Y.Z` |
| `sdk/akg-ts/` | `package.json` `"version"` + git tag | `sdk/akg-ts/vX.Y.Z` |
| Root (`akg.go`, `internal/`) | git tag (spec/reference impl only) | `vX.Y.Z` |

## Phase 1: Identify the affected SDK(s)

Run `git log --oneline -10` and `git show <commit> --stat` for any commits since the last release tag to see which paths changed.

- Changes under `sdk/akg-go/` → Go SDK release
- Changes under `sdk/akg-ts/` → TypeScript SDK release
- Changes under `internal/`, `akg.go`, or `docs/spec/` → Root release

Ask the user to confirm which SDK(s) need a version bump if it is not obvious.

## Phase 2: Determine the next version

Look at the most recent tag for the affected SDK (e.g. `git tag --list 'sdk/akg-go/*' | sort -V | tail -1`).

- **Hotfix** (bug fix, no new API): bump patch (Z)
- **Minor feature** (new API, backwards-compatible): bump minor (Y), reset patch
- **Breaking change**: bump major (X), reset minor and patch

State the resulting version explicitly before proceeding (e.g. last tag `sdk/akg-go/v0.1.3` + a minor feature → `v0.2.0`), and confirm it with the user if the change type is at all ambiguous.

## Phase 3: Write the CHANGELOG entry

Each SDK's `CHANGELOG.md` accumulates changes under a `## Unreleased` heading as work lands. Releasing means stamping that section with the version. There must end up being exactly one `## vX.Y.Z` section at the top, directly below the `# Changelog` header.

- **If a `## Unreleased` section exists** (the normal case): rename it to `## vX.Y.Z`. Review its entries against the diff, tidy them, and do **not** also leave behind an empty `## Unreleased`.
- **If there is no `## Unreleased` section**: add a new `## vX.Y.Z` section below the `# Changelog` header.

Never leave two sections for the same version, and never leave both a stamped version and an empty `## Unreleased`.

Group entries under these subsections, omitting any that are empty:
- `### Added` — new public API or behavior
- `### Fixed` — bug fixes; name the symptom, the root cause, and the change
- `### Changed` — non-breaking behavioral changes
- `### Removed` — deprecated items removed

Derive every entry from the diff (`git show <commit>`, or `git log <last-tag>..HEAD` for the full range) — do not ask the user to describe the changes.

## Phase 4: Bump the version (TypeScript only)

For `sdk/akg-ts/`, update `"version"` in `sdk/akg-ts/package.json` to match the new version string.

Go SDK has no version field — skip this step.

## Phase 5: Commit and tag

Stage, commit, and tag **only** the SDK(s) you are releasing. Use the block that matches.

**Releasing akg-go:**

```sh
git add sdk/akg-go/CHANGELOG.md
git commit -m "chore: release prep for akg-go vX.Y.Z"
git tag sdk/akg-go/vX.Y.Z
```

**Releasing akg-ts** — also stage `package.json`, which carries the version bump from Phase 4:

```sh
git add sdk/akg-ts/CHANGELOG.md sdk/akg-ts/package.json
git commit -m "chore: release prep for akg-ts vX.Y.Z"
git tag sdk/akg-ts/vX.Y.Z
```

**Releasing both at once:** stage both file sets, make a single commit (`chore: release prep for akg-go vX.Y.Z and akg-ts vX.Y.Z`), then create both tags.

Each tag must use its SDK's full prefix (`sdk/akg-go/` or `sdk/akg-ts/`), so the version matches that SDK's existing tag series.

Do **not** push — pushing is the user's call (Phase 6).

## Phase 6: Confirm

Tell the user:
- Which SDK(s) were bumped and to what version
- The tag(s) created
- That they push when ready with `git push && git push --tags` — `git push` alone does not push tags, and `git push --tags` alone does not push the release commit, so both are needed
