#!/usr/bin/env sh
set -eu

# On pull requests, Gira copies the ticket's release-impact declaration to the
# PR body. A user-facing declaration must carry its reviewed Unreleased entry
# in the same change. Legacy PRs without the marker remain compatible.
body="${GIRA_PR_BODY:-}"
start='<!-- gira:release-impact:start -->'
end='<!-- gira:release-impact:end -->'

case "${body}" in
  *"${start}"*"${end}"*) ;;
  *) exit 0 ;;
esac

block="$(printf '%s\n' "${body}" | awk -v start="${start}" -v end="${end}" '
  $0 == start { inside = 1; next }
  $0 == end { exit }
  inside { print }
')"
impact="$(printf '%s\n' "${block}" | sed -n 's/^impact:[[:space:]]*//p' | head -n 1 | tr '[:upper:]' '[:lower:]')"
reason="$(printf '%s\n' "${block}" | sed -n 's/^reason:[[:space:]]*//p' | head -n 1)"

case "${impact}" in
  user-facing)
    base_ref="${GITHUB_BASE_REF:-main}"
    git fetch --no-tags --depth=1 origin "${base_ref}" >/dev/null
    changed_files="$(git diff --name-only FETCH_HEAD HEAD)"
    if ! printf '%s\n' "${changed_files}" | grep -Fx 'CHANGELOG.md' >/dev/null; then
      echo "release-impact=user-facing requires a CHANGELOG.md Unreleased entry in this PR" >&2
      exit 1
    fi
    ;;
  internal)
    ;;
  exempt)
    if [ -z "${reason}" ]; then
      echo "release-impact=exempt requires a non-empty reason" >&2
      exit 1
    fi
    ;;
  *)
    echo "release-impact marker must declare user-facing, internal, or exempt" >&2
    exit 1
    ;;
esac
