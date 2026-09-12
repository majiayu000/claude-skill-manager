package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/majiayu000/claude-skill-manager/internal/config"
	"go.yaml.in/yaml/v3"
)

// Skill represents an installed skill
type Skill struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Description string    `json:"description"`
	Source      string    `json:"source"` // github url or local
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
}

// SkillMeta represents metadata from SKILL.md front matter
type SkillMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// List returns all installed skills
func List() ([]Skill, error) {
	skillsDir := config.GetSkillsDir()

	// Restore skills left under .*.backup-* after an interrupted swap, then
	// drop leftover staging directories so they never appear as installs.
	recoverOrphanedInstallerDirs(skillsDir)

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Skill{}, nil
		}
		return nil, err
	}

	var skills []Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip installer-owned temp dirs (e.g. ".docx.staging-XXXX",
		// ".docx.backup-XXXX"). Do not skip all dotted names — custom installs
		// like --name .foo must remain discoverable via List/Get/Exists.
		if isInstallerTempDir(entry.Name()) {
			continue
		}

		skillPath := filepath.Join(skillsDir, entry.Name())
		skillMdPath := filepath.Join(skillPath, "SKILL.md")

		// Check if SKILL.md exists
		if _, err := os.Stat(skillMdPath); os.IsNotExist(err) {
			continue
		}

		skill := Skill{
			Name: entry.Name(),
			Path: skillPath,
		}

		// Parse SKILL.md for metadata
		if meta, err := parseSkillMd(skillMdPath); err == nil {
			if meta.Name != "" {
				skill.Name = meta.Name
			}
			skill.Description = meta.Description
		}

		// Get modification time as install time approximation
		if info, err := entry.Info(); err == nil {
			skill.InstalledAt = info.ModTime()
		}

		skills = append(skills, skill)
	}

	return skills, nil
}

// Get returns a specific skill by name
func Get(name string) (*Skill, error) {
	skills, err := List()
	if err != nil {
		return nil, err
	}

	for _, s := range skills {
		if s.Name == name || filepath.Base(s.Path) == name {
			return &s, nil
		}
	}

	return nil, nil
}

// Exists checks if a skill is installed
func Exists(name string) bool {
	skill, _ := Get(name)
	return skill != nil
}

// Remove uninstalls a skill
func Remove(name string) error {
	s, err := Get(name)
	if err != nil {
		return err
	}
	if s == nil {
		return os.ErrNotExist
	}

	// Check if exists
	if _, err := os.Stat(s.Path); os.IsNotExist(err) {
		return os.ErrNotExist
	}

	return os.RemoveAll(s.Path)
}

// parseSkillMd extracts metadata from SKILL.md front matter
func parseSkillMd(path string) (*SkillMeta, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	meta := &SkillMeta{}

	frontMatter, ok := extractFrontMatter(string(content))
	if !ok {
		return meta, nil
	}

	if err := yaml.Unmarshal([]byte(frontMatter), meta); err != nil {
		return nil, fmt.Errorf("invalid front matter in %s: %w", path, err)
	}

	meta.Name = strings.TrimSpace(meta.Name)
	meta.Description = strings.TrimSpace(meta.Description)

	return meta, nil
}

// extractFrontMatter returns the YAML block between the leading '---' line and
// the next '---' or '...' line. Unlike splitting the whole file on '---', a
// delimiter appearing inside a value does not truncate the block, because only
// an unindented delimiter on its own line closes it — the same rule YAML uses
// for document boundaries.
func extractFrontMatter(text string) (string, bool) {
	text = strings.TrimPrefix(text, "\ufeff")

	lines := strings.Split(text, "\n")
	if len(lines) == 0 || trimDelimiter(lines[0]) != "---" {
		return "", false
	}

	for i := 1; i < len(lines); i++ {
		switch trimDelimiter(lines[i]) {
		case "---", "...":
			return strings.Join(lines[1:i], "\n"), true
		}
	}

	return "", false
}

func trimDelimiter(line string) string {
	return strings.TrimRight(line, " \t\r")
}

// isInstallerTempDir reports whether name matches install staging/backup
// directories created under the skills root (MkdirTemp prefixes
// ".<base>.staging-" / ".<base>.backup-"). User-chosen names that merely
// contain those substrings (e.g. "foo.backup-prod") are not matched.
func isInstallerTempDir(name string) bool {
	_, kind, ok := parseInstallerTempName(name)
	return ok && (kind == installerTempStaging || kind == installerTempBackup)
}

type installerTempKind int

const (
	installerTempStaging installerTempKind = iota + 1
	installerTempBackup
)

