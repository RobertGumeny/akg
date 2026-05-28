## Task Decomposition Examples

Use this reference when breaking down epic work into agent-executable tasks.

### The atomicity test

A well-formed task:
- Has one clear output — a new function, a removed code path, a rewritten section, a passing test suite
- Has scope fully resolved in the plan: the agent can complete the work using the PRD, KB, and project files as context without having to invent requirements or make design decisions the plan left open
- Does not depend on another in-flight task's unreleased code
- Has acceptance criteria verifiable by running commands, not by reading the diff

The agent is expected to read external context (PRD, KB, project files) — that is the point. What the task must not require is the agent to *resolve ambiguity* that belongs in the plan.

If you cannot describe "done" as a command and its observable output or behavior, the task needs more definition.

---

### Example: Weak task

```yaml
- id: "EPIC-N-001"
  type: "feature"
  description: "Add the batch run command"
  acceptance_criteria:
    - "Command loads job definition JSON"
    - "Dry-run mode works"
    - "Command executes the job items"
```

**Problems:**
- Description is a feature name, not an atomic deliverable — what part of "add the command"?
- Criteria are not binary: "loads job definition JSON" says nothing about validation, error handling, or exit codes
- "Dry-run mode works" — works how? What does it print? What is the exit code?
- Three independent behaviors in one task — if item execution fails, the whole task fails even if parsing is correct

---

### Strong decomposition of the same work

```yaml
- id: "EPIC-N-001"
  type: "feature"
  description: "Add the `mytool batch run` command skeleton with job definition loading and validation."
  acceptance_criteria:
    - "`mytool batch run <path>` is registered and appears in CLI help output."
    - "`mytool batch run <path>` exits non-zero with a descriptive error when the file is missing or contains invalid JSON."
    - "`mytool batch run <path> --dry-run` prints the planned item list to stdout and exits 0 without invoking any execution logic."
    - "Item list order is stable across repeated invocations with the same input file."
    - "The full test suite passes."

- id: "EPIC-N-002"
  type: "feature"
  description: "Implement the item execution loop in `mytool batch run`, delegating each item to the existing `mytool run` single-item path."
  acceptance_criteria:
    - "`mytool batch run <path>` invokes the existing single-item execution path once per planned item."
    - "The single-item execution logic is not duplicated — the existing `mytool run` path is called directly."
    - "An item that errors is reported to stderr; the command exits non-zero after all items have been attempted."
    - "The full test suite passes."

- id: "EPIC-N-003"
  type: "feature"
  description: "Add progress persistence to `mytool batch run` so interrupted runs can be resumed with `--resume`."
  acceptance_criteria:
    - "Completed items are written to a `.progress` sidecar file next to the job definition after each item completes."
    - "`mytool batch run <path> --resume` skips items already marked complete in the sidecar file."
    - "Running without `--resume` when a sidecar file exists prints a warning and exits non-zero rather than silently re-running completed items."
    - "The full test suite passes."
```

**Why this is better:**
- Each task has one clear output verifiable by running commands
- Criteria state inputs, expected outputs, and exit codes — not vague assertions
- Tasks fail independently: execution can be fixed without touching the parsing task
- An agent briefed with only EPIC-N-002 knows exactly what to touch and what to leave alone

---

### Signs a task is too large

- The description contains "and" connecting two distinct outputs
- More than 5 acceptance criteria (usually signals multiple concerns bundled together)
- Criteria use words like "correctly", "appropriately", "as needed", or "works" without specifying observable behavior
- The task requires the agent to invent an interface the PRD did not specify

### Signs a task is too small

- The only acceptance criterion is verifiable by reading the diff, not by running the code
- The task cannot reach DONE without immediately requiring a follow-on task to make it useful
- The task is a pure rename or mechanical move with no behavioral change to verify
