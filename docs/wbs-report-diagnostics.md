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

## Structural vs Execution Reports

The default WBS report is structural. It preserves hierarchy order and is best
for answering "what belongs under which epic?"

```bash
gira report wbs --repo OWNER/app --format csv
```

Structural CSV keeps the tree columns first:

```csv
wbs_id,parent_id,level,kind,repo,issue,title,state,status,priority,owner,milestone,start_date,target_date,progress,children,source,url
1,,1,epic,OWNER/app,10,Checkout rebuild,open,ready,p1,,M1,,2026-07-01,50,2,epic,https://github.com/OWNER/app/issues/10
1.1,1,2,task,OWNER/app,11,Payment form,open,ready,p1,kim,M1,,2026-07-01,0,0,checklist,https://github.com/OWNER/app/issues/11
```

Execution mode emits rows for planning spreadsheets. Summary containers are
marked separately from actionable rows, and diagnostics call out missing owner,
date, parent, and dependency metadata.

```bash
gira report wbs --repo OWNER/app --mode execution --format csv
```

Execution CSV keeps the Sheet-oriented planning columns first:

```csv
phase,workstream,task,owner,start_date,due_date,status,priority,dependency,milestone,issue_url,issue,item_type,row_type,week,source_due_date,scenario_due_date,delta_days,diagnostics
M1,backend,Payment form,kim,,2026-07-01,ready,p1,#9,M1,https://github.com/OWNER/app/issues/11,11,task,actionable,2026-06-29,2026-07-01,,,
```

For date-first planning, use the schedule report. It sorts by due date and week
bucket instead of WBS hierarchy order.

```bash
gira report schedule --repo OWNER/app --by week --format csv
```

Scenario output keeps source and planning dates separate so a compressed plan can
be reviewed without overwriting the current GitHub dates.

```bash
gira report schedule --repo OWNER/app --scenario one-month --format json
```
