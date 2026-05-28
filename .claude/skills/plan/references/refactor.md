## Refactor Planning Lens

Use this reference when planning internal structure changes without a primary net-new user feature.

Check for:

- the invariant behavior that must remain stable
- why the current structure is insufficient
- incremental cutover boundaries that reduce risk
- migration, compatibility, and rollback considerations
- the tests or verification needed to prove the refactor did not regress behavior

Do not hide behavior changes inside a refactor plan. If a new feature is also being introduced, make that explicit.
