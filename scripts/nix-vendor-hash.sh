#!/usr/bin/env sh
# Refresh vendorHash in flake.nix after go.mod/go.sum change, using
# nix-update (the nix-community tool for exactly this), then build to
# confirm. Needs nix with flakes; nix-update is fetched on demand.
set -eu
cd "$(dirname "$0")/.."
nix run nixpkgs#nix-update -- --flake --version=skip default
nix build .#default --no-link
echo "nix build OK: $(sed -n 's/^ *vendorHash = "\([^"]*\)";/\1/p' flake.nix)"
