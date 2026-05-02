# 실행형 PRD v2: GitHub-native PM Maximization (MVP/gh-first)

## 0. 문서 목적
본 문서는 gira가 GitHub를 실행 백엔드로 사용할 때, 사람이 읽고 크론 에이전트가 즉시 집행 가능한 운영 표준을 정의한다.
핵심은 **MVP 경계 준수**, **gh-first**, **idempotency(반복 실행 안정성)** 이다.
또한 본 문서는 운영 규약 확정 문서이며, 코드 변경/기능 추가 자체는 범위에 포함하지 않는다.

---

## 1. 문제 정의
현재 GitHub Issues/PR/Milestones 기반 운영은 가능하지만, 팀/담당자별 관례 차이로 인해 다음 문제가 발생한다.
- 이슈의 실행 밀도 불균일(목표/범위/검증/차단 조건 누락)
- 크론 에이전트 관점의 모호성(어떤 파일을 바꿔야 하는지, 어떻게 완료를 판정하는지 불명확)
- 반복 실행 시 드리프트(라벨/마일스톤/템플릿 규칙 불일치)

결과적으로 자동 루프의 성공률과 예측 가능성이 떨어진다.

---

## 2. 목표 (Goals)
1. GitHub 공식 기능(Issues, PRs, Labels, Milestones)만으로 Jira-style 운영을 실무 수준으로 표준화한다.
2. `gh` CLI 중심의 운영 계약을 확립한다.
3. 이슈를 크론이 바로 집행 가능한 형태로 고정한다.
4. 동일 명령 재실행 시 결과가 안정적인(idempotent) 운영 규칙을 보장한다.

---

## 3. 비목표 (Non-goals, MVP 제외)
아래 항목은 MVP 범위에서 명시적으로 제외한다.
- **GitHub Projects v2 자동화** (필드/보드/오토메이션 생성·동기화)
- Jira API 연동(가져오기/내보내기/양방향 동기화)
- 신규 Web UI 개발
- Slack/Discord PM 봇 연동
- LLM 기반 PRD→이슈 자동 분해

---

## 4. MVP 범위 (In-scope)
- 이슈 실행 계약(템플릿) 표준화
- 라벨 taxonomy 표준화(priority/type/status 최소셋)
- 마일스톤 cadence/네이밍/이월 규칙
- PR 링크 및 병합 게이트 최소 규칙(`Closes #N` 포함)
- 운영 문서/초안/검증 커맨드 정착

---

## 5. 핵심 원칙
### 5.1 gh-first
- GitHub 작업 자동화는 기본적으로 `gh` CLI를 우선 사용한다.
- API 직접 호출은 MVP에서 기본 전략이 아니다.

### 5.2 Idempotency-first
- 동일 이슈/명령을 여러 번 실행해도 결과가 깨지지 않아야 한다.
- 예시:
  - 이미 존재하는 라벨 생성 시 실패 대신 noop 또는 업데이트 수행
  - 이미 존재하는 마일스톤은 중복 생성하지 않음
  - 템플릿/문서 갱신 시 diff가 없으면 변경 없음으로 종료

### 5.3 명시적 차단 보고
- 자동 실행 중 의사결정/권한/외부입력이 필요하면 즉시 아래 포맷으로 차단 보고한다.
- **blocker_format (고정):**
  - `BLOCKED: <reason> | needed: <specific decision/input> | owner: <person/role>`

---

## 6. Jira-style 매핑 (MVP Canonical)
- Epic → Parent Issue
- Story/Task/Bug → Issue
- Sprint → Milestone
- 개발 연결 → PR 본문 `Closes #N`(또는 Fixes/Resolves)

주의: Projects v2의 상태필드/오토메이션 매핑은 MVP 비목표이므로 문서 레벨 참고만 허용하고 자동화 대상에서 제외한다.

---

## 7. 실행 계약 (Cron-executable Contract)
모든 실행 이슈는 아래 7개 필드를 반드시 포함해야 한다.
1. `title`
2. `goal`
3. `scope`
4. `files_to_change`
5. `verification_commands`
6. `acceptance_criteria`
7. `blocker_format`

