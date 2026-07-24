package skill

import (
	"os"
	"path/filepath"
	"testing"
)

// installSkill creates a skill directory under a temporary HOME and returns
// the skills root.
func installSkill(t *testing.T, dirName, skillMd string) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	skillDir := filepath.Join(home, ".claude", "skills", dirName)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMd), 0644); err != nil {
		t.Fatal(err)
	}
	return skillDir
}

func TestExistsMatchesFrontMatterName(t *testing.T) {
	installSkill(t, "pdf-tools", "---\nname: pdf\ndescription: PDF tools\n---\n")

	if !Exists("pdf") {
		t.Fatal("expected the front-matter name to match")
	}
	if !Exists("pdf-tools") {
		t.Fatal("expected the directory name to match")
	}
	if Exists("nope") {
		t.Fatal("unexpected match for an uninstalled skill")
	}
}

func TestExistsFalseWhenSkillsDirMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if Exists("anything") {
		t.Fatal("expected no skills when the skills directory does not exist")
	}
}

func TestListSkipsDirectoriesWithoutSkillMd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillsDir := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(filepath.Join(skillsDir, "not-a-skill"), 0755); err != nil {
		t.Fatal(err)
	}

	skills, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected no skills, got %d", len(skills))
	}
}

func TestRemoveDeletesSkillDir(t *testing.T) {
	skillDir := installSkill(t, "docx", "---\nname: docx\n---\n")

	if err := Remove("docx"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatalf("expected the skill directory to be gone, stat err = %v", err)
	}
	if Exists("docx") {
		t.Fatal("expected the skill to be uninstalled")
	}
}

func TestRemoveMissingSkill(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := Remove("absent"); !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}
