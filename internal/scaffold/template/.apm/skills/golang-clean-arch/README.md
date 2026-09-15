# golang-clean-arch skill bundle

This bundle generalizes the original business-focused Clean Architecture skill
into a capability-oriented standard that supports three project archetypes:

1. business applications;
2. technical applications;
3. libraries / SDKs.

`SKILL.md` contains universal rules. Load only the reference that matches the
current architectural decision.

## Files

```text
golang-clean-arch/
├── SKILL.md
└── references/
    ├── boundary-contracts.md
    ├── business-application.md
    ├── cross-feature-collaboration.md
    ├── library-sdk.md
    ├── samber-do-composition.md
    └── technical-application.md
```

Version `2.0.0` is used because broadening the skill from mandatory
business-feature architecture to archetype-selected architecture changes the
meaning of several existing rules.
