# Premise
Repository-native product work needs explicit PM intent before implementation.

# Actor
The product manager handing work to an AI or human worker.

# Problem
Raw requests lose product intent and evidence as they move into implementation.

# Desired Outcome
Workers receive a reviewable, source-grounded product contract.

# Constraints
- Keep GitHub as the execution backend.
- Keep compilation read-only.

# Non-goals
- Generate implementation code.

# Authority
- The compiler may diagnose but must not approve scope changes.

# Evidence
- docs/pm-operating-policy.md
- issue #857

# Assumptions
- Markdown headings are an acceptable authoring interface.

# Decision Debt
- Decide whether later versions accept additional document formats.

# Success Conditions
- Complete input compiles without missing-field diagnostics.
- JSON preserves every supplied statement and source span.

# Candidate Work
- Parse recognized PM sections.
- Emit deterministic diagnostics.
