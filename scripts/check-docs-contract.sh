#!/usr/bin/env sh
set -eu

root="${1:-.}"

(
	cd "${root}"
	if [ ! -x node_modules/.bin/vitepress ]; then
		npm ci
	fi
	go run ./scripts/refresh-docs-contract.go .
	go test ./internal/gira -run 'Test(CommandReferenceIsGeneratedFromRegistry|AgentOperatorDocsSiteIsGeneratedFromRegistry|AgentSkillManagedBlockIsGeneratedFromRegistry|AgentSkillManagedBlockIsUnique)$'
	sh scripts/build-docs-site.sh docs-site site
	git diff --exit-code -- docs-site/command-reference.md docs-site/agent-operator-skill.md docs/skills/gira-agent-operator.md
)
