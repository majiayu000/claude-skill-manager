package github

import (
	"archive/zip"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseGitHubURLTrimsDirectorySkillFile(t *testing.T) {
	info, err := ParseGitHubURL("langgenius/dify/.agents/skills/frontend-testing/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != ".agents/skills/frontend-testing" {
		t.Fatalf("unexpected path: %s", info.Path)
	}
	if info.FilePath != "" {
		t.Fatalf("expected directory install, got file path: %s", info.FilePath)
	}
	if name := GetSkillName(info); name != "frontend-testing" {
		t.Fatalf("unexpected skill name: %s", name)
	}
}

func TestParseGitHubURLKeepsSingleSkillFile(t *testing.T) {
	info, err := ParseGitHubURL("redmage123/salesforce/.agents/project_analysis_agent_SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.FilePath != ".agents/project_analysis_agent_SKILL.md" {
		t.Fatalf("unexpected file path: %s", info.FilePath)
	}
	if info.Path != "" {
		t.Fatalf("expected file install, got directory path: %s", info.Path)
	}
	if name := GetSkillName(info); name != "project_analysis_agent" {
		t.Fatalf("unexpected skill name: %s", name)
	}
}

func TestParseGitHubURLDoesNotTreatCommandMarkdownAsSkillFile(t *testing.T) {
	info, err := ParseGitHubURL("udecode/plate/.claude/commands/sync-testing-skill.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.FilePath != "" {
		t.Fatalf("command markdown should not be treated as a skill file: %s", info.FilePath)
	}
	if info.Path != ".claude/commands/sync-testing-skill.md" {
		t.Fatalf("unexpected path: %s", info.Path)
	}
}

func TestDownloadClientHasTimeout(t *testing.T) {
	if downloadClient.Timeout <= 0 {
		t.Fatal("download client must have a bounded timeout")
	}
}

func TestDownloadToTempFileTimesOut(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer func() {
		close(block)
		srv.Close()
	}()

	restore := downloadClient
	downloadClient = &http.Client{Timeout: 50 * time.Millisecond}
	defer func() { downloadClient = restore }()

	if _, err := downloadToTempFile(srv.URL); err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
}

func TestDownloadToTempFileReportsStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := downloadToTempFile(srv.URL)
	var statusErr *httpStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected *httpStatusError, got %v", err)
	}
}

func TestDownloadToTempFileWritesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("zip-bytes"))
	}))
	defer srv.Close()

	path, err := downloadToTempFile(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(path) }()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "zip-bytes" {
		t.Fatalf("unexpected body: %q", data)
	}
}

func TestForceReinstallKeepsExistingSkillOnExtractFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillDir := filepath.Join(home, ".claude", "skills", "docx")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	original := "---\nname: docx\ndescription: working install\n---\nkeep me\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(skillDir, "precious.txt")
	if err := os.WriteFile(markerPath, []byte("do-not-delete"), 0644); err != nil {
		t.Fatal(err)
	}

	// Zip has the wrong path / no SKILL.md — extract must fail before swap.
	zipPath := writeTestZip(t, map[string]string{
		"repo-main/other/README.md": "not a skill",
	})

	info := &RepoInfo{
		Owner:  "owner",
		Repo:   "repo",
		Branch: "main",
		Path:   "docx",
	}
	if _, err := installZipAtomically(zipPath, info, "docx", ExtractOptions{}); err == nil {
		t.Fatal("expected extract failure for --force reinstall")
	}

	got, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("original skill directory should still exist: %v", err)
	}
	if string(got) != original {
		t.Fatalf("original SKILL.md changed:\n%s", got)
	}
	marker, err := os.ReadFile(markerPath)
	if err != nil || string(marker) != "do-not-delete" {
		t.Fatalf("original skill contents were disturbed: err=%v marker=%q", err, marker)
	}

	entries, err := os.ReadDir(filepath.Join(home, ".claude", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Name()) > 0 && e.Name()[0] == '.' {
			t.Fatalf("staging directory leaked: %s", e.Name())
		}
	}
}

func TestAtomicInstallReplacesExistingSkillOnSuccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillDir := filepath.Join(home, ".claude", "skills", "docx")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: docx\n---\nold\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "old-only.txt"), []byte("gone"), 0644); err != nil {
		t.Fatal(err)
	}

	zipPath := writeTestZip(t, map[string]string{
		"repo-main/docx/SKILL.md":  "---\nname: docx\ndescription: refreshed\n---\nnew\n",
		"repo-main/docx/helper.md": "helper",
	})

	info := &RepoInfo{
		Owner:  "owner",
		Repo:   "repo",
		Branch: "main",
		Path:   "docx",
	}
	if _, err := installZipAtomically(zipPath, info, "docx", ExtractOptions{AllowReplace: true}); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "---\nname: docx\ndescription: refreshed\n---\nnew\n" {
		t.Fatalf("unexpected SKILL.md after swap:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "helper.md")); err != nil {
		t.Fatalf("expected helper.md after swap: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "old-only.txt")); !os.IsNotExist(err) {
		t.Fatal("expected old-only.txt to be replaced away")
	}
}

func TestRejectParentDirectoryTargetNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	skillsDir := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	zipPath := writeTestZip(t, map[string]string{
		"repo-main/docx/SKILL.md": "---\nname: docx\n---\n",
	})
	info := &RepoInfo{Owner: "owner", Repo: "repo", Branch: "main", Path: "docx"}

	for _, name := range []string{".", "..", "a/b", "../x", "x/.."} {
		if _, err := installZipAtomically(zipPath, info, name, ExtractOptions{}); err == nil {
			t.Fatalf("expected invalid skill name %q to be rejected", name)
		}
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("skills root should be untouched after rejected names, got %v", entries)
	}
}

func TestForceReplaceUsesAliasedInstallPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	aliasDir := filepath.Join(home, ".claude", "skills", "alias")
	if err := os.MkdirAll(aliasDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aliasDir, "SKILL.md"), []byte("---\nname: docx\n---\nold\n"), 0644); err != nil {
		t.Fatal(err)
	}

	zipPath := writeTestZip(t, map[string]string{
		"repo-main/docx/SKILL.md": "---\nname: docx\ndescription: refreshed\n---\nnew\n",
	})
	info := &RepoInfo{Owner: "owner", Repo: "repo", Branch: "main", Path: "docx"}

	finalDir, err := installZipAtomically(zipPath, info, "docx", ExtractOptions{
		FinalDir:     aliasDir,
		AllowReplace: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if finalDir != aliasDir {
		t.Fatalf("expected install into alias path, got %s", finalDir)
	}

	got, err := os.ReadFile(filepath.Join(aliasDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "refreshed") {
		t.Fatalf("alias install was not replaced: %s", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "docx")); !os.IsNotExist(err) {
		t.Fatal("expected no duplicate skills/docx directory")
	}
}

func TestForceInstallByDirectoryNameDoesNotReplaceAlias(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	aliasDir := filepath.Join(home, ".claude", "skills", "pdf-tools")
	if err := os.MkdirAll(aliasDir, 0755); err != nil {
		t.Fatal(err)
	}
	original := "---\nname: pdf\ndescription: PDF tools\n---\nold\n"
	if err := os.WriteFile(filepath.Join(aliasDir, "SKILL.md"), []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	zipPath := writeTestZip(t, map[string]string{
		"repo-main/pdf/SKILL.md": "---\nname: pdf\ndescription: other pdf skill\n---\nnew\n",
	})
	info := &RepoInfo{Owner: "owner", Repo: "repo", Branch: "main", Path: "pdf"}

	// sk install <src> --name pdf --force: occupy skills/pdf only.
	finalDir, err := installZipAtomically(zipPath, info, "pdf", ExtractOptions{AllowReplace: true})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".claude", "skills", "pdf")
	if finalDir != want {
		t.Fatalf("expected install into %s, got %s", want, finalDir)
	}

	got, err := os.ReadFile(filepath.Join(aliasDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("pdf-tools must survive force install targeting pdf: %v", err)
	}
	if string(got) != original {
		t.Fatalf("pdf-tools was modified:\n%s", got)
	}
	installed, err := os.ReadFile(filepath.Join(want, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), "other pdf skill") {
		t.Fatalf("expected new skill at skills/pdf, got %s", installed)
	}
}

func TestRefuseUnforcedNonSkillCollision(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillsDir := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	collision := filepath.Join(skillsDir, "docx")
	if err := os.WriteFile(collision, []byte("not a skill dir"), 0644); err != nil {
		t.Fatal(err)
	}

	zipPath := writeTestZip(t, map[string]string{
		"repo-main/docx/SKILL.md": "---\nname: docx\n---\n",
	})
	info := &RepoInfo{Owner: "owner", Repo: "repo", Branch: "main", Path: "docx"}

	if _, err := installZipAtomically(zipPath, info, "docx", ExtractOptions{}); err == nil {
		t.Fatal("expected collision with non-skill path to be refused")
	}

	got, err := os.ReadFile(collision)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "not a skill dir" {
		t.Fatalf("collision path was modified: %q", got)
	}
}

func TestReplaceRestoresBackupWhenRenameFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillDir := filepath.Join(home, ".claude", "skills", "docx")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	original := "---\nname: docx\n---\nkeep\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	// stagingDir points at a missing path so Rename(staging -> final) fails
	// after the existing install has already been moved aside.
	stagingMissing := filepath.Join(home, ".claude", "skills", ".docx.staging-missing")
	err := replaceSkillDir(skillDir, stagingMissing, true)
	if err == nil {
		t.Fatal("expected rename failure")
	}

	got, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("original skill should be restored after failed rename: %v", err)
	}
	if string(got) != original {
		t.Fatalf("restored contents mismatch: %q", got)
	}
}

func TestValidateSkillName(t *testing.T) {
	if err := validateSkillName("docx"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", ".", "..", "a/b", `a\b`, "victim.", "victim ", ". "} {
		if err := validateSkillName(name); err == nil {
			t.Fatalf("expected %q to be invalid", name)
		}
	}
}

func writeTestZip(t *testing.T, files map[string]string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "skill.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
