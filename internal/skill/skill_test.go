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
