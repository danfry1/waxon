#!/usr/bin/env sh
# Refresh vendorHash in flake.nix after go.mod/go.sum change.
# Builds the Go module derivation with a placeholder hash, reads the hash
# nix reports, writes it back, and rebuilds to confirm. Needs nix.
set -eu
cd "$(dirname "$0")/.."
placeholder="sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
current=$(sed -n 's/^ *vendorHash = "\([^"]*\)";/\1/p' flake.nix)
sed -i.bak "s|vendorHash = \"$current\"|vendorHash = \"$placeholder\"|" flake.nix && rm -f flake.nix.bak
# The mismatch error carries the real hash on a "got:" line.
got=$(nix build .#default --no-link 2>&1 | sed -n 's/^ *got: *\(sha256-[A-Za-z0-9+/=]*\).*/\1/p' | head -1 || true)
if [ -z "$got" ]; then
  # Either the placeholder happened to be right (impossible) or the build
  # failed for another reason: restore and surface it.
  sed -i.bak "s|vendorHash = \"$placeholder\"|vendorHash = \"$current\"|" flake.nix && rm -f flake.nix.bak
  echo "could not determine vendorHash; run 'nix build .#default' for details" >&2
  exit 1
fi
sed -i.bak "s|vendorHash = \"$placeholder\"|vendorHash = \"$got\"|" flake.nix && rm -f flake.nix.bak
if [ "$got" = "$current" ]; then
  echo "vendorHash unchanged ($got)"
else
  echo "vendorHash: $current -> $got"
fi
nix build .#default --no-link
echo "nix build OK"
