# AKG Comprehensive Test Plan

## Purpose

This document defines a practical, multi-layer testing strategy for the AKG MVP reference implementation.

It is designed to validate four things separately and together:

1. **format correctness**
2. **reference API semantic correctness**
3. **SDK-level agent usability**
4. **realistic agent workflow usefulness**

The plan intentionally matches the finalized AKG MVP constraints:

- AKG is a **tiny format-layer reference implementation**, not a full product
- the 4 core mutation actions are central:
  - `PutNode`
  - `PutEdge`
  - `DeleteNode`
  - `DeleteEdge`
- deletes are **strict** and must return not-found when the target is absent
- ordinary reads expose **only current logical state**
- ordinary open automatically applies valid committed WAL through the last valid `COMMIT`
- salvage/recovery is separate from normal open
- authoritative in-memory state is **nodes + edges only**
- secondary keys are derived
- Data section contains **only current live state**, no tombstones
- ordinary commit appends WAL records plus `COMMIT` and leaves WAL in place until compaction

---

## Core Testing Philosophy

The reference implementation should be proven at multiple layers because each layer answers a different question.

### Layer 1: Format / conformance
**Question:** Is the AKG file format implemented correctly?

### Layer 2: Reference API behavior
**Question:** Does the minimal implementation obey AKG semantics in ordinary use?

### Layer 3: SDK integration behavior
**Question:** Can an agent actually use AKG through an SDK surface without special-casing or hand-holding?

### Layer 4: Realistic workflow behavior
**Question:** Does memory improve actual agent loops across turns and sessions?

No single layer is sufficient by itself:

- byte-level conformance alone does not prove agent usability
- an agent demo alone does not prove format correctness
- API unit tests alone do not prove persistence and recovery behavior

So the test plan must cover all four.

---

## Non-Goals of the Test Plan

The tests should **not** turn the reference implementation into a product.

Do not expand test scope into:

- a general query engine
- rich traversal semantics beyond MVP reads
- a broad application SDK abstraction
- a server or service layer
- MCP-specific integration as a requirement
- multi-writer behavior
- merge semantics beyond MVP scope

The tests should stay tightly aligned to the reference implementation goal: proving AKG v1 MVP is clear, correct, and usable.

---

## Test Layers

# 1. Format and Conformance Tests

## Goal

Prove that AKG files, records, WAL, compaction, and validation behavior conform to the spec.

## Test style

Use deterministic tests with:

- golden fixtures where useful
- round-trip read/write/read assertions
- explicit rejection-case fixtures
- direct file inspection for Data/WAL expectations

## Required coverage

### 1.1 Header and binary container

Test:

- valid magic `AKG\0`
- invalid magic rejected
- supported major/minor accepted appropriately
- unsupported major rejected
- reserved bytes written as zero
- header checksum validated
- bad header checksum rejected
- section table parsed correctly
- unknown section types skipped
- section ordering treated as table-driven, not positional

### 1.2 Section integrity and validation

Test:

- Data section checksum validated
- WAL section checksum validated
- bad section checksum rejected
- section length interpreted as payload + checksum bytes
- all fixed-width binary integers decoded as little-endian
- exactly one Data section required
- at most one Bloom section allowed
- at most one WAL section allowed
- unknown section types skipped if structurally valid
- overlapping sections rejected
- out-of-bounds sections rejected
- zero-length Data rejected
- zero-length Bloom rejected
- zero-length WAL allowed

### 1.3 Node payload decoding

Test:

- minimal valid node with required fields only
- fully populated node
- missing required `type` rejected on write
- missing required `title` rejected on write
- missing optional `body` defaults to `""` on read
- missing optional `tags` defaults to `[]` on read
- missing optional `meta` defaults to `{}` on read
- missing `version` defaults to `1` on read
- missing `created_at` defaults to `0` on read
- missing `updated_at` defaults to `0` on read
- unknown MessagePack fields tolerated on read
- unknown MessagePack fields dropped on rewrite

### 1.4 Edge payload decoding

Test:

