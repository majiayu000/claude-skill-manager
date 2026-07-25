package registry

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchRegistryFollowsManifestShards(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/registry.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"deprecated_full_payload":true,"manifest":"registry-manifest.json","version":"pointer-version","total_count":2}`)
	})
	mux.HandleFunc("/registry-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"schema_version":1,"generated_at":"2026-06-17T05:33:59Z","total_count":2,"shards":[{"gzip_path":"registry-shards/00.json.gz","count":1},{"path":"registry-shards/01.json","count":1}]}`)
	})
	mux.HandleFunc("/registry-shards/00.json.gz", func(w http.ResponseWriter, r *http.Request) {
		writeGzip(t, w, `{"schema_version":1,"count":1,"skills":[{"name":"frontend-testing","description":"testing skill","repo":"owner/repo","path":".agents/skills/frontend-testing/SKILL.md","branch":"main","category":"testing"}]}`)
	})
	mux.HandleFunc("/registry-shards/01.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"schema_version":1,"count":1,"skills":[{"name":"docx","description":"docs","repo":"anthropics/skills","path":"skills/docx/SKILL.md","branch":"main","category":"documents","install":"anthropics/skills/skills/docx"}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	registry, err := fetchRegistryFromBaseURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if registry.Version != "1" {
		t.Fatalf("unexpected version: %s", registry.Version)
	}
	if registry.UpdatedAt != "2026-06-17T05:33:59Z" {
		t.Fatalf("unexpected updated at: %s", registry.UpdatedAt)
	}
	if registry.TotalCount != 2 || len(registry.Skills) != 2 {
		t.Fatalf("expected two skills, got total=%d len=%d", registry.TotalCount, len(registry.Skills))
	}
	if got := registry.Skills[0].Install; got != "owner/repo/.agents/skills/frontend-testing/SKILL.md" {
		t.Fatalf("unexpected synthesized install: %s", got)
	}
	if got := registry.Skills[1].Install; got != "anthropics/skills/skills/docx" {
		t.Fatalf("explicit install should win, got: %s", got)
	}
}

func TestFetchRegistryFallsBackToPlainShardWhenGzipFails(t *testing.T) {
	var gzipRequested bool
	var plainRequested bool

	mux := http.NewServeMux()
	mux.HandleFunc("/registry.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"deprecated_full_payload":true,"manifest":"registry-manifest.json","total_count":1}`)
	})
	mux.HandleFunc("/registry-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"total_count":1,"shards":[{"gzip_path":"registry-shards/00.json.gz","path":"registry-shards/00.json","count":1}]}`)
	})
	mux.HandleFunc("/registry-shards/00.json.gz", func(w http.ResponseWriter, r *http.Request) {
		gzipRequested = true
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write([]byte("not gzip"))
	})
	mux.HandleFunc("/registry-shards/00.json", func(w http.ResponseWriter, r *http.Request) {
		plainRequested = true
		writeJSON(t, w, `{"count":1,"skills":[{"name":"plain-fallback","description":"plain shard","repo":"owner/repo","path":".claude/skills/plain-fallback/SKILL.md","category":"testing"}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	registry, err := fetchRegistryFromBaseURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !gzipRequested {
		t.Fatal("expected gzip shard to be tried first")
	}
	if !plainRequested {
		t.Fatal("expected plain shard fallback after gzip failure")
	}
	if len(registry.Skills) != 1 {
		t.Fatalf("expected one skill, got %d", len(registry.Skills))
	}
	if got := registry.Skills[0].Name; got != "plain-fallback" {
		t.Fatalf("unexpected fallback skill: %s", got)
	}
}

func TestFetchRegistryPointerWithoutManifestFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/registry.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"deprecated_full_payload":true,"total_count":10}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	_, err := fetchRegistryFromBaseURL(server.URL)
	if err == nil || !strings.Contains(err.Error(), "missing manifest") {
		t.Fatalf("expected missing manifest error, got %v", err)
	}
}

func TestFetchRegistrySynthesizesNonMainInstallAsTreeURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/registry.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"deprecated_full_payload":true,"manifest":"registry-manifest.json","total_count":1}`)
	})
	mux.HandleFunc("/registry-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"total_count":1,"shards":[{"path":"registry-shards/00.json","count":1}]}`)
	})
	mux.HandleFunc("/registry-shards/00.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"count":1,"skills":[{"name":"legacy-skill","description":"legacy branch skill","repo":"owner/repo","path":".claude/skills/legacy/SKILL.md","branch":"master","category":"testing"}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	registry, err := fetchRegistryFromBaseURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://github.com/owner/repo/tree/master/.claude/skills/legacy/SKILL.md"
	if got := registry.Skills[0].Install; got != want {
		t.Fatalf("unexpected non-main install: got %s want %s", got, want)
	}
}

func TestFetchRegistryShardFailureIncludesPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/registry.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"deprecated_full_payload":true,"manifest":"registry-manifest.json"}`)
	})
	mux.HandleFunc("/registry-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"shards":[{"path":"registry-shards/missing.json","count":1}]}`)
	})
	mux.HandleFunc("/registry-shards/missing.json", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusInternalServerError)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	_, err := fetchRegistryFromBaseURL(server.URL)
	if err == nil || !strings.Contains(err.Error(), "registry-shards/missing.json") {
		t.Fatalf("expected shard path in error, got %v", err)
	}
}

func TestFetchRegistryEmptyShardPathErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/registry.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"deprecated_full_payload":true,"manifest":"registry-manifest.json"}`)
	})
	mux.HandleFunc("/registry-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"shards":[{}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	_, err := fetchRegistryFromBaseURL(server.URL)
	if err == nil || !strings.Contains(err.Error(), "empty shard path") {
		t.Fatalf("expected empty shard path error, got %v", err)
	}
}

func TestFetchRegistryWithSourceDoesNotCachePointerOnFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/registry.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"deprecated_full_payload":true,"manifest":"registry-manifest.json","total_count":1}`)
	})
	mux.HandleFunc("/registry-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"total_count":1,"shards":[{"path":"registry-shards/00.json","count":1}]}`)
	})
	mux.HandleFunc("/registry-shards/00.json", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "shard unavailable", http.StatusInternalServerError)
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	writeConfigForRegistryTest(t, server.URL)

	if _, _, err := FetchRegistryWithSource(); err == nil {
		t.Fatal("expected fetch failure")
	}
	if _, err := os.Stat(configRegistryCachePathForTest(t)); !os.IsNotExist(err) {
		t.Fatalf("expected no registry cache file after failed shard fetch, got %v", err)
	}
}

func TestGetByCategoryFallbackUsesFullRegistryManifest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/categories/testing.json", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "category unavailable", http.StatusInternalServerError)
	})
	mux.HandleFunc("/registry.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"deprecated_full_payload":true,"manifest":"registry-manifest.json","total_count":1}`)
	})
	mux.HandleFunc("/registry-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"total_count":1,"shards":[{"path":"registry-shards/00.json","count":1}]}`)
	})
	mux.HandleFunc("/registry-shards/00.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"count":1,"skills":[{"name":"frontend-testing","description":"testing skill","repo":"owner/repo","path":".agents/skills/frontend-testing/SKILL.md","category":"testing"}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	writeConfigForRegistryTest(t, server.URL)

	skills, source, err := GetByCategoryWithSource("testing")
	if err != nil {
		t.Fatal(err)
	}
	if source != RegistrySourceRemote {
		t.Fatalf("expected remote source, got %s", source)
	}
	if len(skills) != 1 {
		t.Fatalf("expected one fallback skill, got %d", len(skills))
	}
	if got := skills[0].Install; got != "owner/repo/.agents/skills/frontend-testing/SKILL.md" {
		t.Fatalf("unexpected synthesized install: %s", got)
	}
}

