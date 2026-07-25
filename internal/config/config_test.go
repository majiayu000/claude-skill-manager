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
