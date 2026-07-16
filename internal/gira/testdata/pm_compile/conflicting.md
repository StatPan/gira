# Actor
Repository maintainer.

# Problem
Mutating work can escape declared boundaries.

# Desired Outcome
Every mutation stays within its declared boundary.

# Constraints
- publish releases
- must not publish releases

# Evidence
- issue #859

# Success Conditions
- Conflicts are reported without choosing a statement.
