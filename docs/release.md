# Release Readiness

## Current status

`v0.3.0` is the asset-backed manifest/shard registry compatibility release. The
repository uses GoReleaser through a tag-triggered GitHub Actions workflow.

## Release path

1. Confirm CI is green on `main`.
2. Choose the next SemVer tag. For the manifest/shard registry compatibility
   release, use `v0.3.0`.
3. Review `CHANGELOG.md` and move the intended notes out of `Unreleased`.
4. Create and push the tag:

   ```bash
   git tag v0.3.0
   git push origin v0.3.0
   ```

5. Wait for the `Release` workflow to finish.
6. Verify the GitHub release and downloadable assets:

   ```bash
   gh release view v0.3.0
   gh release download v0.3.0 --pattern 'sk_darwin_arm64.tar.gz' --dir /tmp/sk-release-check
   ```

7. Install the downloaded archive on at least one supported platform and run:

   ```bash
   sk --help
   sk doctor
   sk doctor --registry
   HOME=/tmp/sk-home-release-check sk search testing
   ```

For source-tree verification before a tag, run the cold-cache registry smoke
script. It builds `sk`, uses a temporary `HOME` and `XDG_CACHE_HOME`, runs
search/category/install-by-name flows, and leaves the user's real skill
directory untouched:

```bash
bash scripts/smoke-registry.sh
```

## Release blockers before advertising binary installs

- The asset-backed GitHub release must exist.
- Release assets must include the archive names used by the README.
- `go install github.com/majiayu000/claude-skill-manager@latest` must resolve to
  the new tag after it is visible to the Go proxy.
- The GoReleaser Homebrew upload path depends on `HOMEBREW_TAP_GITHUB_TOKEN`;
  if that token is unavailable, Homebrew publication should be documented as
  skipped or handled separately.
- `sk update` should remain documented as planned until the command performs an
  actual update.
