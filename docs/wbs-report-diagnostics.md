# WBS Report Diagnostics

`gira report wbs` keeps the compact text and CSV output stable, but JSON, HTML,
and bundle output include structured diagnostics when parent inference is
ambiguous.

## Before

Older output exposed ambiguous milestone parent inference only as a warning
string. That made the problem visible, but not directly actionable.

```json
{
  "schema_version": "wbs-report/v1alpha1",
  "warnings": [
    "ambiguous_milestone_parent:M1"
  ],
  "warning_items": []
}
```

## After

The report now includes candidate parent issues, affected child issues, evidence
strength, and a remediation hint.

```json
{
  "schema_version": "wbs-report/v1alpha1",
  "warnings": [
    "ambiguous_milestone_parent:M1"
  ],
  "warning_items": [
    {
      "code": "ambiguous_milestone_parent",
      "warning": "ambiguous_milestone_parent:M1",
      "milestone": "M1",
      "candidate_parents": [
        {
          "issue": 10,
          "title": "Planning epic",
          "evidence": ["related", "milestone"],
          "strength": "weak"
        },
        {
          "issue": 11,
          "title": "Delivery epic",
          "evidence": ["checklist", "milestone"],
          "strength": "strong"
        }
      ],
      "affected_children": [
        {
          "issue": 12,
          "title": "Ambiguous child",
          "candidate_parents": [10, 11],
          "evidence": ["related", "milestone"],
          "resolution_reason": "ambiguous_parent_candidates"
        }
      ],
      "remediation": "Add explicit Parent: #EPIC to affected child issues, convert weak Related links to epic checklist items, or leave multiple root epics in the milestone intentionally."
    }
  ]
}
```

## Item Metadata

Child WBS items may also include parent resolution metadata:

```json
{
  "issue": 13,
  "parent_source": "checklist,milestone",
  "parent_resolution_reason": "selected_unique_strongest_candidate",
  "parent_candidates": [
    {
      "issue": 11,
      "evidence": ["checklist", "milestone"],
      "strength": "strong"
    }
  ]
}
```

Evidence strength is intentionally simple:

- `strong`: explicit `Parent: #...` references or epic checklist items.
- `inferred`: generic body references.
- `weak`: `Related: #...` references or shared milestone fallback.

