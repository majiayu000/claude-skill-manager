# Registry Consumer Readiness Spec

## Status

Draft for the `sk` CLI readiness work after the Claude Skills Registry moved
large artifacts to manifest and shard payloads.

## Problem

The registry no longer exposes every large payload as one direct JSON document.
Compatibility entry points can now be pointer files with
`deprecated_full_payload: true` and a `manifest` path. The CLI must consume the
current registry reliably, make failures actionable, and have a release path
that users can install without building from source.

The local source on `main` already contains initial manifest and shard support
for search and categories. Full registry loading still needs first-class
manifest and shard support. The remaining work is to formalize the contract,
close verification gaps, and ship a tagged release with downloadable assets.

## Goals

- Support current registry pointer, manifest, shard, and gzip variants for the
  user-facing CLI paths.
- Keep search and install-by-name working against the default public registry.
- Make cache, fallback, and network behavior visible enough to debug.
- Publish a release users can install with `go install ...@latest` and GitHub
  release archives.
- Track the work as small GitHub issues with clear acceptance criteria.

## Non-Goals

- `sk update` implementation for installed skills.
- Authenticated GitHub, private repositories, or enterprise GitHub hosts.
- Security scanning, signing, sandboxing, or trust policy for third-party skill
  content.
- Changing registry generation logic in `claude-skill-registry-core`.
- Editing generated `claude-skill-registry` artifacts directly.

## Current Artifact Contract

The CLI should treat these as supported public inputs:

- Registry mirror:
  - `registry.json` can be either a legacy full payload or a pointer.
  - `registry-manifest.json` lists full registry shards.
  - `registry-shards/*.json` contain full skill records.
- Pages search:
  - `search-index.json` can be either a compact search payload or a pointer.
  - `search-index.json.gz` remains a compatibility fallback when present.
  - `search-index-manifest.json` lists compact search shards.
  - `search-shards/part-*.json` and `*.json.gz` contain compact `s` records.
- Categories:
  - `categories/<name>.json` can be either a category payload or a pointer.
  - `categories/<name>/manifest.json` lists category parts.
  - `categories/<name>/part-*.json` and `*.json.gz` contain category records.

Pointer files must not be treated as empty registries. If a manifest path is
present, the CLI follows it. If a pointer is unsupported in a code path, the
error must say which payload moved and which command or API path to use.

Full registry shards may not include an explicit `install` field. Consumers
that assemble a full registry must synthesize an installable ref from
`repo`, `path`, and `branch` when needed. Search and category commands should
continue to use their smaller artifacts on the happy path rather than loading
all full registry shards.

## User-Facing Behavior

### Search

`sk search <keyword>` loads the compact search index, follows manifest shards,
filters non-installable refs, deduplicates by install command, and displays the
top results.

Acceptance:

- A cold-cache `sk search testing` returns registry results from the public
  default registry.
- Search works when `search-index.json` is a pointer.
- Search still works when only `search-index.json.gz` is available.
- The cache path is documented and TTL is honored.

### Category Search

`sk search --category <name>` follows category manifests when category entry
points are pointers.

Acceptance:

- `sk search --category testing` returns category results from sharded category
  payloads.
- Empty or missing category manifests fail with a specific error.
- Large categories are capped in display without losing the total count.

### Install by Registry Name

`sk install <name>` resolves names through the compact search index, follows
manifest shards, rejects command markdown refs, then downloads the installable
GitHub skill path.

Acceptance:

- A cold-cache `sk install <known-skill> --name <temp-name>` resolves from the
  current public registry.
- Command markdown refs are not installed as skills.
- The registry lookup failure is shown alongside GitHub URL parse failure.

### Full Registry API

`FetchRegistryWithSource` follows full registry pointers and assembles
`Registry.Skills` from `registry-manifest.json` and `registry-shards/*`.

Acceptance:

- `registry.json` pointer payloads are not cached as complete registries.
- Full registry shards can be loaded through `gzip_path` or `path`.
- Missing or failing shard paths return actionable errors.
- Records without `install` still produce installable refs when `repo` and
  `path` are available.

### Release

The project has module tags `v0.1.0` and `v0.2.0`, but no GitHub release assets
are published. The next release should be `v0.3.0` unless another SemVer reason
appears.

Acceptance:

- CI passes on the release commit.
- `v0.3.0` is tagged from the commit that includes manifest/shard support.
- GitHub release assets include the archive names documented in README.
- `go install github.com/majiayu000/claude-skill-manager@latest` resolves to the
  new version after the tag is visible to the Go proxy.
- One downloaded archive runs `sk --help`, `sk doctor`, and a cold-cache
  `sk search testing`.

## Issue Breakdown

### Issue 1: Implement full registry manifest and shard consumption

Tracking: #9

Scope:

- Follow `registry.json` pointers to `registry-manifest.json`.
- Assemble full registry shards into `Registry.Skills`.
- Prefer `gzip_path` with fallback to `path`.
- Synthesize install refs from full registry records where needed.

Acceptance:

- `FetchRegistryWithSource` returns assembled skills from manifest shards.
- Pointer-only payloads are not saved as full registry cache.
- Search still uses compact search shards on the happy path.

### Issue 2: Harden registry artifact contract tests

Tracking: #6

Scope:

- Add or extend unit tests for pointer files, manifests, plain shards, gzip
  shards, empty manifest paths, and non-200 shard errors.
- Cover full registry, search, category, and install-by-name resolution.

Acceptance:

- `go test ./...` covers manifest happy paths and failure messages.
- Tests do not hit the live network.
- Error messages include the failing artifact path.

### Issue 3: Add cold-cache CLI smoke verification

Tracking: #7

Scope:

- Add a documented smoke checklist, and optionally a script, that verifies the
  CLI against the public registry from a temporary HOME/cache.
- Include `search`, `category`, `install --name`, and `doctor` commands where
  safe.

Acceptance:

- The smoke flow can be run without mutating the user's real `~/.claude/skills`.
- Output proves whether data came from remote or cache.
- The command list is copied into release verification docs.

### Issue 4: Publish v0.3.0 release assets

Tracking: #5

Scope:

- Update changelog/release docs for the first asset-backed release.
- Tag `v0.3.0`.
- Verify the release workflow and downloadable archives.

Acceptance:

- GitHub release exists for `v0.3.0`.
- Archive install commands from README work.
- Go install `@latest` resolves to `v0.3.0`.

### Issue 5: Improve cache and diagnostics UX

Tracking: #8

Scope:

- Add a small user-facing way to see registry source, cache path, TTL, and
  current registry URL.
- Consider flags such as `--no-cache` or `sk doctor --registry`.

Acceptance:

- A user can tell whether `sk search` used remote data or a cache.
- A stale or malformed cache has an actionable recovery path.
- Existing default behavior remains backward compatible.

## First PR Scope

This PR is intentionally documentation and tracking only:

- Add this spec.
- Correct release-readiness docs to reflect existing module tags and missing
  GitHub release assets.
- Link the spec from README and CHANGELOG.
- Open GitHub issues for the five implementation tracks above.

Implementation changes should land in follow-up PRs mapped to the issues.

## Verification

Local:

```bash
go test ./...
HOME=/tmp/sk-home-verify /path/to/sk search testing
HOME=/tmp/sk-home-verify /path/to/sk search --category testing
```

Release:

```bash
gh run list --repo majiayu000/claude-skill-manager --limit 5
gh release view v0.3.0 --repo majiayu000/claude-skill-manager
go list -m -versions github.com/majiayu000/claude-skill-manager@latest
```
