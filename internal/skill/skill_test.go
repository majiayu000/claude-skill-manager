package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkillMd(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseSkillMdFoldedDescription(t *testing.T) {
	meta, err := parseSkillMd(writeSkillMd(t, `---
name: pdf
description: >
  Extract text and tables from PDF files,
  fill forms, and merge documents.
---

# PDF
`))
	if err != nil {
		t.Fatal(err)
	}

	want := "Extract text and tables from PDF files, fill forms, and merge documents."
	if meta.Description != want {
		t.Fatalf("got %q, want %q", meta.Description, want)
	}
	if meta.Name != "pdf" {
		t.Fatalf("unexpected name: %q", meta.Name)
	}
}

func TestParseSkillMdLiteralBlockDescription(t *testing.T) {
	meta, err := parseSkillMd(writeSkillMd(t, `---
name: docx
description: |
  Line one.
  Line two.
---
`))
	if err != nil {
		t.Fatal(err)
	}

	if meta.Description != "Line one.\nLine two." {
		t.Fatalf("unexpected description: %q", meta.Description)
	}
}

func TestParseSkillMdKeepsDelimiterInsideValue(t *testing.T) {
	meta, err := parseSkillMd(writeSkillMd(t, `---
name: dashes
description: "use --- to separate sections"
---

body
`))
	if err != nil {
		t.Fatal(err)
	}

	if meta.Description != "use --- to separate sections" {
		t.Fatalf("unexpected description: %q", meta.Description)
	}
}

func TestParseSkillMdQuotedAndPlainScalars(t *testing.T) {
	meta, err := parseSkillMd(writeSkillMd(t, `---
name: "quoted-name"
description: 'single quoted'
---
`))
	if err != nil {
		t.Fatal(err)
	}

	if meta.Name != "quoted-name" {
		t.Fatalf("unexpected name: %q", meta.Name)
	}
	if meta.Description != "single quoted" {
		t.Fatalf("unexpected description: %q", meta.Description)
	}
}

func TestParseSkillMdWithoutFrontMatter(t *testing.T) {
	meta, err := parseSkillMd(writeSkillMd(t, "# Just a heading\n\nSome prose.\n"))
	if err != nil {
		t.Fatal(err)
	}

	if meta.Name != "" || meta.Description != "" {
		t.Fatalf("expected empty metadata, got %+v", meta)
	}
}

func TestParseSkillMdUnterminatedFrontMatter(t *testing.T) {
	meta, err := parseSkillMd(writeSkillMd(t, "---\nname: broken\n"))
	if err != nil {
		t.Fatal(err)
	}

	if meta.Name != "" {
		t.Fatalf("expected unterminated front matter to be ignored, got %q", meta.Name)
	}
}

func TestParseSkillMdInvalidYAMLErrors(t *testing.T) {
	if _, err := parseSkillMd(writeSkillMd(t, "---\nname: [unclosed\n---\n")); err == nil {
		t.Fatal("expected an error for malformed front matter")
	}
}

func TestExtractFrontMatterStripsBOM(t *testing.T) {
	block, ok := extractFrontMatter("\ufeff---\nname: bom\n---\n")
	if !ok {
		t.Fatal("expected front matter to be found after a BOM")
	}
	if block != "name: bom" {
		t.Fatalf("unexpected block: %q", block)
	}
}

func TestValidateSkillNameRejectsPathEscape(t *testing.T) {
	valid := []string{"docx", "my-skill", "skill_1", "frontend.testing", "foo..bar"}
	for _, name := range valid {
		if err := ValidateSkillName(name); err != nil {
			t.Fatalf("ValidateSkillName(%q): unexpected error: %v", name, err)
		}
	}

	invalid := []string{
		"",
		"/tmp/pwned-skill",
		"../outside",
		"..",
		".",
		". ",  // Win32 strips trailing space → "."
		".. ", // Win32 strips trailing space → ".."
		"...", // Win32 strips trailing periods → empty / current-dir alias
		". .",
		".. .",
		" .",
		"victim.",  // Win32 strips trailing period → existing "victim" skill
		"victim ",  // Win32 strips trailing space → existing "victim" skill
		"victim. ", // Win32 strips trailing period+space → "victim"
		"nested/name",
		`nested\name`,
		"foo/../bar",
		"owner/repo/..",
	}
	for _, name := range invalid {
		if err := ValidateSkillName(name); err == nil {
			t.Fatalf("ValidateSkillName(%q): expected error", name)
		}
	}
}

func TestGetSkillDirRefusesEscape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	skillsDir := filepath.Join(home, ".claude", "skills")

	got, err := GetSkillDir("docx")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(skillsDir, "docx")
	if got != want {
		t.Fatalf("GetSkillDir(docx) = %q, want %q", got, want)
	}

	if got, err := GetSkillDir("/tmp/pwned-skill"); err != nil || got != skillsDir {
		t.Fatalf("GetSkillDir(absolute) = %q, want skills root %q", got, skillsDir)
	}
	if got, err := GetSkillDir("../outside"); err != nil || got != skillsDir {
		t.Fatalf("GetSkillDir(..) = %q, want skills root %q", got, skillsDir)
	}
	if got, err := GetSkillDir("."); err != nil || got != skillsDir {
		t.Fatalf("GetSkillDir(.) = %q, want skills root %q", got, skillsDir)
	}
}

func TestSkillOperationsFailWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Chdir(t.TempDir())
	path := filepath.Join(".claude", "skills", "docx", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: docx\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if got, err := GetSkillDir("docx"); err == nil || got != "" {
		t.Fatalf("expected no path and a home directory error, got %q, %v", got, err)
	}
	if _, err := List(); err == nil {
		t.Fatal("expected List to return a home directory error")
	}
	if err := Remove("docx"); err == nil || os.IsNotExist(err) {
		t.Fatalf("expected Remove to return a home directory error, got %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("working directory skill was modified: %v", err)
	}
}