// parseInstallerTempName recognizes ".<base>.staging-<suffix>" and
// ".<base>.backup-<suffix>" names produced by os.MkdirTemp.
func parseInstallerTempName(name string) (base string, kind installerTempKind, ok bool) {
	if !strings.HasPrefix(name, ".") {
		return "", 0, false
	}
	rest := name[1:]
	const staging = ".staging-"
	const backup = ".backup-"
	stagingIdx := strings.Index(rest, staging)
	backupIdx := strings.Index(rest, backup)

	var marker string
	var idx int
	switch {
	case stagingIdx >= 0 && (backupIdx < 0 || stagingIdx <= backupIdx):
		marker, idx, kind = staging, stagingIdx, installerTempStaging
	case backupIdx >= 0:
		marker, idx, kind = backup, backupIdx, installerTempBackup
	default:
		return "", 0, false
	}

	base = rest[:idx]
	suffix := rest[idx+len(marker):]
	if base == "" || suffix == "" {
		return "", 0, false
	}
	if strings.ContainsAny(base, `/\`) {
		return "", 0, false
	}
	return base, kind, true
}

// RecoverOrphanedInstallerDirs restores backups left behind when a force
// reinstall was interrupted after moving the old skill aside, and removes
// leftover staging directories.
func RecoverOrphanedInstallerDirs(skillsDir string) {
	recoverOrphanedInstallerDirs(skillsDir)
}

// recoverOrphanedInstallerDirs restores backups left behind when a force
// reinstall was interrupted after moving the old skill aside, and removes
// leftover staging directories.
func recoverOrphanedInstallerDirs(skillsDir string) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return
	}

	type backupCandidate struct {
		path    string
		modTime time.Time
	}
	backups := map[string]backupCandidate{}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		base, kind, ok := parseInstallerTempName(name)
		if !ok {
			continue
		}
		fullPath := filepath.Join(skillsDir, name)
		if kind == installerTempStaging {
			_ = os.RemoveAll(fullPath)
			continue
		}

		if _, err := os.Stat(filepath.Join(fullPath, "SKILL.md")); err != nil {
			continue
		}
		finalPath := filepath.Join(skillsDir, base)
		if _, err := os.Lstat(finalPath); err == nil {
			continue
		} else if err != nil && !os.IsNotExist(err) {
			continue
		}

		modTime := time.Time{}
		if info, err := entry.Info(); err == nil {
			modTime = info.ModTime()
		}
		if prev, exists := backups[base]; exists && !modTime.After(prev.modTime) {
			continue
		}
		backups[base] = backupCandidate{path: fullPath, modTime: modTime}
	}

	for base, candidate := range backups {
		finalPath := filepath.Join(skillsDir, base)
		if _, err := os.Lstat(finalPath); err == nil || (err != nil && !os.IsNotExist(err)) {
			continue
		}
		_ = os.Rename(candidate.path, finalPath)
	}
}

// ValidateSkillName rejects names that are empty, ".", "..", contain path
// separators, or do not Clean to a single base path segment. Embedded ".."
// inside one segment (e.g. "foo..bar") is allowed: it has no traversal
// semantics once separators and exact "."/".." are blocked. This blocks
// Join-cleaning aliases that would make filepath.Join(skillsDir, name) equal
// the skills root (SEC-08) or escape it (SEC-07).
//
// Also reject any name that Win32 trailing-space/period trimming would change
// (e.g. ". ", ".. ", "victim.", "victim "). Ordinary Win32 path handling
// strips those characters, so a lexical child can still resolve to the skills
// root or to a different installed skill directory.
func ValidateSkillName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid skill name: must not be empty")
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("invalid skill name %q: must not be an absolute path", name)
	}
	if strings.ContainsRune(name, '/') || strings.ContainsRune(name, '\\') {
		return fmt.Errorf("invalid skill name %q: must not contain path separators", name)
	}
	if isPathReferenceName(name) {
		return fmt.Errorf("invalid skill name %q: must not be a path reference", name)
	}
	cleaned := filepath.Clean(name)
	if cleaned != name {
		return fmt.Errorf("invalid skill name %q", name)
	}
	// Require a single base segment after Clean (not ".", "..", or nested).
	if cleaned == "." || cleaned == ".." || filepath.Base(cleaned) != cleaned {
		return fmt.Errorf("invalid skill name %q: must be a single path segment", name)
	}
	return nil
}

// isPathReferenceName reports whether name is "." / ".." or a Win32 alias that
// ordinary path handling would rewrite by stripping trailing spaces and
// periods from the final segment. Any change under that trim is unsafe: ". "
// / ".." collapse to path references, while "victim." / "victim " collapse to
// an existing "victim" skill directory.
func isPathReferenceName(name string) bool {
	if name == "." || name == ".." {
		return true
	}
	// Strip the same trailing " ." class Win32 removes from a final segment.
	// Reject whenever that changes the name — not only when the result is
	// empty / "." / "..".
	return strings.TrimRight(name, ". ") != name
}

// GetSkillDir returns the full path for a skill. Invalid names refuse to
// leave the skills root (callers should validate earlier for clear errors).
func GetSkillDir(name string) string {
	skillsDir := config.GetSkillsDir()
	if err := ValidateSkillName(name); err != nil {
		return skillsDir
	}
	return filepath.Join(skillsDir, name)
}
