## Greenfield Planning Lens

Use this reference when planning a new project or a major net-new scaffold.

If the repository is empty or near-empty and the user explicitly wants day-0/bootstrap work, the plan should usually become a scaffold-oriented brief first. Do not default that request into feature implementation work unless the user is also asking for post-scaffold delivery work.

Check for:

- stack, runtime, framework, and package-manager choices
- bootstrap constraints that must shape the initial scaffold
- the minimum architecture needed to support the first planned epics
- explicit assumptions about dependencies and developer workflow
- which scaffold details belong in structured setup instructions versus free-form narrative notes
- whether any follow-on implementation work should be sequenced after scaffold generation rather than replacing the scaffold-oriented brief

Keep the initial scaffold minimal and aligned with the first delivery increments. Do not overdesign the repository before early epics justify it.
