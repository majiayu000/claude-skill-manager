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

// GetSkillDir returns the full path for a skill
func GetSkillDir(name string) string {
	return filepath.Join(config.GetSkillsDir(), name)
}
