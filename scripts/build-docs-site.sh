#!/usr/bin/env sh
set -eu

src="${1:-docs-site}"
out="${2:-site}"

if [ ! -d "${src}" ]; then
  echo "docs source directory not found: ${src}" >&2
  exit 1
fi

rm -rf "${out}"
node scripts/check-security-deps.mjs
npm exec -- vitepress build "${src}" --outDir "${out}"

if [ ! -f "${out}/index.html" ]; then
	echo "docs build failed: ${out}/index.html missing" >&2
	exit 1
fi

for page in install quickstart jira-mapping ticket-workflow sprint-release distribution troubleshooting; do
	if [ ! -f "${out}/${page}/index.html" ] && [ ! -f "${out}/${page}.html" ]; then
		echo "docs build failed: ${out}/${page} missing" >&2
		exit 1
	fi
done

if [ ! -f "${out}/CNAME" ]; then
	echo "docs build failed: ${out}/CNAME missing" >&2
	exit 1
fi

echo "docs site built: ${out}"
