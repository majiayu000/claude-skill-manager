package github

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestResolveSkillsTargetDirRejectsEscape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillsDir := filepath.Join(home, ".claude", "skills")
	got, err := resolveSkillsTargetDir("safe-skill")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(skillsDir, "safe-skill")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// Escape outside skills dir (SEC-07) and skills-root aliases (SEC-08).
	for _, name := range []string{"/tmp/pwned-skill", "../outside", "..", ".", "nested/name"} {
		if _, err := resolveSkillsTargetDir(name); err == nil {
			t.Fatalf("resolveSkillsTargetDir(%q): expected error", name)
		}
	}
}

func TestDownloadAndExtractRejectsEscapingNameBeforeDownload(t *testing.T) {
	info := &RepoInfo{Owner: "o", Repo: "r", Branch: "main"}
	for _, name := range []string{"../outside", "."} {
		err := DownloadAndExtract(info, name)
		if err == nil {
			t.Fatalf("expected target name %q to fail before download", name)
		}
	}
}

func TestIsStrictSubdirRejectsSkillsRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	// SEC-08: isWithinDir still accepts rel == ".", so escape checks alone are insufficient.
	if !isWithinDir(root, root) {
		t.Fatal("expected isWithinDir(root, root) == true (rel == \".\")")
	}
	if isStrictSubdir(root, root) {
		t.Fatal("skills root must not count as a strict subdirectory of itself")
	}
	child := filepath.Join(root, "docx")
	if !isStrictSubdir(root, child) {
		t.Fatalf("expected %q to be a strict subdirectory of %q", child, root)
	}
}

func TestRemoveSkillTargetDirRefusesSkillsRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillsDir := filepath.Join(home, ".claude", "skills")
	keepDir := filepath.Join(skillsDir, "keep-me")
	if err := os.MkdirAll(keepDir, 0755); err != nil {
		t.Fatal(err)
	}
	keepFile := filepath.Join(keepDir, "SKILL.md")
	if err := os.WriteFile(keepFile, []byte("---\nname: keep-me\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Simulate the pre-fix cleanup path where targetDir == skillsDir (name ".").
	if err := removeSkillTargetDir(skillsDir); err == nil {
		t.Fatal("expected removeSkillTargetDir(skills root) to refuse")
	}

	if _, err := os.Stat(keepFile); err != nil {
		t.Fatalf("populated skills root must survive refused RemoveAll: %v", err)
	}
}

func TestRemoveSkillTargetDirRemovesOnlySkillSubdir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillsDir := filepath.Join(home, ".claude", "skills")
	keepDir := filepath.Join(skillsDir, "keep-me")
	badDir := filepath.Join(skillsDir, "bad-extract")
	if err := os.MkdirAll(keepDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(badDir, 0755); err != nil {
		t.Fatal(err)
	}
	keepFile := filepath.Join(keepDir, "SKILL.md")
	if err := os.WriteFile(keepFile, []byte("---\nname: keep-me\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "partial.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := removeSkillTargetDir(badDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(badDir); !os.IsNotExist(err) {
		t.Fatalf("expected bad-extract removed, stat err=%v", err)
	}
	if _, err := os.Stat(keepFile); err != nil {
		t.Fatalf("sibling skill must remain: %v", err)
	}
}
