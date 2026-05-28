## PRD Quality Reference

Use this reference when drafting or reviewing an epic PRD. A PRD that does not answer these questions will produce vague tasks and ambiguous agent briefings.

### Required Sections

**Problem — today's pain**
Describe what the operator or user does today, and what is manual, slow, or error-prone. One concrete example (a command they run, a file they edit, a step they repeat) is worth three sentences of abstract description.

**Goal — the single outcome**
State the primary outcome in one sentence. If you cannot, the epic scope is probably too large and should be split into smaller epics.

**Target UX** *(for user-facing or operator-facing features)*
Show the concrete interaction that represents success: a CLI invocation with example output, a UI state, or an API call. "The user can do X" is weaker than showing what X looks like.

**In scope**
Explicit list. Do not leave scope implicit — agents fill gaps with reasonable-seeming scope creep.

**Out of scope / non-goals**
Explicit list. Things that are obviously adjacent but deliberately excluded.

**Constraints**
Hard requirements the agent must not violate. Distinguish hard constraints (must) from preferences (should). Include: interface compatibility, architectural rules, libraries to avoid, files to leave unchanged.

**Open questions** *(optional but honest)*
Unresolved things that need resolution before or during execution. Better to surface them in the PRD than to have the agent guess mid-task.

---

### Example: Weak PRD

> Add an operator-facing batch execution layer. The goal is to eliminate repeated manual command construction by letting the operator run a checked-in job definition directly.
>
> Scope:
> - add `mytool batch run` subcommand
> - load the JSON job definition and derive the planned work set deterministically
> - delegate actual single-item execution to existing `mytool run` logic
> - provide dry-run and status output

**Problems with this PRD:**
- No current-state pain — what does the operator do today, step by step?
- No target UX — what does a successful `mytool batch run` look like? What does it print?
- "Deterministically" is undefined — in what order, from what source?
- Non-goals are absent — can the agent modify `mytool run`? Add new flags to it?
- No constraints — what happens on partial completion? How does resume work?

---

### Example: Strong PRD

**Problem**
Running a batch job today requires the operator to manually invoke `mytool run` once per item, track which items have completed, and reconstruct the full invocation list if interrupted. A 10-item job requires 10 manual commands and ad-hoc bookkeeping with no recovery path.

**Goal**
Let the operator execute a full checked-in job definition with a single command, with resumable progress and dry-run preview.

**Target UX**
```
# Preview planned work without executing
mytool batch run jobs/nightly-export.json --dry-run
# Output: table of planned items with status (pending/done)

# Run all pending items
mytool batch run jobs/nightly-export.json

# Resume after interruption — skips already-completed items
mytool batch run jobs/nightly-export.json --resume
```

**In scope**
- `mytool batch run <job-file>` command
- Loading and validating the job definition JSON
- Deriving item list deterministically (alphabetical by item ID)
- Delegating single-item execution to the existing `mytool run` path
- `--dry-run`: print planned items and exit 0 without executing
- Progress persistence in a sidecar file next to the job definition

**Out of scope**
- Changes to `mytool run`'s single-item interface
- Parallel item execution
- Job authoring, linting, or schema generation tooling
- Non-CLI output formats

**Constraints**
- `mytool run` remains the low-level primitive; do not duplicate its internal logic
- Reruns must be reconstructable from the checked-in job file alone — no state stored outside the project directory

**Open questions**
- Should a failed item halt the batch or continue and report at the end?