func TestRegistryCacheRejectsPointerOnlyPayload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	path := configRegistryCachePathForTest(t)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"deprecated_full_payload":true,"manifest":"registry-manifest.json"}`), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := loadRegistryCache(); err == nil {
		t.Fatal("expected pointer-only registry cache to be rejected")
	}
	if err := saveRegistryCache(&Registry{DeprecatedFullPayload: true, Manifest: "registry-manifest.json"}); err == nil {
		t.Fatal("expected pointer-only registry cache save to be rejected")
	}
}

func TestFetchRegistryWithSourceUsesValidCacheBeforeRemote(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	path := configRegistryCachePathForTest(t)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":"cached","total_count":1,"skills":[{"name":"cached-skill","install":"owner/repo/.claude/skills/cached/SKILL.md","branch":"master"}]}`), 0644); err != nil {
		t.Fatal(err)
	}

	registry, source, err := FetchRegistryWithSource()
	if err != nil {
		t.Fatal(err)
	}
	if source != RegistrySourceCache {
		t.Fatalf("expected cache source, got %s", source)
	}
	if registry.Version != "cached" || len(registry.Skills) != 1 {
		t.Fatalf("unexpected cached registry: %#v", registry)
	}
	want := "https://github.com/owner/repo/tree/master/.claude/skills/cached/SKILL.md"
	if got := registry.Skills[0].Install; got != want {
		t.Fatalf("unexpected cached install: got %s want %s", got, want)
	}
}

