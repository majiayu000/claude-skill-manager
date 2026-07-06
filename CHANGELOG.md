# Changelog

All notable changes to `sk` should be recorded here.

Release notes are grouped by GitHub tag.

## Unreleased

- No unreleased changes.

## v0.3.1 - 2026-07-06

### Changed

- Migrated the Go module path and documentation from `caude-skill-manager` to
  `claude-skill-manager` after the repository rename.

## v0.3.0 - 2026-06-24

### Added

- Documented release status, initial release checklist, and current limitations.
- Linked the README to release notes and release-readiness documentation.
- Added a registry consumer readiness spec for manifest/shard compatibility,
  release verification, and follow-up issue tracking.
- Added full registry manifest and shard consumption, including gzip shard
  preference with plain JSON fallback.
- Added registry artifact contract tests for pointer files, manifests, shards,
  cache rejection, install ref synthesis, and category fallback behavior.
- Added `sk doctor --registry` diagnostics for registry URL, config path, cache
  TTL, cache state, and cache recovery commands.
- Added a cold-cache `scripts/smoke-registry.sh` verification flow that uses a
  temporary `HOME` and cache directory.
- Added registry source reporting for search, category search, and
  install-by-name flows.

### Known limitations

- `sk update` is a placeholder command and does not update installed skills.
- Registry-backed commands depend on the configured registry URL and network
  access.
- Private GitHub repositories, enterprise GitHub hosts, authenticated downloads,
  signature verification, and sandboxing are not documented as supported.
