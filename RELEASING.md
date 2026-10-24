# Releasing waxon

How a release is cut, for maintainers and for agents working on the repo.
Pushing a `v*` tag runs `.github/workflows/release.yml`; everything below
follows from that.

## What the pipeline does

1. **release**: checks the tag has a `CHANGELOG.md` section and matches the
   `version` in `flake.nix`, then runs GoReleaser. That builds the archives
   and deb/rpm/apk packages, publishes the GitHub Release with the changelog
   section as its notes (`scripts/changelog-section.sh`), and pushes the Scoop
   manifest to `danfry1/scoop-bucket`.
2. **homebrew-formula**: opens a pull request on `danfry1/homebrew-tap`
   (branch `waxon-<version>`) that points the source-built `waxon.rb` at the
   tag's source archive and its sha256, and drops the old bottle block. The
   tap's CI (`brew test-bot`) builds and tests bottles on that PR for macOS 15+
   on Apple Silicon, x86_64 Linux and ARM Linux. The job posts the publish
   command as a comment on the PR.
3. **attest**: signs SLSA build provenance for the release assets. Anyone can
   check a download with `gh attestation verify <file> --repo danfry1/waxon`.

Nix users get the new version from the tag directly; there's nothing to
publish for Nix.

## Checklist

1. **Changelog.** Move the `[Unreleased]` notes in `CHANGELOG.md` under a new
   `## [x.y.z] - YYYY-MM-DD` heading and add its compare link at the bottom.
   The release fails if the tag has no section.
2. **Nix.** Set `version = "x.y.z"` in `flake.nix`; the release checks it
   matches the tag. If `go.mod` or `go.sum` changed since the last refresh,
   run `make nix-hash` (CI's `nix build` fails on a stale `vendorHash`).
3. **Merge** those changes to `main` through a PR (main is PR-only with
   required checks), and wait for CI on `main` to pass.
4. **Tag** the merge commit on `main` and push the tag:
   ```sh
   git tag vx.y.z
   git push origin vx.y.z
   ```
5. **Wait** for the bot PR on `danfry1/homebrew-tap` to go green (about 3–4
   minutes).
6. **Publish the bottles** by running the command the bot posted on the PR:
   ```sh
   gh workflow run publish.yml -R danfry1/homebrew-tap -f pull_request=<N> -f head_sha=<sha>
   ```
   This uploads and attests the bottles and updates the tap.
   **Never merge the bot PR with the merge button**: that ships a formula with
   no bottles, so every user compiles waxon from source.
7. **Check** that the release's `attest` job passed and that
   `brew upgrade waxon` pours a bottle rather than building.

## Credentials

The only release credentials are the `TAP_BUMP_APP_ID` repository variable
and the `TAP_BUMP_APP_PRIVATE_KEY` secret. They belong to the
`danfry1-tap-bumper` GitHub App, which has contents and pull-request write
access on `danfry1/homebrew-tap` and `danfry1/scoop-bucket` only. The release
job mints a token scoped to `scoop-bucket`; the formula job one scoped to
`homebrew-tap`. There is no personal access token.

## Homebrew notes

- waxon is a **formula** in the tap (`waxon.rb`, built from source with
  bottles). The old cask was removed and `tap_migrations.json` maps `waxon` to
  the tap's formula. Never re-add `Casks/waxon.rb`.
- Homebrew builds with `GOTOOLCHAIN=local`. Raising the `go` directive in
  `go.mod` above the Go version Homebrew currently ships breaks the formula
  build; the tap's CI on the bot PR is where that shows up.