- minimal valid edge with required fields only
- fully populated edge
- missing required `from_node` rejected on write
- missing required `to_node` rejected on write
- missing required `relation` rejected on write
- missing optional `strength` defaults to `0.5` on read
- missing optional `confidence` defaults to `null` on read
- missing optional `meta` defaults to `{}` on read
- missing `version` defaults to `1` on read
- missing `created_at` defaults to `0` on read
- missing `updated_at` defaults to `0` on read
- `confidence: null` preserved logically
- `confidence: 0.5` preserved logically
- `strength: 0.0` preserved logically
- `strength: 1.0` preserved logically
- unknown MessagePack fields tolerated on read
- unknown MessagePack fields dropped on rewrite

### 1.5 Key layout and key validation

Test node keys:

- `n:{type}:{id}` format
- type preserved exactly
- id preserved exactly
- id with `:` rejected
- id longer than 64 chars rejected

Test edge keys:

- `e:{from}:{relation}:{to}` format
- `ei:{to}:{relation}:{from}` format
- both are emitted for live edges

Test tag keys:

- `t:{tag}:{node_id}` emitted for each tag
- lowercase tag accepted
- snake_case multiword tag accepted
- tag with spaces rejected
- uppercase tag rejected
- more than 32 tags rejected
- duplicate tags rejected

Test temporal keys:

- node temporal key `ts:{timestamp}:n:{type}:{id}` emitted
- edge temporal key `ts:{timestamp}:e:{from}:{relation}:{to}` emitted
- timestamps reflect `updated_at`
- one temporal key per logical record

### 1.6 Data section rules

Test:

- entries encoded as repeated little-endian `uint32 key_len`, `uint32 value_len`, key bytes, value bytes
- entries concatenated with no padding
- empty values encoded with `value_len = 0`
- entries sorted by raw UTF-8 bytewise lexicographic key order
- duplicate keys rejected during validation
- Data section contains only current live state
- Data section contains no tombstones
- Data section contains primary and required derived keys only

### 1.7 WAL record structure and validation

Test:

- valid `PUT_NODE` record
- valid `DELETE_NODE` record
- valid `PUT_EDGE` record
- valid `DELETE_EDGE` record
- valid `COMMIT` record
- `COMMIT` has empty payload
- sequence numbers monotonic
- WAL fixed-width fields encoded little-endian
- `DELETE_NODE` payload requires MessagePack map with `type` and `id`
- `DELETE_EDGE` payload requires MessagePack map with `from_node`, `relation`, and `to_node`
- unknown extra fields in delete payloads tolerated on read
- unknown WAL op rejected
- truncated WAL record rejected
- payload length overrun rejected
- bad WAL record checksum rejected

### 1.8 WAL replay semantics

Test:

- replay through last valid `COMMIT`
- records after last valid `COMMIT` ignored on ordinary open
- WAL with no valid `COMMIT` applies nothing on ordinary open
- ordinary open automatically applies committed WAL
- ordinary open does not expose partial batches
- ordinary open rejects malformed or truncated committed WAL content rather than salvaging automatically
- WAL replay respects record sequence order

### 1.9 Commit semantics

Test:

Given a committed mutation batch:

1. in-memory state mutates
2. WAL records appended
3. `COMMIT` appended
4. fsync boundary respected
5. committed WAL remains present until compaction

Assertions:

- post-commit file may contain non-empty WAL
- ordinary reopen applies committed WAL and exposes only current live logical state
- trailing uncommitted WAL is ignored
- Data section remains the base compacted state representation, not a per-commit full rewrite artifact

### 1.10 Compaction

Test:

- compaction reads current logical state including committed WAL state
- compaction rewrites live state only
- compaction drops tombstones
- compaction rebuilds derived indexes
- compaction rebuilds bloom filter using the normative wire format if present
- compaction discards prior WAL
- compacted file is atomically replaceable
- compacted file preserves logical graph state

### 1.11 Error handling and fail-closed behavior

Test that ordinary readers reject:

- bad magic
- unsupported major version
- bad header checksum
- bad section checksum
- overlapping or out-of-bounds sections
- Data/Bloom/WAL cardinality violations
- malformed Data section
- malformed Bloom section
- truncated WAL record
- invalid WAL opcode
- invalid WAL delete payload shape

Test that ordinary readers tolerate:

- unknown section types
- unknown MessagePack fields
- unknown extra fields in WAL delete payloads
- missing optional fields with defined defaults
- missing timestamps with read default `0`
- trailing uncommitted WAL after the last valid `COMMIT`

