#!/usr/bin/env sh
# Print the CHANGELOG.md section for a version (e.g. "1.6.0" or "v1.6.0"),
# without the heading, for use as release notes. Exits 1 if absent.
set -eu
version="${1#v}"
awk -v ver="$version" '
  /^## \[/ {
    if (found) exit
    if (index($0, "## [" ver "]") == 1) { found = 1; next }
  }
  found { print }
' CHANGELOG.md | sed -e :a -e '/^\n*$/{$d;N;ba' -e '}' > /tmp/changelog-section.$$ || true
if [ ! -s "/tmp/changelog-section.$$" ]; then
  echo "CHANGELOG.md has no section for $version" >&2
  rm -f "/tmp/changelog-section.$$"
  exit 1
fi
cat "/tmp/changelog-section.$$"
rm -f "/tmp/changelog-section.$$"
