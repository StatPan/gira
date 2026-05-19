#!/usr/bin/env sh
set -eu

version="${1:?version is required}"
out_dir="${2:-release-assets}"
name="gira_${version}_docs"

mkdir -p "${out_dir}" "dist/${name}"
rm -rf "dist/${name}"
mkdir -p "dist/${name}/docs/skills" "dist/${name}/docs-site"

cp docs/skills/gira-agent-operator.md "dist/${name}/docs/skills/"
cp docs-site/agent-operator-skill.md "dist/${name}/docs-site/"
cp docs-site/command-reference.md "dist/${name}/docs-site/"
cp docs-site/ticket-workflow.md "dist/${name}/docs-site/"
cp docs/dx.md "dist/${name}/docs/"
cp CHANGELOG.md "dist/${name}/"

cat >"dist/${name}/README.md" <<EOF
# Gira ${version} Docs Contract Snapshot

This archive preserves the agent skill and generated documentation contract for
Gira ${version}.

- Canonical skill: docs/skills/gira-agent-operator.md
- Docs-site agent skill page: docs-site/agent-operator-skill.md
- Command reference: docs-site/command-reference.md
- Ticket workflow docs: docs-site/ticket-workflow.md
- Docs/release policy: docs/dx.md
EOF

tar -C dist -czf "${out_dir}/${name}.tar.gz" "${name}"
echo "${out_dir}/${name}.tar.gz"
