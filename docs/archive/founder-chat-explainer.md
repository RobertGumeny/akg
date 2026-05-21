# Founder Chat Explainer — AKG

## 1. Repo understanding

AKG is a spec-first project for a structured, single-file knowledge graph format for AI agents. It is designed as durable, portable, inspectable working memory for agents: a way to store explicit nodes, relationships, metadata, and update history in a format that can survive across processes, tools, hosts, and implementations. The repo currently contains the MVP spec, key design decisions, section-by-section spec outline, test planning, and a plan for a small Go reference implementation.

- **What it is**
  - A portable binary format for structured agent memory
  - A single-file knowledge graph with explicit nodes, edges, indexes, WAL, checksums, and compaction
  - A spec-first infra project aimed at interoperability, not just one app
- **Who it is for**
  - Primarily SDK authors and infra/tooling builders implementing agent memory systems
  - More broadly, teams building agents that need durable memory beyond repo-local workflows
- **What problem it solves**
  - Gives agents persistent working memory that is structured, inspectable, and portable
  - Avoids leaving useful agent memory trapped inside ad hoc markdown folders inside codebases
  - Provides an alternative to treating memory as semantic search over blobs
- **What is distinctive**
  - Strong bias toward structured memory over vector-first memory
  - Portable single-file format rather than a service dependency
  - Explicit design around correctness: checksums, WAL replay, compaction, conformance corpus
  - Focus on exact identifiers, tags, typed scans, and graph traversal instead of fuzzy recall
- **What it suggests about the user as a builder**
  - Strong taste and willingness to choose constraints deliberately
  - Systems-level thinking about storage, durability, interoperability, and failure modes
  - Practical AI instincts: building around what agents actually do well instead of defaulting to hype patterns
  - Ability to turn a workflow insight into a clean spec and implementation plan
- **What came from the repo vs what came from the interview**
  - **From the repo:** AKG is a structured single-file knowledge graph format for AI agents; it emphasizes portability, inspectability, explicit structure, WAL/compaction/recovery, and a Go reference implementation plan.
  - **From the interview:** the idea came from a markdown-based memory workflow that already worked for coding agents; a startup job posting about long-lived agent context helped crystallize the broader use case beyond repos; the main point is structured memory and portability, not novelty in file-format mechanics; the desired shape is lightweight in-process memory that can persist across sessions without heavy RAG infrastructure; the current state is an almost-finalized MVP spec with the reference implementation starting now; the goal is to sound strong to Seed–Series A startup teams.

## 2. Speaking versions

### 10-second version
I’m building a portable memory file format for AI agents — basically structured agent memory instead of just dumping everything into vector search.

### 20-second version
I started with a markdown-based memory system for coding agents that worked surprisingly well, then realized the useful idea was bigger than repos. So I’m building AKG, a portable single-file format for structured agent memory.

### 30-second version
I’d been using a markdown memory system for coding agents — docs, frontmatter, categorized notes — and it worked because agents could inspect and maintain their own context. AKG is me pulling that pattern out into a general file format: structured, portable agent memory with explicit relationships, instead of treating memory as fuzzy semantic retrieval.

### Casual networking version
I got interested in agent memory from a very unglamorous place: I had coding agents working off a markdown knowledge base in a repo, and it actually worked really well. Then I saw a startup talking about the problem of helping agents keep useful context over long periods of time, and it clicked that the same pattern should exist outside codebases too — for support agents, assistants, whatever. So I started designing AKG, which is basically a portable file format for structured agent memory — explicit nodes and relationships, durable storage, and a cleaner model than just throwing everything into embeddings and hoping retrieval works.

### More technical founder version
AKG is a spec-first, single-file knowledge graph format for agent memory. The interesting part isn’t that the binary format is exotic — it isn’t — it’s that it makes a different bet than most memory systems. Instead of treating memory as semantic similarity over blobs, it treats memory as structured state with explicit records, relations, tags, timestamps, WAL-backed durability, compaction, and conformance-tested portability. It came from a markdown memory workflow I’d already validated in coding agents, and I’m now turning that into a real format plus reference implementation.

