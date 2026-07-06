package cmd

import (
	"fmt"

	"github.com/majiayu000/claude-skill-manager/internal/skill"
	"github.com/majiayu000/claude-skill-manager/internal/ui"
	"github.com/majiayu000/claude-skill-manager/pkg/styles"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List installed skills",
	Long:    `List all skills installed in your Claude Code skills directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		skills, err := skill.List()
		if err != nil {
			fmt.Println(styles.RenderError("Failed to list skills: " + err.Error()))
			return
		}

		fmt.Println()
		fmt.Println(styles.TitleStyle.Render(styles.IconPackage + " Installed Skills"))
		fmt.Println()
		fmt.Println(ui.RenderSkillTable(skills))
		fmt.Println()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
