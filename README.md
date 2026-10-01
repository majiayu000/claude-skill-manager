> **Status:** no new features. Install skills from [spellbook](https://github.com/majiayu000/spellbook). The public index is [claude-skill-registry](https://github.com/majiayu000/claude-skill-registry).

# sk - Claude Skills Manager

<p align="center">
  <a href="https://github.com/majiayu000/claude-skill-manager/releases"><img src="https://img.shields.io/github/v/release/majiayu000/claude-skill-manager?style=flat-square" alt="Release"></a>
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square" alt="PRs Welcome">
</p>

> **npm for Claude Code Skills** — The package manager for Claude Code skills

```
   _____ __ __
  / ___// //_/
  \__ \/ ,<
 ___/ / /| |
/____/_/ |_|
```

## Why sk?

- 🚀 **One-command install** — `sk install user/repo`
- 🔄 **Batch update** — `sk update` (coming soon)
- 🔍 **Smart search** — `sk search testing`
- 🩺 **Health check** — `sk doctor` to find issues
- 🎨 **Beautiful TUI** — Built with [Charm](https://charm.sh)

## Quick Start

### Install

```bash
# Using Go
go install github.com/majiayu000/claude-skill-manager@latest
# Go names the executable claude-skill-manager; expose the documented sk command.
sk_bin_dir="$(go env GOBIN)"
if [ -z "$sk_bin_dir" ]; then sk_bin_dir="$(go env GOPATH)/bin"; fi
mv "$sk_bin_dir/claude-skill-manager" "$sk_bin_dir/sk"
# Ensure sk_bin_dir is on your PATH.

# Or download a binary archive from the latest GitHub release.
```

### Release status

`v0.3.0` is the first asset-backed release for manifest/shard registry support.
Use Go install or download a release archive from GitHub.

The expected archive names are:

```bash
# macOS (Apple Silicon)
curl -L https://github.com/majiayu000/claude-skill-manager/releases/latest/download/sk_darwin_arm64.tar.gz | tar xz
sudo mv sk /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/majiayu000/claude-skill-manager/releases/latest/download/sk_darwin_amd64.tar.gz | tar xz
sudo mv sk /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/majiayu000/claude-skill-manager/releases/latest/download/sk_linux_amd64.tar.gz | tar xz
sudo mv sk /usr/local/bin/
```

### Usage

```bash
# Install a skill from GitHub
sk install anthropics/skills/skills/docx
sk install obra/superpowers
sk install https://github.com/user/repo

# Install a skill by registry name
sk install docx

# List installed skills
sk list

# Search for skills
sk search           # Show popular skills
sk search testing   # Search by keyword

# Get skill details
sk info my-skill

# Remove a skill
sk uninstall my-skill

# Check health
sk doctor
sk doctor --registry

# Update skills
sk update            # Planned; currently prints manual reinstall guidance
```

## Demo

```
$ sk list

📦 Installed Skills

  NAME                       DESCRIPTION
───────────────────────────────────────────────────────────────────────────────
  docx                       Comprehensive document creation and editing...
  frontend-design            Create distinctive, production-grade frontend...
  superpowers                20+ battle-tested skills for Claude Code

  3 skill(s) installed
```

```
$ sk search

★ Popular Skills

Anthropic Official (anthropics/skills)
  Official Claude Code skills from Anthropic

    📦 docx                      sk install anthropics/skills/skills/docx
    📦 pdf                       sk install anthropics/skills/skills/pdf
    📦 pptx                      sk install anthropics/skills/skills/pptx
    ...
```

## Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `sk install <source>` | `i`, `add` | Install a skill from GitHub |
| `sk list` | `ls`, `l` | List installed skills |
| `sk search [keyword]` | `s`, `find` | Search for skills |
| `sk info <name>` | `show` | Show skill details |
| `sk uninstall <name>` | `rm`, `remove` | Remove a skill |
| `sk update [name]` | `up`, `upgrade` | Planned update flow; currently prints manual reinstall guidance |
| `sk doctor` | - | Check skills health |

## Supported Sources

```bash
# From registry by name
sk install docx

# Short format
sk install owner/repo              # Entire repo
sk install owner/repo/path         # Subdirectory (monorepo)

# Full URL
sk install https://github.com/owner/repo
sk install https://github.com/owner/repo/tree/main/path

# Examples
sk install anthropics/skills/skills/docx  # Official Anthropic skill
sk install obra/superpowers        # Community skill
```

## Find and install one Claude Code skill

Search the [public registry](https://majiayu000.github.io/claude-skill-registry/) or
[SkillsMP](https://skillsmp.com/), then open the original repository. Read its
`SKILL.md`, scripts and license before installing. A catalog entry is a discovery
pointer; its inclusion does not establish authorship, permission or safety.

For example, Anthropic keeps its document skill in
[`skills/docx`](https://github.com/anthropics/skills/tree/main/skills/docx), with
its own [license](https://github.com/anthropics/skills/blob/main/skills/docx/LICENSE.txt).
After reviewing that source:

```bash
sk install anthropics/skills/skills/docx
sk list
sk info docx
sk doctor
```

Use the exact repository subdirectory: `owner/repo/path` selects that path,
not a recursive search for a skill name. `sk info` shows an **installed** skill's
source and location; it does not preview remote content. `sk doctor` checks local
installation health, not the trustworthiness of third-party instructions.

`sk` manages skills in its configured Claude Code directory. For installation
across other agent runtimes, see the supported agents and selection options in
[Vercel's skills CLI documentation](https://github.com/vercel-labs/skills).

### Troubleshooting

- **`sk` is not found:** check that the Go-installed executable was renamed as
  shown above and that its directory is on `PATH`.
- **Registry search is unavailable:** run `sk doctor --registry` to inspect the
  configured registry and cache. Registry requests require network access;
  fallback featured results are not the complete catalog.
- **A skill is already installed:** inspect it with `sk info <name>`. To replace
  it with a reviewed source, use `sk install <source> --force`. Automated
  `sk update` is not implemented.
- **Installation succeeds but the skill is missing:** compare the location from
  `sk info <name>` with the skills directory used by your Claude Code session.

Report reproducible CLI problems in
[GitHub issues](https://github.com/majiayu000/claude-skill-manager/issues).
Report problems with skill instructions to their original repository.

## Configuration

Skills are installed to `~/.claude/skills/` by default.

Config file: `~/.skrc`

```json
{
  "skills_dir": "~/.claude/skills",
  "registry": "https://raw.githubusercontent.com/majiayu000/claude-skill-registry/main",
  "registry_ttl_hours": 24
}
```

Registry cache:
- Location: `~/.cache/sk/registry.json`
- Search index cache: `~/.cache/sk/search-index.json`
- TTL: `registry_ttl_hours` (cache is ignored after expiry)

Registry verification:

```bash
bash scripts/smoke-registry.sh
```

## Limitations

- `sk update` is present as a command, but automated updates are not implemented
  yet. For now, reinstall with `sk uninstall <name> && sk install <source>`.
- Registry-backed search and install depend on the configured registry URL and
  network access. Featured search may show a small fallback list when the
  registry is unavailable.
- Installs are designed for public GitHub repositories and GitHub URLs. Private
  repositories, enterprise GitHub hosts, and authenticated downloads are not
  documented as supported.
- Installed skill content is copied into the configured local skills directory.
  `sk` does not sandbox, sign, or audit third-party skill content before use.

## Changelog

Release history and launch-readiness notes are tracked in [CHANGELOG.md](CHANGELOG.md).
The release checklist is documented in [docs/release.md](docs/release.md).
Registry consumer readiness is tracked in [docs/registry-consumer-spec.md](docs/registry-consumer-spec.md).

## Built With

- [Cobra](https://github.com/spf13/cobra) — CLI framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Style definitions
- [Huh](https://github.com/charmbracelet/huh) — Form components

## Contributing

PRs welcome!

1. Fork the repo
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.

---

<p align="center">
  Made with ❤️ for the Claude Code community
</p>
