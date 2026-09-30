package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if body == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(home, ".skrc"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestGetRegistryBaseURLFallsBackToDefault(t *testing.T) {
	for name, body := range map[string]string{
		"no config file":  "",
		"empty registry":  `{"registry": ""}`,
		"legacy 'github'": `{"registry": "github"}`,
	} {
		t.Run(name, func(t *testing.T) {
			writeConfig(t, body)
			if got := GetRegistryBaseURL(); got != DefaultRegistryURL {
				t.Fatalf("got %q, want %q", got, DefaultRegistryURL)
			}
		})
	}
}

func TestGetRegistryBaseURLHonoursOverride(t *testing.T) {
	writeConfig(t, `{"registry": "https://example.test/registry"}`)
	if got := GetRegistryBaseURL(); got != "https://example.test/registry" {
		t.Fatalf("got %q, want the configured override", got)
	}
}

func TestLoadReadsFileOnlyOnce(t *testing.T) {
	writeConfig(t, `{"registry": "https://example.test/registry"}`)

	first := Load()
	if err := os.Remove(ConfigPath()); err != nil {
		t.Fatal(err)
	}

	// The file is gone; an uncached Load would fall back to the defaults.
	if second := Load(); second != first {
		t.Fatalf("expected the cached config, got a fresh read: %+v", second)
	}
	if got := GetRegistryBaseURL(); got != "https://example.test/registry" {
		t.Fatalf("accessor re-read disk: got %q", got)
	}
}

func TestGetSkillsDirResolvesHomePaths(t *testing.T) {
	for _, tt := range []struct {
		name         string
		body         string
		want         string
		relativeHome bool
	}{
		{"default", "", ".claude/skills", true},
		{"empty", `{"skills_dir": ""}`, ".claude/skills", true},
		{"README sample", `{"skills_dir": "~/.claude/skills"}`, ".claude/skills", true},
		{"home itself", `{"skills_dir": "~"}`, ".", true},
		{"home slash", `{"skills_dir": "~/"}`, ".", true},
		{"custom home path", `{"skills_dir": "~/custom/skills"}`, "custom/skills", true},
		{"absolute override", `{"skills_dir": "/tmp/custom-skills"}`, "/tmp/custom-skills", false},
		{"relative override", `{"skills_dir": "custom/skills"}`, "custom/skills", false},
		{"literal tilde prefix", `{"skills_dir": "~other/skills"}`, "~other/skills", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			writeConfig(t, tt.body)
			want := tt.want
			if tt.relativeHome {
				want = filepath.Join(os.Getenv("HOME"), want)
			}
			got, err := GetSkillsDir()
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("got skills directory %q, want %q", got, want)
			}
		})
	}
}

func TestEnsureSkillsDirCreatesHomeDirectory(t *testing.T) {
	for _, body := range []string{`{"skills_dir": "~/.claude/skills"}`, `{"skills_dir": ""}`} {
		t.Run(body, func(t *testing.T) {
			writeConfig(t, body)
			cwd := t.TempDir()
			t.Chdir(cwd)
			if err := EnsureSkillsDir(); err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(os.Getenv("HOME"), ".claude", "skills")
			if info, err := os.Stat(want); err != nil || !info.IsDir() {
				t.Fatalf("expected skills directory at %q, info=%v, err=%v", want, info, err)
			}
			if entries, err := os.ReadDir(cwd); err != nil || len(entries) != 0 {
				t.Fatalf("working directory was modified: entries=%v, err=%v", entries, err)
			}
		})
	}
}

func TestEnsureSkillsDirFailsWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	cwd := t.TempDir()
	t.Chdir(cwd)
	if err := os.WriteFile(".skrc", []byte(`{"skills_dir": "custom/skills"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSkillsDir(); err == nil {
		t.Fatal("expected a home directory error")
	}
	if entries, err := os.ReadDir(cwd); err != nil || len(entries) != 1 || entries[0].Name() != ".skrc" {
		t.Fatalf("working directory was modified: entries=%v, err=%v", entries, err)
	}
}
