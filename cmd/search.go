package cmd

import (
	"fmt"
	"strings"

	"github.com/majiayu000/claude-skill-manager/internal/registry"
	"github.com/majiayu000/claude-skill-manager/pkg/styles"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:     "search [keyword]",
	Aliases: []string{"s", "find"},
	Short:   "Search for skills in the registry",
	Long: `Search for Claude Code skills in the registry.

Uses the skill-registry for fast and reliable results.`,
	Example: `  sk search testing
  sk search pdf
  sk search --category documents
  sk search --popular`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		popular, _ := cmd.Flags().GetBool("popular")
		category, _ := cmd.Flags().GetString("category")

		fmt.Println()

		// Show popular/featured skills
		if popular || (len(args) == 0 && category == "") {
			showFeaturedSkills()
			return
		}

		// Show by category
		if category != "" {
			showByCategory(category)
			return
		}

		// Search by keyword
		keyword := args[0]
		searchRegistry(keyword)
	},
}

func showFeaturedSkills() {
	fmt.Println(styles.TitleStyle.Render(styles.IconStar + " Popular Skills (Top 100)"))
	fmt.Println()

	featured, err := registry.FetchFeatured()
	if err != nil {
		fmt.Println(styles.WarningStyle.Render("Could not fetch featured skills: " + err.Error()))
		fmt.Println(styles.MutedStyle.Render("Showing fallback list..."))
		fmt.Println()
		showFallbackSkills()
		return
	}

	for i, skill := range featured.Skills {
		starStr := ""
		if skill.Stars > 0 {
			starStr = fmt.Sprintf(" %s%d", styles.IconStar, skill.Stars)
		}

		fmt.Printf("  %s %-28s%s\n",
			styles.SuccessStyle.Render(fmt.Sprintf("%2d.", i+1)),
			styles.SkillNameStyle.Render(skill.Name),
			styles.MutedStyle.Render(starStr),
		)
		if skill.Description != "" {
			desc := skill.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			fmt.Printf("      %s\n", styles.SkillDescStyle.Render(desc))
		}
		fmt.Printf("      %s sk install %s\n\n",
			styles.MutedStyle.Render(styles.IconArrow),
			skill.Install,
		)
	}

	fmt.Println(styles.MutedStyle.Render("─────────────────────────────────────────────────"))
	updatedAt := featured.UpdatedAt
	if len(updatedAt) >= 10 {
		updatedAt = updatedAt[:10]
	}
	fmt.Printf("%s %d featured skills | Updated: %s\n",
		styles.MutedStyle.Render(styles.IconInfo),
		featured.Count,
		updatedAt,
	)
	fmt.Println()
}

func showFallbackSkills() {
	// Hardcoded fallback when registry is unavailable
	skills := []struct {
		name, install, desc string
	}{
		{"docx", "anthropics/skills/docx", "Document creation and editing"},
		{"pdf", "anthropics/skills/pdf", "PDF document manipulation"},
		{"pptx", "anthropics/skills/pptx", "PowerPoint presentations"},
		{"superpowers", "obra/superpowers", "20+ battle-tested skills"},
	}

	for _, s := range skills {
		fmt.Printf("    %s %-22s\n",
			styles.SuccessStyle.Render(styles.IconPackage),
			s.name,
		)
		fmt.Printf("       %s\n", styles.SkillDescStyle.Render(s.desc))
		fmt.Printf("       %s sk install %s\n\n",
			styles.MutedStyle.Render(styles.IconArrow),
			s.install,
		)
	}
}

func showByCategory(category string) {
	fmt.Printf("%s Category: %s\n\n",
		styles.TitleStyle.Render(styles.IconFolder),
		styles.CodeStyle.Render(category),
	)

	skills, source, err := registry.GetByCategoryWithSource(category)
	if err != nil {
		fmt.Println(styles.RenderError("Failed to fetch category: " + err.Error()))
		showAvailableCategories()
		return
	}
	printRegistrySource(source)

	if len(skills) == 0 {
		fmt.Println(styles.MutedStyle.Render("No skills found in this category."))
		fmt.Println()
		showAvailableCategories()
		return
	}

	// Limit display to top 50 for large categories
	displaySkills := skills
	truncated := false
	if len(displaySkills) > 50 {
		displaySkills = displaySkills[:50]
		truncated = true
	}

	for _, skill := range displaySkills {
		fmt.Printf("  %s %s",
			styles.SuccessStyle.Render(styles.IconPackage),
			styles.SkillNameStyle.Render(skill.Name),
		)
		if skill.Stars > 0 {
			fmt.Printf("  %s%d", styles.MutedStyle.Render(styles.IconStar), skill.Stars)
		}
		fmt.Println()

		if skill.Description != "" {
			desc := skill.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			fmt.Printf("     %s\n", styles.SkillDescStyle.Render(desc))
		}
		fmt.Printf("     %s sk install %s\n\n",
			styles.MutedStyle.Render(styles.IconArrow),
			skill.Install,
		)
	}

	if truncated {
		fmt.Printf("%s Showing top 50 of %d skills in '%s'\n\n",
			styles.MutedStyle.Render(styles.IconInfo),
			len(skills),
			category,
		)
	} else {
		fmt.Printf("%s Found %d skill(s) in '%s'\n\n",
			styles.MutedStyle.Render(styles.IconInfo),
			len(skills),
			category,
		)
	}
}

