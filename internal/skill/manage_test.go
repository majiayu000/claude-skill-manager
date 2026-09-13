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

func TestExistsMatchesDirectoryOnly(t *testing.T) {
	installSkill(t, "pdf-tools", "---\nname: pdf\ndescription: PDF tools\n---\n")

	if Exists("pdf") {
		t.Fatal("Exists must not match front-matter aliases")
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

func TestGetPrefersDirectoryAndAllowsUnambiguousAlias(t *testing.T) {
	installSkill(t, "pdf-tools", "---\nname: pdf\ndescription: PDF tools\n---\n")

	byDir, err := Get("pdf-tools")
	if err != nil {
		t.Fatal(err)
	}
	if byDir == nil || filepath.Base(byDir.Path) != "pdf-tools" {
		t.Fatalf("expected directory lookup, got %+v", byDir)
	}

	byAlias, err := Get("pdf")
	if err != nil {
		t.Fatal(err)
	}
	if byAlias == nil || filepath.Base(byAlias.Path) != "pdf-tools" {
		t.Fatalf("expected unambiguous alias lookup, got %+v", byAlias)
	}
}

func TestGetRejectsAmbiguousFrontMatterAlias(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillsDir := filepath.Join(home, ".claude", "skills")
	for _, dir := range []string{"pdf-tools", "pdf-extra"} {
		skillDir := filepath.Join(skillsDir, dir)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: pdf\ndescription: PDF tools\n---\n"
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := Get("pdf")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected ambiguous alias to resolve to nil, got %+v", got)
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

func TestRemoveByAliasDoesNotDeleteDirectory(t *testing.T) {
	skillDir := installSkill(t, "pdf-tools", "---\nname: pdf\ndescription: PDF tools\n---\n")

	if err := Remove("pdf"); !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist when removing by alias, got %v", err)
	}
	if _, err := os.Stat(skillDir); err != nil {
		t.Fatalf("expected pdf-tools to remain, stat err = %v", err)
	}
	if !Exists("pdf-tools") {
		t.Fatal("expected pdf-tools to still be installed")
	}
}

func TestForceReinstallTargetDoesNotTouchAliasDirectory(t *testing.T) {
	aliasDir := installSkill(t, "pdf-tools", "---\nname: pdf\ndescription: PDF tools\n---\n")

	// Simulates: sk install <src> --name pdf --force when only pdf-tools exists.
	if Exists("pdf") {
		t.Fatal("destination pdf must not be considered installed via alias")
	}
	if err := Remove("pdf"); !os.IsNotExist(err) {
		t.Fatalf("force remove of pdf must be a no-op miss, got %v", err)
	}
	if _, err := os.Stat(aliasDir); err != nil {
		t.Fatalf("pdf-tools must survive force reinstall targeting pdf: %v", err)
	}

	// Occupying the real destination still works.
	destDir := filepath.Join(filepath.Dir(aliasDir), "pdf")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "SKILL.md"), []byte("---\nname: pdf\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !Exists("pdf") {
		t.Fatal("expected directory pdf to be installed")
	}
	if err := Remove("pdf"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		t.Fatalf("expected pdf destination removed, stat err = %v", err)
	}
	if _, err := os.Stat(aliasDir); err != nil {
		t.Fatalf("pdf-tools must still exist after removing pdf: %v", err)
	}
}

func TestListSkipsStagingDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillsDir := filepath.Join(home, ".claude", "skills")
	for _, tempName := range []string{".docx.staging-abc123", ".docx.backup-xyz"} {
		tempDir := filepath.Join(skillsDir, tempName)
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, "SKILL.md"), []byte("---\nname: docx\n---\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	real := filepath.Join(skillsDir, "docx")
	if err := os.MkdirAll(real, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "SKILL.md"), []byte("---\nname: docx\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	skills, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected only the real skill, got %#v", skills)
	}
	if filepath.Base(skills[0].Path) != "docx" {
		t.Fatalf("unexpected skill path: %s", skills[0].Path)
	}
	if Exists(".docx.staging-abc123") || Exists(".docx.backup-xyz") {
		t.Fatal("installer temp directories must not match Exists")
	}
}

func TestListDiscoversLeadingDotSkillNames(t *testing.T) {
	installSkill(t, ".foo", "---\nname: foo\ndescription: dotted install\n---\n")

	skills, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected dotted skill to be listed, got %#v", skills)
	}
	if filepath.Base(skills[0].Path) != ".foo" {
		t.Fatalf("unexpected skill path: %s", skills[0].Path)
	}
	if !Exists(".foo") {
		t.Fatal("expected Exists to find --name .foo install by directory")
	}
	if Exists("foo") {
		t.Fatal("Exists must not match the front-matter alias of --name .foo")
	}
	got, err := Get("foo")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || filepath.Base(got.Path) != ".foo" {
		t.Fatalf("expected Get alias lookup for .foo, got %+v", got)
	}
}

func TestListDoesNotHideCustomNamesContainingBackupSubstring(t *testing.T) {
	installSkill(t, "foo.backup-prod", "---\nname: foo.backup-prod\n---\n")

	skills, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected custom name with backup substring to be listed, got %#v", skills)
	}
	if !Exists("foo.backup-prod") {
		t.Fatal("expected Exists to find --name foo.backup-prod")
	}
	if isInstallerTempDir("foo.backup-prod") {
		t.Fatal("user-chosen foo.backup-prod must not match installer temp pattern")
	}
	if !isInstallerTempDir(".docx.backup-abc123") {
		t.Fatal("expected installer backup dir to match temp pattern")
	}
	if !isInstallerTempDir(".docx.staging-abc123") {
		t.Fatal("expected installer staging dir to match temp pattern")
	}
}

func TestListRecoversOrphanedBackupWhenFinalMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillsDir := filepath.Join(home, ".claude", "skills")
	backupDir := filepath.Join(skillsDir, ".docx.backup-orphan1")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatal(err)
	}
	original := "---\nname: docx\n---\nrecovered\n"
	if err := os.WriteFile(filepath.Join(backupDir, "SKILL.md"), []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	skills, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected recovered skill, got %#v", skills)
	}
	finalDir := filepath.Join(skillsDir, "docx")
	got, err := os.ReadFile(filepath.Join(finalDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("expected backup restored to docx: %v", err)
	}
	if string(got) != original {
		t.Fatalf("restored contents mismatch: %q", got)
	}
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Fatal("expected orphaned backup directory to be moved away")
	}
}

func TestRemoveMissingSkill(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := Remove("absent"); !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestRemoveRejectsUnsafeNames(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	for _, name := range []string{"", ".", "..", "../etc", "a/b", `a\b`} {
		if err := Remove(name); !os.IsNotExist(err) {
			t.Fatalf("Remove(%q): expected os.ErrNotExist, got %v", name, err)
		}
		if Exists(name) {
			t.Fatalf("Exists(%q) should be false", name)
		}
	}
}
