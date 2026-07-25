package registry

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/majiayu000/claude-skill-manager/internal/config"
)

// Registry represents the skill registry
type Registry struct {
	Version               string  `json:"version"`
	UpdatedAt             string  `json:"updated_at"`
	TotalCount            int     `json:"total_count"`
	Skills                []Skill `json:"skills"`
	DeprecatedFullPayload bool    `json:"deprecated_full_payload"`
	Manifest              string  `json:"manifest"`
}

// RegistrySource indicates where registry data came from.
type RegistrySource string

const (
	RegistrySourceRemote RegistrySource = "remote"
	RegistrySourceCache  RegistrySource = "cache"
)

// Skill represents a skill in the registry
type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Install     string   `json:"install"`
	Repo        string   `json:"repo"`
	Path        string   `json:"path"`
	Branch      string   `json:"branch"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Source      string   `json:"source"`
	Stars       int      `json:"stars"`
	Featured    bool     `json:"featured"`
}

// GitHubURL returns the GitHub URL for viewing this skill's SKILL.md
func (s *Skill) GitHubURL() string {
	if s.Repo == "" {
		return ""
	}
	branch := s.Branch
	if branch == "" {
		branch = "main"
	}
	if s.Path != "" {
		return fmt.Sprintf("https://github.com/%s/blob/%s/%s/SKILL.md", s.Repo, branch, s.Path)
	}
	return fmt.Sprintf("https://github.com/%s/blob/%s/SKILL.md", s.Repo, branch)
}

// Category represents a category index
type Category struct {
	Category              string  `json:"category"`
	Code                  string  `json:"code"`
	UpdatedAt             string  `json:"updated_at"`
	Count                 int     `json:"count"`
	Skills                []Skill `json:"skills"`
	DeprecatedFullPayload bool    `json:"deprecated_full_payload"`
	Manifest              string  `json:"manifest"`
}

// Featured represents the featured skills response
type Featured struct {
	UpdatedAt string  `json:"updated_at"`
	Count     int     `json:"count"`
	Skills    []Skill `json:"skills"`
}

// CategoryIndexEntry represents one entry in the category index
type CategoryIndexEntry struct {
	Name  string `json:"name"`
	Code  string `json:"code"`
	Count int    `json:"count"`
}

// CategoryIndex represents the category index
type CategoryIndex struct {
	UpdatedAt  string               `json:"updated_at"`
	Categories []CategoryIndexEntry `json:"categories"`
}

// SearchIndexEntry represents one skill in the compact search index
type SearchIndexEntry struct {
	Name        string   `json:"n"`
	Description string   `json:"d"`
	Category    string   `json:"c"`
	Tags        []string `json:"g"`
	Stars       int      `json:"r"`
	Install     string   `json:"i"`
	Branch      string   `json:"b"`
}

// SearchIndex represents the compact search index
type SearchIndex struct {
	Version    string             `json:"v"`
	TotalCount int                `json:"t"`
	Skills     []SearchIndexEntry `json:"s"`
}

type artifactPart struct {
	Path     string `json:"path"`
	GzipPath string `json:"gzip_path"`
	Count    int    `json:"count"`
}

type searchIndexPayload struct {
	Version               string             `json:"v"`
	TotalCount            int                `json:"t"`
	Skills                []SearchIndexEntry `json:"s"`
	DeprecatedFullPayload bool               `json:"deprecated_full_payload"`
	Manifest              string             `json:"manifest"`
}

type searchManifest struct {
	Version    string         `json:"v"`
	TotalCount int            `json:"total_count"`
	Shards     []artifactPart `json:"shards"`
}

type searchShard struct {
	Version string             `json:"v"`
	Count   int                `json:"count"`
	Skills  []SearchIndexEntry `json:"s"`
}

type categoryManifest struct {
	Category  string         `json:"category"`
	Code      string         `json:"code"`
	UpdatedAt string         `json:"updated_at"`
	Count     int            `json:"count"`
	Parts     []artifactPart `json:"parts"`
}

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

func registryBaseURL() string {
	base := config.GetRegistryBaseURL()
	if base == "" || base == "github" {
		return config.DefaultRegistryURL
	}
	return strings.TrimRight(base, "/")
}

func docsBaseURL() string {
	return registryBaseURL() + "/docs"
}

func artifactURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func fetchJSON(url string, target any) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("returned status %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if strings.HasSuffix(url, ".gz") {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer func() { _ = gzReader.Close() }()
		reader = gzReader
	}

	if err := json.NewDecoder(reader).Decode(target); err != nil {
		return err
	}
	return nil
}

// FetchRegistry fetches the full registry
func FetchRegistry() (*Registry, error) {
	registry, _, err := FetchRegistryWithSource()
	return registry, err
}

// FetchRegistryWithSource fetches the full registry and indicates data source.
func FetchRegistryWithSource() (*Registry, RegistrySource, error) {
	if cached, cacheErr := loadRegistryCache(); cacheErr == nil {
		return cached, RegistrySourceCache, nil
	}

	registry, err := fetchRegistryFromBaseURL(registryBaseURL())
	if err != nil {
		if cached, cacheErr := loadRegistryCache(); cacheErr == nil {
			return cached, RegistrySourceCache, nil
		}
		return nil, "", fmt.Errorf("failed to fetch registry: %w", err)
	}

	if err := saveRegistryCache(registry); err != nil {
		return registry, RegistrySourceRemote, nil
	}
	return registry, RegistrySourceRemote, nil
}

// FetchFeatured fetches the top featured skills (52KB vs 44MB full registry)
func FetchFeatured() (*Featured, error) {
	url := docsBaseURL() + "/featured.json"

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch featured: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("featured returned status %d", resp.StatusCode)
	}

	var featured Featured
	if err := json.NewDecoder(resp.Body).Decode(&featured); err != nil {
		return nil, fmt.Errorf("failed to parse featured: %w", err)
	}

	return &featured, nil
}

// FetchCategory fetches skills for a specific category from docs
func FetchCategory(category string) (*Category, error) {
	return fetchCategoryFromBaseURL(docsBaseURL(), category)
}

func fetchCategoryFromBaseURL(baseURL, category string) (*Category, error) {
	var cat Category
	url := artifactURL(baseURL, fmt.Sprintf("categories/%s.json", category))
	if err := fetchJSON(url, &cat); err != nil {
		return nil, fmt.Errorf("failed to fetch category: %w", err)
	}

	if cat.DeprecatedFullPayload && cat.Manifest != "" {
		return fetchCategoryFromManifest(baseURL, cat.Manifest)
	}

	normalizeRegistrySkills(cat.Skills)
	return &cat, nil
}

func fetchCategoryFromManifest(baseURL, manifestPath string) (*Category, error) {
	var manifest categoryManifest
	if err := fetchJSON(artifactURL(baseURL, manifestPath), &manifest); err != nil {
		return nil, fmt.Errorf("failed to fetch category manifest: %w", err)
	}

	category := &Category{
		Category:  manifest.Category,
		Code:      manifest.Code,
		UpdatedAt: manifest.UpdatedAt,
		Count:     manifest.Count,
	}

	for _, part := range manifest.Parts {
		partPath := part.GzipPath
		if partPath == "" {
			partPath = part.Path
		}
		if partPath == "" {
			return nil, fmt.Errorf("category manifest contains empty part path")
		}

		var payload Category
		if err := fetchJSON(artifactURL(baseURL, partPath), &payload); err != nil {
			return nil, fmt.Errorf("failed to fetch category part %s: %w", partPath, err)
		}
		normalizeRegistrySkills(payload.Skills)
		category.Skills = append(category.Skills, payload.Skills...)
	}

	if category.Count == 0 {
		category.Count = len(category.Skills)
	}
	return category, nil
}

// FetchCategoryIndex fetches the category index listing all available categories
func FetchCategoryIndex() (*CategoryIndex, error) {
	url := docsBaseURL() + "/categories/index.json"

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch category index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("category index returned status %d", resp.StatusCode)
	}

	var idx CategoryIndex
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return nil, fmt.Errorf("failed to parse category index: %w", err)
	}

	return &idx, nil
}

// FetchSearchIndex fetches the search index, following the registry shard
// manifest when the compatibility entry point is a pointer.
func FetchSearchIndex() (*SearchIndex, RegistrySource, error) {
	if cached, err := loadSearchIndexCache(); err == nil {
		return cached, RegistrySourceCache, nil
	}

	idx, err := fetchSearchIndexFromBaseURL(docsBaseURL())
	if err != nil {
		return nil, "", err
	}

	_ = saveSearchIndexCache(idx)
	return idx, RegistrySourceRemote, nil
}

func fetchSearchIndexFromBaseURL(baseURL string) (*SearchIndex, error) {
	idx, err := fetchSearchIndexFromPath(baseURL, "search-index.json")
	if err == nil {
		return idx, nil
	}
	idx, gzipErr := fetchSearchIndexFromPath(baseURL, "search-index.json.gz")
	if gzipErr == nil {
		return idx, nil
	}
	return nil, fmt.Errorf("failed to fetch search index: %w", err)
}

func fetchSearchIndexFromPath(baseURL, indexPath string) (*SearchIndex, error) {
	var payload searchIndexPayload
	if err := fetchJSON(artifactURL(baseURL, indexPath), &payload); err != nil {
		return nil, err
	}

	if payload.DeprecatedFullPayload && payload.Manifest != "" {
		return fetchSearchIndexFromManifest(baseURL, payload.Manifest, payload.Version, payload.TotalCount)
	}

	return &SearchIndex{
		Version:    payload.Version,
		TotalCount: payload.TotalCount,
		Skills:     payload.Skills,
	}, nil
}

func fetchSearchIndexFromManifest(baseURL, manifestPath, fallbackVersion string, fallbackTotal int) (*SearchIndex, error) {
	var manifest searchManifest
	if err := fetchJSON(artifactURL(baseURL, manifestPath), &manifest); err != nil {
		return nil, fmt.Errorf("failed to fetch search manifest: %w", err)
	}

	idx := &SearchIndex{
		Version:    manifest.Version,
		TotalCount: manifest.TotalCount,
	}
	if idx.Version == "" {
		idx.Version = fallbackVersion
	}
	if idx.TotalCount == 0 {
		idx.TotalCount = fallbackTotal
	}

	for _, shard := range manifest.Shards {
		shardPath := shard.GzipPath
		if shardPath == "" {
			shardPath = shard.Path
		}
		if shardPath == "" {
			return nil, fmt.Errorf("search manifest contains empty shard path")
		}

		var payload searchShard
		if err := fetchJSON(artifactURL(baseURL, shardPath), &payload); err != nil {
			return nil, fmt.Errorf("failed to fetch search shard %s: %w", shardPath, err)
		}
		idx.Skills = append(idx.Skills, payload.Skills...)
	}

	if idx.TotalCount == 0 {
		idx.TotalCount = len(idx.Skills)
	}
	return idx, nil
}

// categoryCodeToName maps short category codes back to full names
var categoryCodeToName = map[string]string{
	"dev": "development",
	"ops": "devops",
	"sec": "security",
	"doc": "documents",
	"des": "design",
	"tst": "testing",
	"prd": "product",
	"mkt": "marketing",
	"pro": "productivity",
	"dat": "data",
	"off": "official",
	"oth": "other",
}

// entryToSkill converts a compact SearchIndexEntry to a full Skill
func entryToSkill(e SearchIndexEntry) Skill {
	// Resolve category code to full name
	cat := e.Category
	if full, ok := categoryCodeToName[cat]; ok {
		cat = full
	}

	install := normalizeInstallForBranch(e.Install, e.Branch)

	return Skill{
		Name:        e.Name,
		Description: e.Description,
		Install:     install,
		Repo:        repoFromInstallRef(install),
		Branch:      e.Branch,
		Category:    cat,
		Tags:        e.Tags,
		Stars:       e.Stars,
	}
}

// Search searches for skills matching the keyword
func Search(keyword string) ([]Skill, error) {
	skills, _, err := SearchWithSource(keyword)
	return skills, err
}

// SearchWithSource searches using the compact search index (9MB gzip vs 44MB full registry)
func SearchWithSource(keyword string) ([]Skill, RegistrySource, error) {
	idx, source, err := FetchSearchIndex()
	if err != nil {
		return nil, "", err
	}

	keyword = strings.ToLower(keyword)
	var results []Skill

	for _, entry := range idx.Skills {
		if !isInstallableSkillRef(entry.Install) {
			continue
		}

		if strings.Contains(strings.ToLower(entry.Name), keyword) {
			results = append(results, entryToSkill(entry))
			continue
		}

		if strings.Contains(strings.ToLower(entry.Description), keyword) {
			results = append(results, entryToSkill(entry))
			continue
		}

		for _, tag := range entry.Tags {
			if strings.Contains(strings.ToLower(tag), keyword) {
				results = append(results, entryToSkill(entry))
				break
			}
		}
	}

	return dedupeSkills(results), source, nil
}

// GetByCategory returns skills in a category
func GetByCategory(category string) ([]Skill, error) {
	skills, _, err := GetByCategoryWithSource(category)
	return skills, err
}

// GetByCategoryWithSource returns skills in a category and indicates data source.
func GetByCategoryWithSource(category string) ([]Skill, RegistrySource, error) {
	cat, err := FetchCategory(category)
	if err == nil {
		return dedupeSkills(cat.Skills), RegistrySourceRemote, nil
	}

	// Fallback to filtering from full registry.
	registry, source, err := FetchRegistryWithSource()
	if err != nil {
		return nil, "", err
	}

	var results []Skill
	for _, skill := range registry.Skills {
		if strings.EqualFold(skill.Category, category) {
			results = append(results, skill)
		}
	}
	return dedupeSkills(results), source, nil
}

// ResolveInstall finds a skill by name and returns its install string.
func ResolveInstall(name string) (string, RegistrySource, error) {
	idx, source, err := FetchSearchIndex()
	if err != nil {
		return "", "", err
	}

	install, err := resolveInstallFromIndex(idx, name)
	return install, source, err
}

func resolveInstallFromIndex(idx *SearchIndex, name string) (string, error) {
	for _, skill := range idx.Skills {
		install := normalizeInstallForBranch(skill.Install, skill.Branch)
		if !isInstallableSkillRef(install) {
			continue
		}
		if strings.EqualFold(skill.Name, name) {
			return install, nil
		}
	}

	return "", fmt.Errorf("no skill named %q in registry", name)
}

func loadRegistryCache() (*Registry, error) {
	path := config.RegistryCachePath()
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	ttl := time.Duration(config.GetRegistryTTL()) * time.Hour
	if time.Since(info.ModTime()) > ttl {
		return nil, fmt.Errorf("registry cache expired")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, err
	}
	if registry.DeprecatedFullPayload && len(registry.Skills) == 0 {
		return nil, fmt.Errorf("registry cache contains pointer without skills")
	}

	normalizeRegistrySkills(registry.Skills)
	return &registry, nil
}

func saveRegistryCache(registry *Registry) error {
	if registry.DeprecatedFullPayload && len(registry.Skills) == 0 {
		return fmt.Errorf("registry cache cannot save pointer without skills")
	}

	path := config.RegistryCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func searchIndexCachePath() string {
	return config.SearchIndexCachePath()
}

func loadSearchIndexCache() (*SearchIndex, error) {
	path := searchIndexCachePath()
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	ttl := time.Duration(config.GetRegistryTTL()) * time.Hour
	if time.Since(info.ModTime()) > ttl {
		return nil, fmt.Errorf("search index cache expired")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var idx SearchIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	if idx.TotalCount > 0 && len(idx.Skills) == 0 {
		return nil, fmt.Errorf("search index cache contains pointer without skills")
	}

	return &idx, nil
}

func saveSearchIndexCache(idx *SearchIndex) error {
	path := searchIndexCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func dedupeSkills(skills []Skill) []Skill {
	seen := make(map[string]int, len(skills))
	result := make([]Skill, 0, len(skills))

	for _, skill := range skills {
		if !isInstallableSkillRef(skill.Install) {
			continue
		}
		key := skill.Install
		if key == "" {
			key = skill.Name + "|" + skill.Repo + "|" + skill.Path
		}
		if idx, ok := seen[key]; ok {
			if preferSkillName(skill, result[idx]) {
				result[idx] = skill
			}
			continue
		}
		seen[key] = len(result)
		result = append(result, skill)
	}

	return result
}

func preferSkillName(candidate, existing Skill) bool {
	if candidate.Name == "" {
		return false
	}
	if existing.Name == "" {
		return true
	}
	return len(candidate.Name) < len(existing.Name)
}

func isInstallableSkillRef(install string) bool {
	install = strings.TrimSpace(install)
	if install == "" {
		return true
	}
	parts := strings.Split(install, "/")
	if len(parts) <= 2 {
		return true
	}

	base := strings.ToLower(parts[len(parts)-1])
	if !strings.HasSuffix(base, ".md") {
		return true
	}
	return base == "skill.md" || strings.HasSuffix(base, "_skill.md")
}
