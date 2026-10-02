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
	RunE: func(cmd *cobra.Command, args []string) error {
		skills, err := skill.List()
		if err != nil {
			return fmt.Errorf("failed to list skills: %w", err)
		}

		fmt.Println()
		fmt.Println(styles.TitleStyle.Render(styles.IconPackage + " Installed Skills"))
		fmt.Println()
		fmt.Println(ui.RenderSkillTable(skills))
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