func TestFetchSearchIndexFollowsManifestShards(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search-index.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"deprecated_full_payload":true,"manifest":"search-index-manifest.json","v":"2026-05-24","t":2}`)
	})
	mux.HandleFunc("/search-index-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"v":"2026-05-24","total_count":2,"shards":[{"gzip_path":"search-shards/part-000.json.gz","count":2}]}`)
	})
	mux.HandleFunc("/search-shards/part-000.json.gz", func(w http.ResponseWriter, r *http.Request) {
		writeGzip(t, w, `{"v":"2026-05-24","count":2,"s":[{"n":"frontend-testing","d":"testing skill","c":"tst","g":["test"],"r":10,"i":"owner/repo/.agents/skills/frontend-testing/SKILL.md","b":"main"},{"n":"docx","d":"docs","c":"doc","g":[],"r":5,"i":"anthropics/skills/skills/docx","b":"main"}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	idx, err := fetchSearchIndexFromBaseURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if idx.TotalCount != 2 || len(idx.Skills) != 2 {
		t.Fatalf("expected two skills, got total=%d len=%d", idx.TotalCount, len(idx.Skills))
	}
	if idx.Skills[0].Install != "owner/repo/.agents/skills/frontend-testing/SKILL.md" {
		t.Fatalf("unexpected install: %s", idx.Skills[0].Install)
	}
}

func TestFetchCategoryFollowsManifestParts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/categories/testing.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"category":"testing","code":"testing","count":1,"deprecated_full_payload":true,"manifest":"categories/testing/manifest.json"}`)
	})
	mux.HandleFunc("/categories/testing/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"category":"testing","code":"testing","count":1,"parts":[{"gzip_path":"categories/testing/part-000.json.gz","count":1}]}`)
	})
	mux.HandleFunc("/categories/testing/part-000.json.gz", func(w http.ResponseWriter, r *http.Request) {
		writeGzip(t, w, `{"category":"testing","count":1,"skills":[{"name":"frontend-testing","description":"testing skill","install":"owner/repo/.agents/skills/frontend-testing/SKILL.md","branch":"master","category":"testing"}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	category, err := fetchCategoryFromBaseURL(server.URL, "testing")
	if err != nil {
		t.Fatal(err)
	}
	if category.Count != 1 || len(category.Skills) != 1 {
		t.Fatalf("expected one skill, got total=%d len=%d", category.Count, len(category.Skills))
	}
	if category.Skills[0].Name != "frontend-testing" {
		t.Fatalf("unexpected skill: %s", category.Skills[0].Name)
	}
	want := "https://github.com/owner/repo/tree/master/.agents/skills/frontend-testing/SKILL.md"
	if got := category.Skills[0].Install; got != want {
		t.Fatalf("unexpected category install: got %s want %s", got, want)
	}
}

func TestDedupeSkillsUsesInstallCommand(t *testing.T) {
	skills := []Skill{
		{Name: "frontend-testing-owner-repo", Install: "owner/repo/.agents/skills/frontend-testing/SKILL.md"},
		{Name: "frontend-testing", Install: "owner/repo/.agents/skills/frontend-testing/SKILL.md"},
		{Name: "docx", Install: "anthropics/skills/skills/docx"},
	}

	got := dedupeSkills(skills)
	if len(got) != 2 {
		t.Fatalf("expected 2 unique skills, got %d", len(got))
	}
	if got[0].Name != "frontend-testing" {
		t.Fatalf("expected first skill to win, got %s", got[0].Name)
	}
}

func TestInstallableSkillRefRejectsCommandMarkdown(t *testing.T) {
	valid := []string{
		"anthropics/skills/skills/docx",
		"langgenius/dify/.agents/skills/frontend-testing/SKILL.md",
		"redmage123/salesforce/.agents/project_analysis_agent_SKILL.md",
	}
	for _, install := range valid {
		if !isInstallableSkillRef(install) {
			t.Fatalf("expected installable ref: %s", install)
		}
	}

	invalid := "udecode/plate/.claude/commands/sync-testing-skill.md"
	if isInstallableSkillRef(invalid) {
		t.Fatalf("expected command markdown to be rejected: %s", invalid)
	}
}

func TestResolveInstallFromIndexRejectsCommandMarkdown(t *testing.T) {
	idx := &SearchIndex{
		Skills: []SearchIndexEntry{
			{Name: "sync-testing-skill", Install: "udecode/plate/.claude/commands/sync-testing-skill.md"},
			{Name: "frontend-testing", Install: "langgenius/dify/.agents/skills/frontend-testing/SKILL.md"},
		},
	}

	if _, err := resolveInstallFromIndex(idx, "sync-testing-skill"); err == nil {
		t.Fatal("expected command markdown ref to be rejected")
	}

	install, err := resolveInstallFromIndex(idx, "frontend-testing")
	if err != nil {
		t.Fatal(err)
	}
	if install != "langgenius/dify/.agents/skills/frontend-testing/SKILL.md" {
		t.Fatalf("unexpected install: %s", install)
	}
}

func TestResolveInstallUsesCompactSearchIndex(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/search-index.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"deprecated_full_payload":true,"manifest":"search-index-manifest.json","v":"2026-05-24","t":1}`)
	})
	mux.HandleFunc("/docs/search-index-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"v":"2026-05-24","total_count":1,"shards":[{"path":"search-shards/part-000.json","count":1}]}`)
	})
	mux.HandleFunc("/docs/search-shards/part-000.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"v":"2026-05-24","count":1,"s":[{"n":"frontend-testing","d":"testing skill","c":"tst","g":["test"],"r":10,"i":"owner/repo/.agents/skills/frontend-testing/SKILL.md","b":"master"}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	writeConfigForRegistryTest(t, server.URL)

	install, source, err := ResolveInstall("frontend-testing")
	if err != nil {
		t.Fatal(err)
	}
	if source != RegistrySourceRemote {
		t.Fatalf("expected remote source, got %s", source)
	}
	want := "https://github.com/owner/repo/tree/master/.agents/skills/frontend-testing/SKILL.md"
	if install != want {
		t.Fatalf("unexpected install: got %s want %s", install, want)
	}
}

func writeGzip(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(b.Bytes()); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
}

func configRegistryCachePathForTest(t *testing.T) string {
	t.Helper()
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(cacheDir, "sk", "registry.json")
}

func writeConfigForRegistryTest(t *testing.T, registryURL string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	body := []byte(`{"registry":"` + registryURL + `","registry_ttl_hours":24}`)
	if err := os.WriteFile(filepath.Join(home, ".skrc"), body, 0644); err != nil {
		t.Fatal(err)
	}
}

// The compact search index carries no featured flag, so every skill produced
// from it has Featured == false. `sk search` relies on this: it deliberately
// renders no featured badge. If the index ever gains the field, revisit
// searchRegistry in cmd/search.go.
func TestEntryToSkillNeverMarksFeatured(t *testing.T) {
	skill := entryToSkill(SearchIndexEntry{
		Name:        "frontend-testing",
		Description: "Testing skill",
		Install:     "owner/repo/.agents/skills/frontend-testing/SKILL.md",
		Branch:      "main",
		Stars:       42,
	})
	if skill.Featured {
		t.Fatal("search index entries cannot be featured")
	}
}