func showAvailableCategories() {
	idx, err := registry.FetchCategoryIndex()
	if err != nil {
		return
	}

	fmt.Println(styles.MutedStyle.Render("Available categories:"))
	for _, cat := range idx.Categories {
		if cat.Count > 10 {
			fmt.Printf("  %s %-24s %s\n",
				styles.MutedStyle.Render(styles.IconFolder),
				cat.Name,
				styles.MutedStyle.Render(fmt.Sprintf("(%d skills)", cat.Count)),
			)
		}
	}
	fmt.Println()
}

func searchRegistry(keyword string) {
	fmt.Printf("%s Searching for '%s'...\n\n",
		styles.SpinnerStyle.Render(styles.IconSearch),
		styles.CodeStyle.Render(keyword),
	)

	skills, source, err := registry.SearchWithSource(keyword)
	if err != nil {
		fmt.Println(styles.RenderError("Search failed: " + err.Error()))
		return
	}
	printRegistrySource(source)

	if len(skills) == 0 {
		fmt.Println(styles.MutedStyle.Render("No skills found matching your query."))
		fmt.Println()
		fmt.Println(styles.MutedStyle.Render("Try:"))
		fmt.Println(styles.MutedStyle.Render("  • sk search --popular"))
		fmt.Println(styles.MutedStyle.Render("  • sk search --category documents"))
		fmt.Println(styles.MutedStyle.Render("  • Browse: https://skillsmp.com"))
		fmt.Println()
		return
	}

	total := len(skills)
	// Limit display to top 30 results
	if len(skills) > 30 {
		skills = skills[:30]
	}

	fmt.Printf("%s Found %d skill(s):\n\n",
		styles.SuccessStyle.Render(styles.IconCheck),
		total,
	)

	for i, skill := range skills {
		// Highlight matched keyword
		name := skill.Name
		desc := skill.Description

		fmt.Printf("%s %s",
			styles.SuccessStyle.Render(fmt.Sprintf("%2d.", i+1)),
			styles.SkillNameStyle.Render(name),
		)

		if skill.Stars > 0 {
			fmt.Printf("  %s %d",
				styles.MutedStyle.Render(styles.IconStar),
				skill.Stars,
			)
		}

		fmt.Println()

		if desc != "" {
			if len(desc) > 70 {
				desc = desc[:67] + "..."
			}
			fmt.Printf("    %s\n", styles.SkillDescStyle.Render(desc))
		}

		// Tags
		if len(skill.Tags) > 0 {
			tags := strings.Join(skill.Tags, ", ")
			if len(tags) > 50 {
				tags = tags[:47] + "..."
			}
			fmt.Printf("    %s %s\n",
				styles.MutedStyle.Render("tags:"),
				styles.MutedStyle.Render(tags),
			)
		}

		fmt.Printf("    %s sk install %s\n\n",
			styles.MutedStyle.Render(styles.IconArrow),
			skill.Install,
		)
	}

	if total > len(skills) {
		fmt.Printf("%s Showing top %d of %d results. Use --category to narrow down.\n\n",
			styles.MutedStyle.Render(styles.IconInfo),
			len(skills),
			total,
		)
	}
}

func printRegistrySource(source registry.RegistrySource) {
	message := registrySourceMessage(source)
	if message == "" {
		return
	}
	fmt.Println(styles.MutedStyle.Render(message))
	fmt.Println()
}

func registrySourceMessage(source registry.RegistrySource) string {
	switch source {
	case registry.RegistrySourceRemote:
		return "Using remote registry data..."
	case registry.RegistrySourceCache:
		return "Using cached registry data..."
	default:
		return ""
	}
}

func init() {
	searchCmd.Flags().BoolP("popular", "p", false, "Show popular/featured skills")
	searchCmd.Flags().StringP("category", "c", "", "Filter by category (documents, development, design, testing)")
	rootCmd.AddCommand(searchCmd)
}
