package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the global configuration
type Config struct {
	SkillsDir        string `json:"skills_dir"`
	Registry         string `json:"registry"`
	RegistryTTLHours int    `json:"registry_ttl_hours"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		SkillsDir:        filepath.Join(homeDir, ".claude", "skills"),
		Registry:         "github",
		RegistryTTLHours: 24,
	}
}

// GetSkillsDir returns the skills directory path
func GetSkillsDir() string {
	cfg := Load()
	return cfg.SkillsDir
}

// GetRegistryTTL returns registry cache TTL in hours.
func GetRegistryTTL() int {
	cfg := Load()
	if cfg.RegistryTTLHours <= 0 {
		return DefaultConfig().RegistryTTLHours
	}
	return cfg.RegistryTTLHours
}

// GetRegistryBaseURL returns the registry base URL.
// If config uses legacy "github", return default registry URL.
func GetRegistryBaseURL() string {
	cfg := Load()
	if cfg.Registry == "" || cfg.Registry == "github" {
		return "https://raw.githubusercontent.com/majiayu000/claude-skill-registry/main"
	}
	return cfg.Registry
}

// ConfigPath returns the path to config file
func ConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".skrc")
}

// RegistryCachePath returns the registry cache file path.
func RegistryCachePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		homeDir, _ := os.UserHomeDir()
		cacheDir = filepath.Join(homeDir, ".cache")
	}
	return filepath.Join(cacheDir, "sk", "registry.json")
}

// SearchIndexCachePath returns the compact search index cache file path.
func SearchIndexCachePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		homeDir, _ := os.UserHomeDir()
		cacheDir = filepath.Join(homeDir, ".cache")
	}
	return filepath.Join(cacheDir, "sk", "search-index.json")
}

// Load loads configuration from file
func Load() *Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "Warning: failed to parse config file, using defaults:", err)
		return cfg
	}
	return cfg
}

// EnsureSkillsDir creates the skills directory if it doesn't exist
func EnsureSkillsDir() error {
	dir := GetSkillsDir()
	return os.MkdirAll(dir, 0755)
}
