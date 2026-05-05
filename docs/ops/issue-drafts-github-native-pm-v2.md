# Three Immediately Executable Issues (v2)

> Purpose: executable issue drafts for running the GitHub-native PM MVP.
> Principles: keep MVP boundaries, exclude Projects v2 automation, use gh-first execution, preserve idempotency, and keep blocker_format fixed.

---

## Issue 1

### title

[Task] Freeze PRD v2: document MVP boundaries, gh-first execution, idempotency, and blocker format

### goal

Freeze the existing PRD as the v2 executable contract so automation loops have one canonical operating document.

### scope

- Finalize `docs/prd/github-native-pm-maximization-prd-v2.md`.
- Explicitly exclude **GitHub Projects v2 automation** from MVP scope.
- Document the `gh` CLI first strategy and idempotency principle.
- Include the fixed blocker format.
- Exclude code changes and feature implementation.

### files_to_change

- `docs/prd/github-native-pm-maximization-prd-v2.md`

### verification_commands

- `git diff --check`
- `test -f docs/prd/github-native-pm-maximization-prd-v2.md`
- `rg "Projects v2 automation|gh-first|idempotency|BLOCKED:" docs/prd/github-native-pm-maximization-prd-v2.md`

### acceptance_criteria

- [ ] PRD v2 separates MVP scope from non-goals.
- [ ] Non-goals explicitly include "Projects v2 automation".
- [ ] The gh-first principle is documented.
- [ ] The idempotency principle and examples are documented.
- [ ] The exact blocker_format string is included.

### blocker_format

`BLOCKED: <reason> | needed: <specific decision/input> | owner: <person/role>`

---

## Issue 2

### title

[Task] Document the cron-executable issue contract template

### goal

Document the template standard that makes every work issue follow the same execution contract: title, goal, scope, files_to_change, verification_commands, acceptance_criteria, and blocker_format.

### scope

- Define required execution contract fields.
- Document writing rules for each field, including one outcome, file boundaries, and verifiable commands.
- Include short good and bad examples.
- Exclude GitHub publishing automation.

### files_to_change

- `docs/ops/issue-drafts-github-native-pm-v2.md`
- `docs/prd/github-native-pm-maximization-prd-v2.md`

### verification_commands

- `git diff --check`
- `rg "title|goal|scope|files_to_change|verification_commands|acceptance_criteria|blocker_format" docs/prd/github-native-pm-maximization-prd-v2.md`
- `rg "Issue 2|blocker_format" docs/ops/issue-drafts-github-native-pm-v2.md`

### acceptance_criteria

- [ ] All seven execution contract fields are defined.
- [ ] Field-level writing rules are documented.
- [ ] blocker_format is presented as a fixed string.
- [ ] The contract is written from the perspective of agent executability.

### template_standard_notes

- Fixed field order: `title -> goal -> scope -> files_to_change -> verification_commands -> acceptance_criteria -> blocker_format`.
- Every field must be present; reordering is not allowed.
- Include good and bad examples to reduce authoring variance.
- Each acceptance criterion must map directly to a verification command or produced artifact.
- The contract should be written around this question: "Can an agent execute this immediately?"

### good_example

```md
### title
[Task] Document execution contract template standard

### goal
Define the rule that every work issue must use the same seven fields.

### scope
- Define required fields
- Document field-level writing rules
- Exclude GitHub publishing automation

### files_to_change
- docs/ops/issue-drafts-github-native-pm-v2.md
- docs/prd/github-native-pm-maximization-prd-v2.md

### verification_commands
- git diff --check
- rg "title|goal|scope|files_to_change|verification_commands|acceptance_criteria|blocker_format" docs/prd/github-native-pm-maximization-prd-v2.md

### acceptance_criteria
- [ ] All seven fields are defined.
- [ ] blocker_format is included as a fixed string.

### blocker_format
BLOCKED: <reason> | needed: <specific decision/input> | owner: <person/role>
```

### bad_example

```md
### goal
Clean up docs

### scope
- Change what is needed

### verification_commands
- Check it
```

Problems: missing required fields, missing file boundaries, unverifiable command, and unclear completion criteria.

### blocker_format

`BLOCKED: <reason> | needed: <specific decision/input> | owner: <person/role>`

---

## Issue 3

### title

[Task] Freeze MVP operating rules: labels, milestones, and PR linkage gate

### goal

Define the minimum GitHub operating rules needed for repeatable MVP execution: label taxonomy, milestone cadence, and PR linkage gates.

### scope

- Define the minimum priority and type label taxonomy.
- Define milestone cadence, naming, and rollover rules.
- Require PR bodies to include `Closes #N`.
- Exclude additional Projects v2 automation rules.

### files_to_change

- `docs/prd/github-native-pm-maximization-prd-v2.md`
- `docs/ops/issue-drafts-github-native-pm-v2.md`

### verification_commands

- `git diff --check`
- `rg "priority/P0|priority/P3|milestone|rollover|Closes #N" docs/prd/github-native-pm-maximization-prd-v2.md`
- `rg "Issue 3|Projects v2" docs/ops/issue-drafts-github-native-pm-v2.md`

### acceptance_criteria

- [ ] The minimum priority and type labels are defined.
- [ ] Milestone cadence and naming rules are documented.
- [ ] Unfinished issue rollover rules are documented.
- [ ] PR linkage rules include `Closes #N`.
- [ ] Projects v2 automation remains a non-goal.

### blocker_format

`BLOCKED: <reason> | needed: <specific decision/input> | owner: <person/role>`
