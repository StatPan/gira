# 크론 즉시 집행 가능한 이슈 3개 (v2)

> 목적: GitHub-native PM MVP를 실제로 진행하기 위한 실행형 이슈 초안.
> 원칙: MVP 경계 준수(Projects v2 자동화 비목표), gh-first, idempotency, blocker_format 고정.

---

## Issue 1
### title
[Task] PRD v2 고정: MVP 경계/gh-first/idempotency/차단 포맷 명문화

### goal
기존 PRD를 v2 실행형 기준으로 고정하여, 자동 실행 루프가 참조할 단일 규약 문서를 확정한다.

### scope
- `docs/prd/github-native-pm-maximization-prd-v2.md` 내용 확정
- MVP 비목표로 **Projects v2 자동화 제외**를 명시
- `gh` CLI 우선 전략과 멱등성 원칙 명시
- 고정 blocker 포맷 포함
- 코드 변경/기능 추가는 제외

### files_to_change
- `docs/prd/github-native-pm-maximization-prd-v2.md`

### verification_commands
- `git diff --check`
- `test -f docs/prd/github-native-pm-maximization-prd-v2.md`
- `rg "Projects v2 자동화|gh-first|idempotency|BLOCKED:" docs/prd/github-native-pm-maximization-prd-v2.md`

### acceptance_criteria
- [ ] PRD v2에 MVP 범위/비범위가 분리되어 있다.
- [ ] 비목표에 "Projects v2 자동화"가 명시되어 있다.
- [ ] gh-first 원칙이 명시되어 있다.
- [ ] idempotency 원칙과 예시가 명시되어 있다.
- [ ] blocker_format이 정확한 문자열로 포함되어 있다.

### blocker_format
`BLOCKED: <reason> | needed: <specific decision/input> | owner: <person/role>`

---

## Issue 2
### title
[Task] 크론 실행형 이슈 계약 템플릿 표준안 문서화

### goal
모든 작업 이슈가 동일한 실행 계약(title/goal/scope/files_to_change/verification_commands/acceptance_criteria/blocker_format)을 따르도록 템플릿 표준안을 문서화한다.

### scope
- 실행 계약 필수 필드 정의
- 각 필드 작성 규칙(단일 목표, 파일 경계, 검증 가능 명령) 명시
- 좋은 예시/나쁜 예시 간단 비교 포함
- GitHub 게시 자동화 구현은 제외

### files_to_change
- `docs/ops/issue-drafts-github-native-pm-v2.md`
- `docs/prd/github-native-pm-maximization-prd-v2.md`

### verification_commands
- `git diff --check`
- `rg "title|goal|scope|files_to_change|verification_commands|acceptance_criteria|blocker_format" docs/prd/github-native-pm-maximization-prd-v2.md`
- `rg "Issue 2|blocker_format" docs/ops/issue-drafts-github-native-pm-v2.md`

### acceptance_criteria
- [ ] 실행 계약 7개 필드가 누락 없이 정의되어 있다.
- [ ] 필드별 작성 규칙이 문서화되어 있다.
- [ ] blocker_format이 고정 문자열로 제시되어 있다.
- [ ] 템플릿 규약이 크론 집행 가능 관점으로 서술되어 있다.

### template_standard_notes
- 고정 필드 순서: `title → goal → scope → files_to_change → verification_commands → acceptance_criteria → blocker_format`
- 좋은 예시/나쁜 예시를 함께 제시해 작성 편차를 줄인다.
- 각 acceptance 항목은 검증 명령 또는 산출물 파일과 직접 매핑되어야 한다.
- 규약 설명은 "에이전트가 즉시 실행 가능한가" 기준으로 작성한다.

### blocker_format
`BLOCKED: <reason> | needed: <specific decision/input> | owner: <person/role>`

---

## Issue 3
### title
[Task] MVP 운영 규칙 확정: 라벨/마일스톤/PR 링크 게이트

### goal
MVP 범위 내에서 반복 실행 가능한 GitHub 운영 최소규칙(라벨 taxonomy, 마일스톤 cadence, PR 링크 게이트)을 확정한다.

### scope
- 라벨 최소 taxonomy(priority/type) 정의
- 마일스톤 cadence/네이밍/이월 규칙 정의
- PR 본문 `Closes #N` 필수 규칙 명시
- Projects v2 관련 자동화 규칙 추가는 제외

### files_to_change
- `docs/prd/github-native-pm-maximization-prd-v2.md`
- `docs/ops/issue-drafts-github-native-pm-v2.md`

### verification_commands
- `git diff --check`
- `rg "priority/P0|priority/P3|milestone|rollover|Closes #N" docs/prd/github-native-pm-maximization-prd-v2.md`
- `rg "Issue 3|Projects v2" docs/ops/issue-drafts-github-native-pm-v2.md`

### acceptance_criteria
- [ ] priority/type 라벨 최소셋이 정의되어 있다.
- [ ] 마일스톤 cadence와 네이밍 규칙이 명시되어 있다.
- [ ] 미완료 이슈 이월(rollover) 규칙이 명시되어 있다.
- [ ] PR 링크 규칙(`Closes #N`)이 포함되어 있다.
- [ ] Projects v2 자동화가 비목표로 유지된다.

### blocker_format
`BLOCKED: <reason> | needed: <specific decision/input> | owner: <person/role>`
