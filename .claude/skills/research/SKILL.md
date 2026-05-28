---
name: "research"
description: "Perform read-only codebase analysis and produce a portable research document. Use when exploring a feature, module, file, function, or the whole codebase to understand current behavior, dependencies, and technical debt."
---

# Research Workflow

Read the repository instructions first, then use this workflow for read-only analysis. Do not modify product code, docs, or task files as part of the research itself unless the user or repository workflow explicitly asks you to update the research artifact.

## Phase 1: Clarify Scope

Valid scope types:

1. Feature or module
2. File or function origin / usage tracing
3. Full codebase map

If the scope is still ambiguous after reading the task request and repository guidance, ask for clarification before continuing.

## Phase 2: Gather Context

1. Read the task request and repository guidance
2. Read product context when requirements or goals are relevant
3. Read nearby documentation when repository-specific rules affect the area you are tracing

## Phase 3: Explore the Codebase

Use read-only tools to map the requested scope:

- `Glob`: find relevant files
- `Grep`: locate definitions, imports, usages, and references
- `Read`: inspect the implementation
- `LS`: map directory structure

For feature or module research:

1. Identify entry points
2. Trace data flow through the relevant components, services, commands, and storage layers
3. Map the files that participate in the feature

For file or function origin research:

1. Start from the named file or symbol
2. Find what it depends on
3. Find what depends on it

For full-codebase research:

1. Map the top-level structure
2. Identify major modules and responsibilities
3. Summarize the architectural shape

## Phase 4: Write the Report

Create or update the research report requested by the user or repository workflow. If no output path is specified, ask where the report should be saved or provide the report in the current session.

Use this structure when no more specific format is provided:

```markdown
# Research Report: [Scope Description]

**Generated**: [YYYY-MM-DD]
**Scope Type**: [Feature/Module | File/Function Origin | Full Codebase]
**Context**: [Repository guidance, product docs, or direct user request]

---

## Overview

[2-3 sentences max]

---

## File Manifest

| File | Purpose |
| ---- | ------- |
| `path/to/file` | Brief description |

---

## Data Flow

[Describe how data moves through the system]

---

## Dependencies

### Internal Dependencies
- `path/to/file` — Purpose

### External Dependencies
- `package-name` — Purpose

---

## Patterns Observed

- **Pattern**: How it is used

---

## Anti-Patterns & Tech Debt

- **Issue**: Description and location

---

## Requirement Alignment

[How the implementation lines up with the task, product intent, or stated requirements]

---

## Raw Notes

[Optional observations that do not fit above]
```

## Phase 5: Finalize

1. Review the report for accuracy and scope discipline
2. Ensure all file paths and identifiers are correct
3. Confirm the report is saved or delivered according to the requested output path or repository workflow
4. Summarize the key findings to the user