### 1.12 Round-trip invariants

Test:

- read → rewrite → read preserves logical content
- exact bytes may differ
- unknown MessagePack fields may disappear after rewrite
- derived keys remain logically correct after rewrite

## Conformance corpus

The corpus should grow incrementally and include at least:

### Baseline fixtures

- empty graph
- minimal node
- full node
- single edge
- small realistic graph

### Encoding edge fixtures

- all valid MessagePack kinds in `meta`
- node with 32 tags
- node with large `body`
- edge with `confidence: null`
- edge with `confidence: 0.5`
- edge with `strength: 0.0`
- edge with `strength: 1.0`

### Format state fixtures

- file with committed WAL requiring replay
- file with trailing uncommitted WAL ignored on ordinary open
- compacted file with empty or absent WAL
- file involving deletions in logical history

### Rejection fixtures

- wrong magic
- unsupported major
- bad header checksum
- bad section checksum
- overlapping or out-of-bounds sections
- cardinality violations
- malformed Data section
- malformed Bloom section
- truncated WAL
- invalid delete payload shape

---

# 2. Reference API Behavior Tests

## Goal

Prove the minimal reference implementation behaves correctly in ordinary caller use.

## Recommended minimal API under test

The exact naming can vary, but the behavior under test should map to:

- `open(path)`
- `putNode(...)`
- `putEdge(...)`
- `deleteNode(...)`
- `deleteEdge(...)`
- `getNode(type, id)`
- `getEdge(from, relation, to)`
- `listOutbound(nodeId)`
- `listInbound(nodeId)`
- `listByTag(tag)`
- `validate()`
- `compact()`

## Mutation semantics tests

### 2.1 PutNode

Test:

- creates a new node when identity absent
- upserts by `(type, id)`
- generated id is 16 random hex chars when omitted
- caller-provided id accepted when valid
- caller-provided invalid id rejected
- payload mutation increments `version`
- writer-owned fields updated correctly

### 2.2 PutEdge

Test:

- creates a new edge when identity absent
- upserts by `(from_node, relation, to_node)`
- edge mutation increments `version`
- writer-owned fields updated correctly
- edge write updates both primary and inbound index logically

### 2.3 DeleteNode

Test:

- deleting existing node succeeds
- deleting missing node returns not-found
- deleted node absent from exact lookup
- deleted node absent from tag lookup
- deleted node absent after reopen
- no tombstone visible through reads

### 2.4 DeleteEdge

Test:

- deleting existing edge succeeds
- deleting missing edge returns not-found
- deleted edge absent from exact lookup
- deleted edge absent from outbound listing
- deleted edge absent from inbound listing
- deleted edge absent after reopen
- no tombstone visible through reads

### 2.5 Type change behavior

Test:

- changing node `type` is treated as identity change
- it is not an in-place type mutation
- original identity remains separate unless explicitly deleted

### 2.6 Dangling edges

Test:

- edge pointing to missing node may exist
- validator tolerates dangling edges
- no cascade delete enforced automatically

## Read semantics tests

### 2.7 Exact lookup

Test:

- exact node lookup returns current version only
- exact edge lookup returns current version only
- deleted records not returned
- no stale version visible

### 2.8 Outbound / inbound listing

Test:

- outbound listing returns current outbound edges only
- inbound listing returns current inbound edges only
- deleted edges excluded
- stale superseded edges excluded

### 2.9 Tag lookup

Test:

- tag lookup returns nodes or references for current tagged nodes only
- deleted nodes excluded
- removed tags no longer visible after update

## Persistence behavior tests

### 2.10 Ordinary open

Test:

- opening a clean file loads current logical state
- opening a file with committed WAL applies through last valid `COMMIT`
- opening ignores trailing uncommitted records
- opening does not run salvage behavior

### 2.11 Reopen invariants

For each mutation shape:

- mutate
- commit
- close
- reopen
- assert same logical state

## CLI tests

### 2.12 `inspect`

Test:

- can inspect valid file
- displays current logical state, not tombstones/raw WAL internals unless explicitly framed as internal diagnostics

### 2.13 `validate`

Test:

- passes on valid files
- fails clearly on invalid files

### 2.14 `compact`

Test:

- compacts valid files
- output preserves logical state
- resulting file validates