### Non-technical founder version
I’d found that agents work better when they can manage a structured memory they can inspect and update themselves, instead of relying only on giant prompts or fuzzy search. Then I started thinking about the same need in things like support agents or personal assistant-style agents, where you want long-lived context without standing up heavy retrieval infrastructure. AKG is my attempt to turn that into a portable format that could work across tools and environments, not just inside one repo.

### “Hire me, not insane” version
I’m not trying to invent magical AGI memory. I started from a very practical workflow that already worked — markdown-based memory for coding agents — and then asked how that same pattern could work for things like support agents or assistants without a heavy RAG stack. The point is to give agents durable, structured context in a way that’s inspectable and interoperable, and to be honest about where semantic retrieval helps versus where explicit structure is better.

### One default best version
I started with a markdown-based memory system for coding agents — basically a repo-local knowledge base the agent could inspect, update, and use while working — and it worked well enough that I wanted the same idea outside coding workflows. A startup job post about helping agents keep useful context over long time horizons helped crystallize that for me: support agents, assistants, workout agents, whatever, should be able to keep durable context without needing a heavy RAG setup. So I’m building AKG, a portable single-file format for structured agent memory. The point isn’t that the file format itself is magical; it’s that I think a lot of agent memory should be explicit and structured rather than treated as pure vector search. It’s a spec-first project right now, with the MVP mostly finalized and the reference implementation starting now.

## 3. Follow-up answers

### Why are you building this?
Because I had a repo-based markdown memory workflow that genuinely helped coding agents, and I wanted to preserve that good part — agents managing explicit context — without tying it to a codebase. The broader goal is something lightweight that can live with the agent itself and still persist across sessions.

### What is interesting about it?
It makes a different bet from a lot of AI memory work. Instead of assuming memory should mostly be fuzzy retrieval over text, it treats memory as structured state the agent can inspect, maintain, and traverse directly.

### How is it different from existing approaches?
It is more portable and explicit than ad hoc app-specific memory layers, and much more structured than “embed everything and retrieve by similarity.” It is also meant to be lightweight enough to live in-process with the agent while still surviving across sessions, and it is designed as a file format, so interoperability and inspectability are first-class.

### What did you learn building it?
That a lot of useful agent memory is less about clever retrieval and more about good structure, explicit relationships, and tight constraints. Also that if you want something interoperable, you have to think early about failure modes, validation, and conformance rather than hand-wave them.

### Why does this make you a good fit for an early team?
Because it shows I can start from observed behavior in a real workflow, extract the durable principle, and turn it into a scoped technical system. I have strong opinions, but they’re grounded in implementation and constraints rather than ideology.

### Why not just use RAG?
I see it as complementary to RAG, not a replacement for it. RAG is great when you need to retrieve relevant information from a large corpus. AKG is for durable, structured working memory: explicit facts, relationships, preferences, decisions, and state that an agent should carry across sessions. In a serious system, you would often want both.

## 4. Conversation guidance

### 5 things not to say
- “This solves memory for AGI.”
- “Vector search is useless.”
- “It’s basically a revolutionary new database.”
- “The format itself is the breakthrough.”
- “I already know this is the universal standard agents will use.”

### 5 good pivots back to their company/team
- How are you currently handling memory or long-lived context in your agents?
- Where have you found structure beats retrieval, or vice versa?
- Do you think your team needs more durable state, or better orchestration around transient context?
- Are you mostly building product behavior right now, or infra primitives under it?
- What kinds of technical taste decisions matter most on your team at this stage?

### 3 ways to describe it without going deep into implementation
- A portable memory format for agents
- Structured working memory for agents, instead of memory as pure semantic search
- A way to take the useful parts of repo-local agent knowledge bases and make them general across support agents, assistants, and other long-lived agent workflows

## 5. Optional notes

### Ambiguities to avoid
- Don’t imply there is already broad adoption or a mature ecosystem around AKG.
- Don’t blur “spec mostly finalized” into “finished product.”
- Don’t let people think this is anti-embedding in all contexts; it is a scoped bet about memory design.

### Claims to be careful not to overstate
- That structured memory is always better than vector-based retrieval
- That portability alone creates a market
- That the reference implementation is already done

### Recommended default version for tomorrow
Use the default best version, then quickly add: “The reason I think it’s worth building is that the underlying pattern was already validated in a markdown-based workflow for coding agents.” That gives you both honesty and proof of groundedness.