### 필드 작성 규칙
- 필드 순서는 고정한다: `title → goal → scope → files_to_change → verification_commands → acceptance_criteria → blocker_format`
- goal: 단일 결과(One outcome)로 작성
- scope: 포함/제외 경계가 분명해야 함
- files_to_change: 상대경로 명시, 과도한 와일드카드 금지
- verification_commands: 로컬에서 재현 가능한 명령만 사용
- acceptance_criteria: 체크리스트 형태로 완료 조건 명시
- blocker_format: 고정 문자열 포맷을 그대로 포함

### 크론 집행 가능성 체크
- title은 작업 유형과 대상 산출물을 동시에 드러내야 한다.
- scope는 "무엇을 한다"와 함께 "무엇을 하지 않는다"를 최소 1개 이상 포함한다.
- verification_commands는 실행 성공/실패가 명확한 명령만 사용한다(자연어 설명 금지).
- acceptance_criteria는 검증 명령 또는 산출물 파일과 1:1 대응되도록 작성한다.

### 좋은 예시 (Good)
```md
### title
[Task] 이슈 실행 계약 템플릿 규칙 문서화

### goal
모든 작업 이슈가 동일한 7개 필드를 사용하도록 표준 규칙을 확정한다.

### scope
- 템플릿 필수 필드 정의
- 필드별 작성 규칙 명시
- 자동 게시 구현은 제외

### files_to_change
- docs/prd/github-native-pm-maximization-prd-v2.md

### verification_commands
- git diff --check
- rg "title|goal|scope|files_to_change|verification_commands|acceptance_criteria|blocker_format" docs/prd/github-native-pm-maximization-prd-v2.md

### acceptance_criteria
- [ ] 7개 필드 정의가 문서에 존재한다.
- [ ] blocker_format 고정 문자열이 포함된다.

### blocker_format
BLOCKED: <reason> | needed: <specific decision/input> | owner: <person/role>
```

### 나쁜 예시 (Bad)
```md
### goal
문서 좀 정리

### scope
- 필요한 것들 수정

### verification_commands
- 확인해보기
```

- 문제점: 필드 누락(title/files_to_change/acceptance_criteria/blocker_format), 검증 불가 문장, 경계 불명확.
- 이 템플릿 규약의 운영 초안은 `docs/ops/issue-drafts-github-native-pm-v2.md`의 Issue 2와 동기화한다.

---

## 8. 운영 정책 (MVP)
### 8.1 Labels
- 최소 taxonomy
  - `priority/P0`, `priority/P1`, `priority/P2`, `priority/P3`
  - `type/feature`, `type/bug`, `type/chore`, `type/docs`
- 라벨 동기화는 멱등적으로 수행한다(이미 있으면 유지/보정).

### 8.2 Milestones
- cadence: 주간 또는 격주 중 팀 단일 선택
- 네이밍: `YYYY-Www` 또는 `YYYY-MM-SprintN` 중 하나를 팀 표준으로 고정
- rollover: 미완료 이슈는 다음 milestone으로 이월하고 이유를 남긴다.

### 8.3 PR Merge Gate
- 필수: 관련 이슈 링크(`Closes #N`), 테스트/검증 통과
- 차단 리뷰 존재 시 자동 병합 금지
- 충돌/권한/정책 이슈는 blocker_format으로 에스컬레이션

---

## 9. 성공 지표 (MVP)
- Issue→PR 링크율 95%+
- 이슈 템플릿 필수 필드 충족률 95%+
- 재실행 시 오류 없는 작업 비율 증가
- Blocked 이슈 평균 체류시간 감소

---

## 10. 롤아웃 계획
1. 문서 고정: PRD v2 + 이슈 초안 3종 확정
2. 파일럿: 단일 저장소에서 1~2주 운영
3. 점검: 템플릿 충족률/링크율/차단 리드타임 측정
4. 확장: 다른 리포로 동일 규칙 확산

---

## 11. 완료 정의 (Definition of Done)
- PRD v2 문서가 MVP 경계, gh-first, idempotency, blocker_format을 명시한다.
- 크론 즉시 집행 가능한 이슈 3개가 표준 필드를 모두 충족한다.
- 팀이 문서만으로 동일 운영을 재현할 수 있다.