---

# 3. SDK-Level Agent Usability Tests

## Goal

Prove that an agent can use AKG through an SDK and tool interface in a normal loop.

This is the most important bridge between “correct storage” and “useful memory.”

## Key principle

Do **not** broaden AKG itself into a product SDK abstraction just for testing.

Instead, build a **thin adapter** over the reference API and expose it to an agent through the Pi SDK as a custom tool.

## Recommended integration shape

Use Pi SDK programmatic sessions with:

- `createAgentSession(...)`
- explicit `cwd`
- in-memory or temp-dir session management
- a tightly scoped system prompt
- `customTools` to register AKG memory access
- event subscriptions to capture tool usage and outcomes

Pi supports this directly through `createAgentSession` and `customTools`.

## Recommended test tool design

Prefer **one structured tool** instead of many tiny tools.

Suggested tool: `akg_memory`

Parameters:

- `action`
- action-specific fields

Suggested actions:

- `put_node`
- `put_edge`
- `delete_node`
- `delete_edge`
- `get_node`
- `get_edge`
- `list_outbound`
- `list_inbound`
- `list_by_tag`
- `validate`
- `compact`

### Why a single tool is better

- fewer tool-selection mistakes by the model
- easier logging and assertions
- direct mapping to AKG semantics
- simpler harness implementation

## Tool behavior recommendations

For SDK usability tests, mutation actions should usually:

- perform the AKG mutation
- commit immediately on success
- return structured results

Reasoning:

- easier for agent loops
- easier to reason about persistence across turns
- WAL batch semantics are already tested below this layer

If desired, add a separate advanced test mode that allows explicit multi-mutation commit boundaries. But that should not be required for the first proof of usability.

## Structured tool result design

Return simple, explicit JSON-like results such as:

- `ok: true/false`
- `error_code`
- `message`
- `node`
- `edge`
- `items`
- `not_found: true`

This helps both the agent and the test harness.

## What to capture from Pi

Subscribe to Pi session events and log at least:

- prompt text
- tool execution start
- tool execution end
- tool parameters
- tool result
- final assistant answer
- turn boundaries
- session boundaries

This gives evidence for:

- whether the agent actually chose to use memory
- whether it used the correct operation
- whether the final answer depended on returned memory

## SDK usability test categories

### 3.1 Retrieval-trigger tests

Prompt the agent with tasks where memory is required or strongly helpful.

Examples:

- “Remember that Sam prefers email.”
- “What does Sam prefer?”

Assertions:

- agent calls `put_node` or `put_edge` appropriately
- later agent calls `get_node`, `list_by_tag`, or edge listing appropriately
- final answer matches stored memory

### 3.2 Update tests

Examples:

- “Remember that Sam prefers email.”
- “Actually Sam prefers Signal now.”
- “What does Sam prefer now?”

Assertions:

- agent performs overwrite/update path correctly
- later read returns only current state
- final answer uses updated value, not stale value

### 3.3 Delete tests

Examples:

- create memory
- “Forget that preference.”
- “What does Sam prefer?”

Assertions:

- agent performs delete action
- subsequent lookup does not surface deleted item
- final answer reflects absence

Also test:

- repeated delete returns not-found
- agent handles not-found sanely

### 3.4 Cross-session persistence tests

Flow:

1. Session A stores facts
2. Session A ends
3. Session B starts fresh against same AKG file
4. Agent is asked to use prior memory

Assertions:

- Session B retrieves prior memory
- no special recovery step required
- ordinary open semantics are sufficient

### 3.5 Restart / reopen correctness tests

Flow:

- create state
- terminate process/session cleanly
- start new process/session
- retrieve state

Optional crash-style test:

- inject file in state with committed WAL pending normal open application
- fresh session opens file
- committed state appears
- uncommitted trailing state does not

### 3.6 Negative lookup tests

Examples:

- ask for absent memory
- verify agent does not hallucinate a hit from storage

Assertions:

- tool result indicates absence clearly
- final answer reports absence appropriately

### 3.7 Tag lookup usability tests

Examples:

- store several nodes tagged `preference`
- ask “What preferences do you know?”

Assertions:

- agent can use `list_by_tag`
- returned items are useful enough for downstream reasoning

### 3.8 Inbound/outbound edge usability tests

Examples:

