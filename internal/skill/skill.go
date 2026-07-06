package skill

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/majiayu000/claude-skill-manager/internal/config"
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
	text := string(content)

	// Simple front matter parsing (between ---)
	if strings.HasPrefix(text, "---") {
		parts := strings.SplitN(text, "---", 3)
		if len(parts) >= 3 {
			frontMatter := parts[1]
			lines := strings.Split(frontMatter, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "name:") {
					meta.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
					meta.Name = strings.Trim(meta.Name, "\"'")
				} else if strings.HasPrefix(line, "description:") {
					meta.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
					meta.Description = strings.Trim(meta.Description, "\"'")
				}
			}
		}
	}

	return meta, nil
}

// GetSkillDir returns the full path for a skill
func GetSkillDir(name string) string {
	return filepath.Join(config.GetSkillsDir(), name)
}
