#!/usr/bin/env sh
# Print the CHANGELOG.md section for a version (e.g. "1.6.0" or "v1.6.0"),
# without the heading and without trailing blank lines, for use as release
# notes. Exits 1 if the version has no section. Pure awk for portability
# (GNU and BSD sed disagree on multi-line idioms).
set -eu
version="${1#v}"
out=$(awk -v ver="$version" '
  /^## \[/ {
    if (found) exit
    if (index($0, "## [" ver "]") == 1) { found = 1; next }
  }
  found {
    buf[n++] = $0
  }
  END {
    # Trim trailing blank lines.
    while (n > 0 && buf[n-1] ~ /^[[:space:]]*$/) n--
    for (i = 0; i < n; i++) print buf[i]
  }
' CHANGELOG.md)
if [ -z "$out" ]; then
  echo "CHANGELOG.md has no section for $version" >&2
  exit 1
fi
printf '%s\n' "$out"