- build small fact graph
- ask relation-oriented question:
  - “What does Sam prefer?”
  - “Who prefers Signal?”

Assertions:

- agent uses outbound and/or inbound edge listing appropriately
- answers are grounded in graph state

## SDK usability pass criteria

The SDK layer is successful if:

- the agent consistently chooses the memory tool in relevant tasks
- the tool interface is simple enough for reliable use
- reads and writes succeed across turns and sessions
- deletes are actually usable and correctly interpreted
- final answers are grounded in retrieved current-state memory

---

# 4. Realistic Agent Workflow Tests

## Goal

Prove that AKG-backed memory is useful in real agent loops rather than just toy API calls.

These tests should be scenario-driven and slower than unit tests.

They are closer to acceptance/evaluation tests.

## Scenario design principles

A realistic scenario should require the agent to:

- observe information
- decide it is worth storing
- store it correctly
- retrieve it later when relevant
- update or delete it when reality changes
- use only current logical state
- survive restart/reopen boundaries

## Scenario class A: Personal memory notebook

This is the best **small MVP test project**.

### Concept

An agent maintains durable memory about a user or small team across chats.

Possible node types:

- `person`
- `preference`
- `commitment`
- `decision`
- `fact`

Possible edge relations:

- `prefers`
- `dislikes`
- `committed_to`
- `decided`
- `related_to`

### Why this project is ideal for MVP

- tiny schema
- obvious ground truth
- naturally exercises all 4 mutations
- easy multi-session testing
- easy pass/fail assertions

### Core scenario set

#### Scenario A1: basic recall

- user tells agent a preference
- agent stores it
- later turn asks for it
- agent retrieves and answers correctly

#### Scenario A2: update

- user changes preference
- agent updates corresponding node/edge
- later query returns only updated preference

#### Scenario A3: delete

- user explicitly asks to forget something
- agent deletes corresponding node or edge
- later query reports absence

#### Scenario A4: cross-session continuity

- memory created in session 1
- session 2 asks about prior known facts
- agent retrieves them correctly

#### Scenario A5: relation queries

- several people and preferences stored
- ask questions requiring edge traversal direction:
  - “What does Sam prefer?”
  - “Who prefers Signal?”

## Scenario class B: Pi coding-session memory sidecar

This is the best **realistic scenario**.

### Concept

A Pi-based coding agent stores durable project memory while working in a repository.

Possible node types:

- `file`
- `module`
- `issue`
- `decision`
- `bug`
- `hypothesis`
- `preference`

Possible edge relations:

- `depends_on`
- `modifies`
- `caused_by`
- `supersedes`
- `relates_to`
- `blocks`

### Why this is the best realistic proof

- matches Pi’s natural operating environment
- naturally spans multiple sessions
- update/delete behavior matters because hypotheses change
- memory should help the agent avoid rediscovering prior context

### Core scenario set

#### Scenario B1: persistent issue memory

- agent learns bug hypothesis in session 1
- stores it
- session 2 asks what was learned previously
- agent retrieves prior hypothesis

#### Scenario B2: hypothesis supersession

- agent stores initial explanation for a bug
- later discovers a better explanation
- updates/supersedes old memory
- later summary reflects current state only

#### Scenario B3: stale memory deletion

- obsolete workaround is removed from memory
- later question does not surface deleted workaround

#### Scenario B4: graph-supported coding recall

- agent stores relationships between issue, files, and decisions
- later asked “which files are tied to this issue?”
- agent uses edge retrieval to answer

#### Scenario B5: restart continuity

- repository memory created in one run
- new Pi session started later
- memory is still usable without special recovery workflow

## Optional scenario class C: task/meeting memory

Useful but less aligned with Pi’s coding-first setting.

Possible use if you want a less technical realistic demo:

- action items
- decisions
- attendees
- follow-ups

Good fallback, but the coding-session sidecar is stronger evidence for Pi integration.

---

# 5. Integration with the Pi Harness

## Goal

Use Pi as the agent runtime that proves AKG-backed memory is actually usable in practice.

## Recommended architecture

Build a small test harness app that:

1. creates a temp project directory
2. creates or points at an AKG file
3. starts a Pi SDK session
4. registers a custom AKG memory tool
5. sends prompts programmatically
6. records all tool interactions and final outputs
7. asserts correctness

