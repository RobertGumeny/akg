## Definition Lens

Use this reference when an epic needs executable tasks and acceptance criteria.

### Atomicity test

Before writing a task, confirm it passes:
- One clear output — a new function, a removed path, a rewritten section, a passing test suite
- Scope fully resolved: the agent can complete the task using the PRD, KB, and project files as context without having to invent requirements or make design decisions the plan left open
- No dependency on another in-flight task's unreleased code
- Acceptance criteria verifiable by running commands, not by reading the diff

The agent is expected to read external context (PRD, KB, project files) — that is the point. What the task must not require is the agent to *resolve ambiguity* that belongs in the plan.

If you cannot describe "done" as a command and its observable output, the task needs more definition.

### Focus on

- decomposing work into agent-executable tasks that fit in one pass
- giving each task a concrete description with a single clear output
- writing 2-5 binary acceptance criteria per task — each phrased as an observable input/output or exit code, not an assertion about quality
- rejecting vague wording such as "correctly", "reasonable", "appropriate", or "as needed"
- splitting tasks that hide multiple independent decisions (watch for "and" in the description)

If you cannot write concrete acceptance criteria, the task is still underspecified.

See `references/task-examples.md` for before/after examples of weak vs. well-formed tasks.
