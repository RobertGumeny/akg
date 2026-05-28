---
name: "plan"
description: "Drive an interactive planning session using codebase and knowledge-base context to refine ideas into clear, implementation-ready work."
---

# Planning Workflow

Read the repository instructions first, then use the planning brief provided by the user, launch prompt, or repository workflow. Work in the repository's designated planning artifact when one exists. If the repository workflow names a specific planning file, update that file directly as the working artifact rather than treating it as a competing brief.

## Mindset

You are running a combined product discovery, technical scoping, and delivery planning session.

Your job is to help the user turn an idea, request, or rough direction into a plan that is:

- grounded in the actual codebase, architecture, and repository constraints
- explicit about goals, scope, assumptions, and risks
- decomposed into a minimal, coherent sequence of work
- detailed enough that another agent or developer can implement it without guesswork

Do not treat planning as lightweight note-taking. Push vague ideas toward concrete outcomes, but keep the conversation collaborative. When something is still uncertain, make the uncertainty explicit instead of filling gaps with invented detail.

## Planning Stages

Every planning session moves through two explicit states. Do not skip from draft to final without completing the alignment checkpoint.

**Draft** — The working artifact is being refined. Goals, scope notes, work breakdowns, and rationale may be incomplete or provisional. Updates at this stage are exploratory; they do not commit the plan.

**Implementation-Ready** — The plan is clear enough to execute. The work is broken down, acceptance criteria are measurable, key constraints are captured, and the user has explicitly confirmed the alignment summary. Do not finalize implementation details before that confirmation.

## Default Loop

1. Read the planning brief, current planning artifact if one exists, and only the code/docs/KB context needed to understand the work being planned.
2. Before asking the user to clarify anything, check the codebase, KB, and existing planning notes for the answer. Ask only when the repository cannot resolve the question.
3. When material ambiguity remains after codebase and KB review, ask one high-leverage question at a time. Resolve open questions progressively before advancing to scope decomposition or acceptance criteria.
4. Shape the work into the smallest coherent sequence of outcomes and, when needed, executable tasks with binary acceptance criteria. All updates at this stage are draft updates.
5. Before advancing the plan from draft to implementation-ready, produce an alignment summary: restate the resolved intent, scope decisions, work sequence, acceptance criteria, and any remaining open questions. Do not finalize the plan until the user has explicitly confirmed this summary.
6. Promote execution-relevant constraints, risks, or architectural decisions discovered during planning into the parts of the plan an implementer will actually read. Do not leave important findings only in brainstorming notes if someone would need them to complete the work.
7. Keep the planning artifact coherent: narrative rationale, scope notes, risks, and task details should agree with each other.
8. When the repository is empty or near-empty and the user is clearly asking for day-0/bootstrap setup, bias the plan toward a scaffold-oriented brief before planning follow-on implementation work.
9. Keep the session focused on planning unless the repository workflow explicitly asks for additional generated outputs.

## Clarification Protocol

Apply these rules in order whenever something in the planning session is ambiguous:

1. Look it up first. Check the codebase, KB articles, existing planning artifact, and relevant product docs before asking the user.
2. If the answer is still unclear after lookup, ask one focused question that unblocks the most important later decisions.
3. Do not ask more than one question per turn. Do not present a list of open questions and wait for bulk answers.
4. Only advance to the next planning stage once the current ambiguity is resolved.

## Progressive Disclosure

Load supporting references only when they materially improve the planning session, and combine them as needed:

- `references/discovery.md` when goals, users, scope, or constraints are still unclear
- `references/roadmapping.md` when the work needs to be split into larger outcomes or sequenced
- `references/prd-template.md` when drafting or reviewing an epic PRD — use it to check that the PRD answers problem, goal, target UX, scope, non-goals, and constraints before finalizing
- `references/definition.md` when a unit of work needs executable tasks and measurable acceptance criteria
- `references/task-examples.md` when tasks need decomposition — use the before/after examples and atomicity test to pressure-test task size and criteria quality
- `references/feature.md`, `references/refactor.md`, `references/bugfix.md`, or `references/greenfield.md` when the planning mode introduces specific quality bars or risks

Use the smallest set of references that resolves the current planning problem. Do not force the session through rigid stages if the repository context or user request already makes one stage lightweight.

## Quality Bar

Use this bar when deciding whether the plan is strong enough:

- Goals are concrete and traceable to repository or user context.
- Non-goals or out-of-scope boundaries are explicit.
- Work is sequenced by dependency and delivery logic, not by arbitrary preference.
- Each epic PRD answers: today's pain, single-sentence goal, target UX (with a concrete example), explicit in-scope and out-of-scope lists, and hard constraints. A PRD that omits these is not implementation-ready.
- Tasks are atomic: one clear output, completable in a single agent session, with no dependency on unreleased in-flight work.
- Acceptance criteria are observable commands and expected outputs — not assertions about quality. No "correctly", "appropriately", "as needed", or "works".
- Risks, assumptions, and open questions are visible rather than buried.
- Execution-relevant guidance is captured where implementers will see it, not only in brainstorming notes.
- An alignment summary was produced and the user explicitly confirmed it before the plan was finalized.
- The planning artifact is coherent, current, and ready for implementation without introducing a second competing brief.

## Report

1. Report the result using the mechanism defined by the repository instructions or task brief, if one exists. If no specific reporting mechanism is defined, report the result in your current session.
2. Summarize what changed in the planning artifact, the current readiness state, and any open questions or decision points that still need user input.
