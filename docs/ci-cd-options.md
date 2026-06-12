# CI/CD Options — Analysis & Recommendation

> Status: proposal for discussion (2026-06-12). No workflows changed yet.
> Decision captured so far: **tag-triggered publish** for releases; pursue all four
> goals (release automation, pre-merge bug-catching, supply-chain/security, speed).

## 0. Decisions (locked 2026-06-12)

1. **Go matrix** — keep `go.mod` floor at **1.22**, test **`1.22` + `1.26`** (widest consumer
   reach; matrix is ~free on a public repo).
2. **npm publishing** — **OIDC trusted publishing** with `--provenance`. No stored
   `NPM_TOKEN`; requires a one-time trusted-publisher setup on npmjs.com and npm CLI ≥ 11.5.1
   in CI.
3. **Coverage** — **artifact-only** (`go test -coverprofile` uploaded as an artifact; no
   third-party service/token).
4. **Lint** — **blocking from day one** (`staticcheck` for Go, ESLint for TS). Codebase is
   small and clean, so no advisory grace period.

Implied by the above: Actions/CodeQL/Dependabot are free (public repo); a required-checks
set + branch protection becomes the enforcement surface for the matrix, conformance, lint,
and release dry-run.

## 1. Where we are today

A single workflow, `.github/workflows/ci.yml`, runs on every push and PR with two jobs:

- **Go** — `go test ./...` on root, regenerate Go SDK docs graph + fail on drift,
  then build / test / `vet` `sdk/akg-go`.
- **TypeScript** — `npm ci`, build, docs-graph freshness check, `vitest`, `tsc --noEmit`.

The repo is **three independently-versioned units**:

| Unit | Module / package | Tag prefix | Published to |
|---|---|---|---|
| Reference SDK (root) | `github.com/RobertGumeny/akg` | `vX.Y.Z` | Go module proxy |
| Go application SDK | `github.com/RobertGumeny/akg/sdk/akg-go` | `sdk/akg-go/vX.Y.Z` | Go module proxy |
| TypeScript SDK | npm `akg-ts` | `sdk/akg-ts/vX.Y.Z` | npm registry |

Releases are manual: the `prep-for-release` skill writes the changelog, commits, and
tags; the actual publish (npm, Go proxy warm-up) is by hand.

## 2. Gaps

| # | Gap | Why it matters |
|---|---|---|
| G1 | Go CI pins **1.22 only**; `go.mod` says `1.22` but dev is on `1.26`. No matrix. | A `1.26`-only construct passes locally + CI, breaks for a `1.22` consumer. We don't test what users run. |
| G2 | **No release automation** — riskiest step (publish) has zero guardrails. | Easy to publish an unbuilt/untested artifact or skip a step. Three modules multiplies the chance. |
| G3 | **No caching** (`cache: false`, no npm cache). | Slower, more minutes — cheap to fix. |
| G4 | **Thin linting** — `go vet` on SDK only; no `gofmt`/`staticcheck`; no ESLint for TS. | Style/correctness drift unflagged. |
| G5 | **Conformance/parity not a named gate** — runs inside `go test`, not surfaced. | The "correctness contract" (AGENTS.md) isn't a first-class signal. |
| G6 | **No security scanning** — no `govulncheck`, `npm audit`, Dependabot, CodeQL. | No visibility into vulnerable deps for *published* libraries. |
| G7 | **No coverage reporting.** | Can't see test-surface regressions. |
| G8 | **No concurrency cancellation / path filters.** | Stacked pushes run redundant jobs. |
| G9 | `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24` env var — a Node20-deprecation workaround. | Fragile; pin action versions instead. |
| G10 | No CODEOWNERS / documented required-checks set. | Green CI can be bypassed. |

## 3. Tiered options for pre-merge CI

Three tiers, cumulative. Pick a stopping point.

### Tier A — Hardening (low effort, high value)
- Add **Go version matrix**: `1.22` (go.mod floor) **and** `1.26` (dev). Optionally `stable`.
- Turn **caching on** (`actions/setup-go` cache, `actions/setup-node` npm cache). (G3)
- Add **`gofmt -l` check** and **`go vet ./...` on root** too. (G4)
- Add **concurrency block** to cancel superseded runs + **path filters** so doc-only
  pushes skip heavy jobs. (G8)
