package cmd

import (
	"fmt"

	"github.com/majiayu000/claude-skill-manager/internal/skill"
	"github.com/majiayu000/claude-skill-manager/pkg/styles"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:     "update [skill-name]",
	Aliases: []string{"up", "upgrade"},
	Short:   "Update installed skills",
	Long: `Update one or all installed skills to their latest versions.

If no skill name is provided, all skills will be updated.`,
	Example: `  sk update           # Update all skills
  sk update my-skill  # Update specific skill`,
	Run: func(cmd *cobra.Command, args []string) {
		skills, err := skill.List()
		if err != nil {
			fmt.Println(styles.RenderError("Failed to list skills: " + err.Error()))
			return
		}

		if len(skills) == 0 {
			fmt.Println(styles.RenderWarning("No skills installed."))
			return
		}

		target := ""
		if len(args) > 0 {
			target = args[0]
		}

		if target != "" {
			found := false
			for _, s := range skills {
				if s.Name == target {
					found = true
					break
				}
			}
			if !found {
				fmt.Println(styles.RenderError(fmt.Sprintf("Skill '%s' is not installed.", target)))
				return
			}
		}

		fmt.Println()
		fmt.Println(styles.TitleStyle.Render(styles.IconSync + " Update not implemented"))
		fmt.Println()
		if target != "" {
			fmt.Printf("  %s %s\n",
				styles.MutedStyle.Render(styles.IconArrow),
				styles.MutedStyle.Render("Updates for a single skill are not supported yet."),
			)
		} else {
			fmt.Printf("  %s %s\n",
				styles.MutedStyle.Render(styles.IconArrow),
				styles.MutedStyle.Render("Bulk updates are not supported yet."),
			)
		}
		fmt.Println()
		fmt.Println(styles.MutedStyle.Render("  For now, use: sk uninstall <name> && sk install <source>"))
		fmt.Println()
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