## Why use a harness app first

This is simpler and cleaner than making a production Pi extension immediately.

Benefits:

- fewer moving parts
- deterministic test setup
- easy temp-dir isolation
- easy CI integration
- no need to commit to a final product UX yet

## Suggested Pi SDK configuration

- `createAgentSession(...)`
- `SessionManager.inMemory(...)` for tests
- explicit `cwd`
- minimal controlled system prompt
- `tools` kept minimal
- `customTools: [akgMemoryTool]`

## Suggested system prompt shape

The system prompt for evaluation runs should say clearly:

- durable memory is available via `akg_memory`
- use it when remembering or recalling stable facts is helpful
- memory operations are explicit
- be concise and grounded in tool results

This reduces prompt ambiguity and makes results easier to interpret.

## Suggested event logging

Capture at least:

- scenario id
- session id
- prompt
- ordered tool calls
- ordered tool results
- assistant final answer
- validation result
- whether a reopen/new-session step occurred

## Suggested harness output artifacts

Per scenario, emit:

- human-readable markdown or text summary
- machine-readable JSON trace
- pass/fail verdict
- optional final AKG file artifact for debugging

## Recommended assertion types

### Structural assertions

- expected tool call happened
- unexpected tool call did not happen
- delete of missing item produced not-found

### State assertions

- final AKG state matches expected nodes/edges
- `validate` passes
- current-state reads match expectations

### Behavioral assertions

- final assistant answer includes correct recalled fact
- final assistant answer does not include deleted/stale fact
- after restart, agent still recalls persisted fact

---

# 6. Detailed Test Matrix

## A. Core mutation matrix

Every mutation should be tested across these dimensions.

### `PutNode`

- create new node
- update existing node same `(type, id)`
- auto-generate id
- accept valid caller id
- reject invalid caller id
- update tags
- remove tags on rewrite
- unknown fields dropped on rewrite
- reopen after commit

### `PutEdge`

- create new edge
- update existing edge same identity
- update strength/confidence/meta
- inbound/outbound visibility correct
- temporal key updated correctly
- reopen after commit

### `DeleteNode`

- delete existing node
- delete missing node => not-found
- node absent from exact lookup
- node absent from tag lookup
- node absent after reopen
- Data contains no tombstone after rewrite/compaction

### `DeleteEdge`

- delete existing edge
- delete missing edge => not-found
- edge absent from exact lookup
- edge absent from inbound/outbound listing
- edge absent after reopen
- Data contains no tombstone after rewrite/compaction

## B. Open/recovery matrix

- clean file open
- file with committed WAL open
- file with trailing uncommitted WAL open
- file with no valid `COMMIT` open
- file with truncated WAL rejected
- salvage kept separate from ordinary open

## C. Read visibility matrix

Ensure no read path exposes:

- tombstones
- stale superseded node version
- stale superseded edge version
- partial batch after invalid/uncommitted WAL tail
- raw WAL internals

## D. Derived index matrix

For each live logical node/edge, verify correct derived keys:

- `ei:` for edges
- `t:` for tags
- `ts:` for nodes
- `ts:` for edges

And verify deleted/superseded state does not leak via derived indexes.

---

# 7. Test Data and Fixture Strategy

## Principles

Use a mix of:

- hand-authored tiny fixtures
- generated valid fixtures
- generated invalid/corrupt fixtures
- scenario-generated files from the reference implementation itself

## Golden fixtures should cover

- canonical small valid files
- files with committed replayable WAL
- files with section ordering variations
- files with unknown sections
- files with unknown payload fields
- corrupt files for rejection tests

## Property-style / generative tests

Where useful, generate random small graphs and assert:

- write/read logical equivalence
- compaction preserves logical state
- reopen preserves logical state
- exact and derived lookups agree

Keep generation bounded and deterministic via seeded randomness.

---

# 8. CI Test Suite Structure

## Suggested suite tiers

### Tier 1: fast unit tests

Run on every change.

Includes:

- payload validation
- key generation
- mutation semantics
- read helper semantics

### Tier 2: file-format integration tests

Includes:

- real file read/write
- WAL replay
- compaction
- rejection fixtures
- round-trip invariants

### Tier 3: Pi SDK integration tests

Includes:

