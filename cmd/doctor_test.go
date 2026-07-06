package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/majiayu000/claude-skill-manager/internal/registry"
)

func TestInspectCacheFileReportsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")

	got := inspectCacheFile(path, time.Hour, nil)

	if got.State != "missing" {
		t.Fatalf("state = %q, want missing", got.State)
	}
	if got.Path != path {
		t.Fatalf("path = %q, want %q", got.Path, path)
	}
}

func TestInspectCacheFileReportsFreshValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := os.WriteFile(path, []byte(`{"skills":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	got := inspectCacheFile(path, time.Hour, nil)

	if got.State != "fresh" {
		t.Fatalf("state = %q, want fresh; detail=%s", got.State, got.Detail)
	}
}

func TestInspectCacheFileReportsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := os.WriteFile(path, []byte(`{bad json`), 0644); err != nil {
		t.Fatal(err)
	}

	got := inspectCacheFile(path, time.Hour, nil)

	if got.State != "malformed" {
		t.Fatalf("state = %q, want malformed", got.State)
	}
}

func TestInspectCacheFileReportsExpiredJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := os.WriteFile(path, []byte(`{"skills":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	got := inspectCacheFile(path, time.Hour, nil)

	if got.State != "expired" {
		t.Fatalf("state = %q, want expired; detail=%s", got.State, got.Detail)
	}
}

func TestInspectCacheFileReportsInvalidRegistryPointerCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte(`{"deprecated_full_payload":true,"manifest":"registry-manifest.json"}`), 0644); err != nil {
		t.Fatal(err)
	}

	got := inspectCacheFile(path, time.Hour, validateRegistryCachePayload)

	if got.State != "invalid" {
		t.Fatalf("state = %q, want invalid; detail=%s", got.State, got.Detail)
	}
}

func TestInspectCacheFileReportsInvalidSearchPointerCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search-index.json")
	if err := os.WriteFile(path, []byte(`{"t":1}`), 0644); err != nil {
		t.Fatal(err)
	}

	got := inspectCacheFile(path, time.Hour, validateSearchIndexCachePayload)

	if got.State != "invalid" {
		t.Fatalf("state = %q, want invalid; detail=%s", got.State, got.Detail)
	}
}

func TestRegistrySourceMessage(t *testing.T) {
	tests := []struct {
		name   string
		source registry.RegistrySource
		want   string
	}{
		{
			name:   "remote",
			source: registry.RegistrySourceRemote,
			want:   "Using remote registry data...",
		},
		{
			name:   "cache",
			source: registry.RegistrySourceCache,
			want:   "Using cached registry data...",
		},
		{
			name:   "unknown",
			source: registry.RegistrySource(""),
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := registrySourceMessage(tt.source); got != tt.want {
				t.Fatalf("message = %q, want %q", got, tt.want)
			}
		})
	}
}