- Pin actions / drop the Node24 force-flag. (G9)

*Cost: ~half a day. Risk: near zero. This is the "just do it" baseline.*

### Tier B — Confidence (medium effort)
- Promote **conformance + behavior-parity to a named, required job** so the contract
  is a visible check, not a side effect of `go test`. (G5)
- Add **`staticcheck`** (Go) and **ESLint** (TS) — start advisory (non-blocking), flip
  to required once clean. (G4)
- Add **coverage** upload (Codecov/Coveralls or artifact-only) for both SDKs. (G7)
- Add a **cross-SDK parity assertion** as its own check (both SDKs load the shared
  `testdata/behavior/` fixtures — make a failure here unmissable).

### Tier C — Supply chain & security (medium effort)
- **`govulncheck`** in Go job; **`npm audit --audit-level=high`** in TS job. (G6)
- **Dependabot** for `gomod`, `npm`, and `github-actions` ecosystems.
- **CodeQL** (Go + JS/TS) on push to `main` + PR.
- On release: **build provenance / SLSA attestation** (npm supports `--provenance`;
  GitHub `attest-build-provenance` for Go artifacts).

## 4. Release automation — tag-triggered publish (chosen direction)

Decision: **you push a version tag, CI does build → test → publish.** You keep control
of *when*; CI owns *how*. This fits three independent modules cleanly because each tag
prefix routes to a different publish path.

### Shape
A new `.github/workflows/release.yml` triggered on tag push, with a job per prefix
gated by the matched tag:

| Tag pushed | CI does |
|---|---|
| `sdk/akg-ts/vX.Y.Z` | `npm ci` → build → test → `npm publish --provenance` (npm trusted publishing / OIDC, no long-lived token). |
| `sdk/akg-go/vX.Y.Z` | build + full test, then `GOPROXY proxy.golang.org` warm-up fetch so the version is indexed; verify tag ↔ module path. |
| `vX.Y.Z` (root) | build + test reference SDK; proxy warm-up. |
| any of the above | extract the matching CHANGELOG section → create a **GitHub Release** with notes; verify the tag commit's `package.json`/version matches the tag. |

### Guardrails to bake in
- **Re-run the full test + conformance suite on the tagged commit** before publishing —
  never publish on trust that "CI was green on the PR."
- **Version ↔ tag consistency check** (TS `package.json` version must equal the tag's
  `X.Y.Z`; fail loudly otherwise). This is the single most common manual-release bug.
- **`npm publish --provenance` via OIDC trusted publishing** — no `NPM_TOKEN` secret to
  leak/rotate.
- Keep `prep-for-release` as the *authoring* step (changelog + commit + tag); the
  workflow is the *publishing* step. They compose: skill tags → push → CI publishes.

### Alternatives considered (and why not, for now)
- **Fully automated (release-please)**: bot-driven release PRs from conventional
  commits. Most hands-off, but imposes a commit-message convention on a small team and
  hides the "when" decision. Revisit if release cadence climbs.
- **Keep manual + verification-only workflow**: smallest change, but leaves the publish
  step itself manual — doesn't remove G2's core risk.

## 5. Recommended sequence

1. **Tier A** now — pure upside, unblocks everything (matrix catches the 1.22/1.26 gap today).
2. **Release workflow (§4)** next — removes the highest-risk manual step; the version↔tag
   check alone prevents a whole class of bad publishes.
3. **Tier C security** — fast to add (Dependabot + govulncheck + npm audit are a few lines
   each), high signal for published libraries.
4. **Tier B confidence** — staticcheck/ESLint advisory first, named conformance gate,
   then flip to required once clean and **add branch protection** requiring the matrix +
   conformance + release-dry-run checks (G10).

### Open questions for you
- **Go matrix floor**: keep `go.mod` at `1.22`, or bump the floor to `1.26` and test only
  recent? (Affects who can consume the modules.)
- **npm publishing identity**: set up **trusted publishing (OIDC)** on the npm package, or
  use a stored `NPM_TOKEN`? (OIDC strongly preferred.)
- **Coverage**: external service (Codecov) or artifact-only to avoid a third-party dep?
- **Lint strictness**: start `staticcheck`/ESLint advisory, or block from day one?