- AKG custom tool
- single-session memory use
- cross-session persistence
- delete behavior

### Tier 4: realistic scenario/eval tests

May run less frequently.

Includes:

- personal memory notebook scenarios
- coding-session memory sidecar scenarios
- multi-turn and restart flows

## Flakiness policy

SDK/agent tests should be designed to minimize flakiness by using:

- fixed prompts
- low-temperature or most deterministic available model settings
- strongly structured tool interfaces
- explicit system instructions
- assertions based primarily on tool traces and state, not prose style

If possible, separate:

- strict deterministic integration tests
- looser benchmark/eval runs

---

# 9. Recommended Candidate Test Projects

## Project 1: Personal Memory Notebook

### Recommendation

Build this first.

### Why

It is the smallest convincing proof that:

- AKG storage works
- the tool interface is usable
- the agent can store, recall, update, and delete memory
- persistence works across sessions

### MVP feature set

- store a person fact/preference
- recall it later
- update it
- forget it
- list by tag
- answer relationship questions

### What it proves

- all 4 mutation actions matter in practice
- exact lookup and simple graph retrieval are sufficient for a real use case
- the memory tool is understandable to the agent

## Project 2: Pi Coding-Session Memory Sidecar

### Recommendation

Build this second.

### Why

It is the strongest practical proof for Pi integration because it lives in Pi’s native environment: coding workflows in repositories.

### MVP feature set

- remember issue hypotheses
- remember coding decisions
- remember file/issue relationships
- update or supersede hypotheses
- delete obsolete information
- recall memory in later sessions

### What it proves

- AKG-backed memory helps a coding agent over time
- graph relationships are genuinely useful
- restart continuity works
- stale/deleted memory is not incorrectly reused

---

# 10. Suggested Acceptance Criteria

The AKG memory solution should be considered proven for MVP purposes when all of the following are true.

## Format-level acceptance

- conformance corpus passes
- required rejection cases fail correctly
- required tolerance cases succeed correctly
- round-trip invariant holds

## Semantics-level acceptance

- all 4 mutation actions are directly tested and passing
- delete-not-found behavior is enforced
- reads expose only current logical state
- ordinary open applies committed WAL through last valid `COMMIT`
- trailing uncommitted WAL is ignored
- compaction preserves logical graph state

## SDK-level acceptance

- Pi SDK harness can expose AKG via custom tool cleanly
- agent reliably uses the tool when memory is relevant
- cross-session recall works
- update and delete workflows work through the tool
- final answers are grounded in retrieved memory

## Workflow-level acceptance

- Personal Memory Notebook scenario set passes
- Pi Coding-Session Memory Sidecar scenario set passes
- deleted/stale memory is not resurfaced in later answers
- persisted memory is reusable in fresh sessions without recovery mode

---

# 11. Recommended Implementation Order for Testing

## Phase 1: deterministic correctness

Build first:

1. unit tests for records, keys, validation
2. file-format integration tests
3. WAL/open/compaction tests
4. conformance fixtures

## Phase 2: thin SDK adapter

Build next:

5. thin AKG tool adapter
6. Pi SDK harness app
7. deterministic SDK integration tests

## Phase 3: scenario proofs

Build last:

8. Personal Memory Notebook project
9. Pi Coding-Session Memory Sidecar project
10. multi-session and restart evaluations

This order is important: agent tests should sit on top of already-solid storage semantics, not substitute for them.

---

# 12. Practical Next Deliverables

A good next implementation sequence would be:

1. define the minimal reference API under test
2. define the `akg_memory` custom tool schema
3. create the conformance fixture layout
4. implement deterministic integration tests for WAL/open/compaction
5. build a Pi SDK harness runner for scripted scenarios
6. implement the Personal Memory Notebook evaluation
7. implement the Coding-Session Memory Sidecar evaluation

---

## Final Recommendation

To prove the AKG memory solution works in practice without bloating the reference implementation:

- keep AKG itself tiny and spec-faithful
- test the format ruthlessly at the conformance layer
- expose AKG to agents through a **thin Pi custom tool adapter**
- prove usability first with **Personal Memory Notebook**
- prove practical value next with **Pi Coding-Session Memory Sidecar**

That gives the strongest evidence across correctness, usability, and real workflow value while preserving the MVP architecture.
