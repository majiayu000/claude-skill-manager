package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/majiayu000/claude-skill-manager/internal/config"
	"github.com/majiayu000/claude-skill-manager/internal/registry"
	"github.com/majiayu000/claude-skill-manager/internal/skill"
	"github.com/majiayu000/claude-skill-manager/pkg/styles"
	"github.com/spf13/cobra"
)

var doctorRegistry bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check skills health",
	Long:  `Run diagnostics to check for common issues with your skills setup and registry cache.`,
	Run: func(cmd *cobra.Command, args []string) {
		if doctorRegistry {
			runRegistryDiagnostics()
			return
		}

		fmt.Println()
		fmt.Println(styles.TitleStyle.Render(styles.IconGear + " Skills Health Check"))
		fmt.Println()

		issues := 0

		// Check skills directory exists
		skillsDir := config.GetSkillsDir()
		if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
			fmt.Printf("  %s Skills directory does not exist: %s\n",
				styles.WarningStyle.Render(styles.IconWarning),
				skillsDir,
			)
			fmt.Printf("    %s Run %s to create it\n",
				styles.MutedStyle.Render(styles.IconArrow),
				styles.CodeStyle.Render("sk install <skill>"),
			)
			issues++
		} else {
			fmt.Printf("  %s Skills directory: %s\n",
				styles.SuccessStyle.Render(styles.IconCheck),
				skillsDir,
			)
		}

		// Check installed skills
		skills, err := skill.List()
		if err != nil {
			fmt.Printf("  %s Failed to list skills: %s\n",
				styles.ErrorStyle.Render(styles.IconCross),
				err.Error(),
			)
			issues++
		} else {
			fmt.Printf("  %s Installed skills: %d\n",
				styles.SuccessStyle.Render(styles.IconCheck),
				len(skills),
			)

			// Check each skill for issues
			for _, s := range skills {
				skillIssues := checkSkillHealth(s)
				if len(skillIssues) > 0 {
					fmt.Printf("\n  %s %s has issues:\n",
						styles.WarningStyle.Render(styles.IconWarning),
						s.Name,
					)
					for _, issue := range skillIssues {
						fmt.Printf("    %s %s\n",
							styles.MutedStyle.Render(styles.IconArrow),
							issue,
						)
					}
					issues += len(skillIssues)
				}
			}
		}

		// Summary
		fmt.Println()
		if issues == 0 {
			fmt.Println(styles.SuccessStyle.Render("  All checks passed! Your skills setup is healthy."))
		} else {
			fmt.Printf(styles.WarningStyle.Render("  Found %d issue(s). See above for details.\n"), issues)
		}
		fmt.Println()
	},
}

func checkSkillHealth(s skill.Skill) []string {
	var issues []string

	// Check SKILL.md exists
	skillMdPath := filepath.Join(s.Path, "SKILL.md")
	if _, err := os.Stat(skillMdPath); os.IsNotExist(err) {
		issues = append(issues, "Missing SKILL.md file")
	}

	// Check if directory is empty
	entries, err := os.ReadDir(s.Path)
	if err != nil {
		issues = append(issues, "Cannot read skill directory")
	} else if len(entries) == 0 {
		issues = append(issues, "Skill directory is empty")
	}

	// Check for description
	if s.Description == "" {
		issues = append(issues, "No description in SKILL.md")
	}

	return issues
}

type cacheInspection struct {
	Path   string
	State  string
	Detail string
}

func runRegistryDiagnostics() {
	ttl := time.Duration(config.GetRegistryTTL()) * time.Hour

	fmt.Println()
	fmt.Println(styles.TitleStyle.Render(styles.IconGear + " Registry Diagnostics"))
	fmt.Println()
	fmt.Printf("  %s Registry URL: %s\n", styles.SuccessStyle.Render(styles.IconCheck), config.GetRegistryBaseURL())
	fmt.Printf("  %s Config file: %s\n", styles.SuccessStyle.Render(styles.IconCheck), config.ConfigPath())
	fmt.Printf("  %s Cache TTL: %d hour(s)\n", styles.SuccessStyle.Render(styles.IconCheck), config.GetRegistryTTL())
	fmt.Println()

	printCacheInspection("Full registry cache", inspectCacheFile(config.RegistryCachePath(), ttl, validateRegistryCachePayload))
	printCacheInspection("Search index cache", inspectCacheFile(config.SearchIndexCachePath(), ttl, validateSearchIndexCachePayload))

	fmt.Println()
	fmt.Println(styles.MutedStyle.Render("Recovery: remove stale or malformed cache files, then rerun a registry command."))
	fmt.Printf("  rm -f %s %s\n", config.RegistryCachePath(), config.SearchIndexCachePath())
	fmt.Println()
}

func inspectCacheFile(path string, ttl time.Duration, validate func([]byte) error) cacheInspection {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cacheInspection{
				Path:   path,
				State:  "missing",
				Detail: "cache will be created after the next successful remote registry fetch",
			}
		}
		return cacheInspection{
			Path:   path,
			State:  "unreadable",
			Detail: err.Error(),
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cacheInspection{
			Path:   path,
			State:  "unreadable",
			Detail: err.Error(),
		}
	}
	if !json.Valid(data) {
		return cacheInspection{
			Path:   path,
			State:  "malformed",
			Detail: "file is not valid JSON; remove it to force a clean remote fetch",
		}
	}
	if validate != nil {
		if err := validate(data); err != nil {
			return cacheInspection{
				Path:   path,
				State:  "invalid",
				Detail: err.Error(),
			}
		}
	}
	if ttl > 0 && time.Since(info.ModTime()) > ttl {
		return cacheInspection{
			Path:   path,
			State:  "expired",
			Detail: fmt.Sprintf("last modified %s; remove it or rerun a registry command to refresh", info.ModTime().Format(time.RFC3339)),
		}
	}

	return cacheInspection{
		Path:   path,
		State:  "fresh",
		Detail: fmt.Sprintf("last modified %s", info.ModTime().Format(time.RFC3339)),
	}
}

func validateRegistryCachePayload(data []byte) error {
	var payload registry.Registry
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("file is not a valid full registry cache: %w", err)
	}
	if payload.DeprecatedFullPayload && len(payload.Skills) == 0 {
		return fmt.Errorf("registry cache contains pointer without skills; remove it to force a clean remote fetch")
	}
	return nil
}

func validateSearchIndexCachePayload(data []byte) error {
	var payload registry.SearchIndex
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("file is not a valid search index cache: %w", err)
	}
	if payload.TotalCount > 0 && len(payload.Skills) == 0 {
		return fmt.Errorf("search index cache contains pointer without skills; remove it to force a clean remote fetch")
	}
	return nil
}

func printCacheInspection(name string, inspection cacheInspection) {
	icon := styles.IconCheck
	style := styles.SuccessStyle
	if inspection.State != "fresh" {
		icon = styles.IconWarning
		style = styles.WarningStyle
	}

	fmt.Printf("  %s %s: %s\n", style.Render(icon), name, inspection.Path)
	fmt.Printf("    state: %s\n", inspection.State)
	fmt.Printf("    detail: %s\n", inspection.Detail)
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorRegistry, "registry", false, "Show registry configuration and cache diagnostics")
	rootCmd.AddCommand(doctorCmd)
}
