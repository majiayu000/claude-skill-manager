package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/majiayu000/claude-skill-manager/internal/skill"
	"github.com/majiayu000/claude-skill-manager/pkg/styles"
	"github.com/spf13/cobra"
)

var uninstallForce bool

var uninstallCmd = &cobra.Command{
	Use:     "uninstall <skill-name>",
	Aliases: []string{"rm", "remove", "delete"},
	Short:   "Remove an installed skill",
	Long:    `Remove a skill from your Claude Code skills directory.`,
	Example: `  sk uninstall my-skill
  sk rm my-skill --force`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		// Check if exists
		s, err := skill.Get(name)
		if err != nil {
			fmt.Println(styles.RenderError("Failed to check skill: " + err.Error()))
			os.Exit(1)
		}
		if s == nil {
			fmt.Println(styles.RenderError(fmt.Sprintf("Skill '%s' is not installed.", name)))
			os.Exit(1)
		}

		// Confirm unless --force
		if !uninstallForce {
			var confirm bool
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Remove skill '%s'?", name)).
				Description("This action cannot be undone.").
				Affirmative("Yes, remove").
				Negative("Cancel").
				Value(&confirm).
				Run()

			if err != nil || !confirm {
				fmt.Println(styles.MutedStyle.Render("Cancelled."))
				return
			}
		}

		// Remove
		if err := skill.Remove(name); err != nil {
			fmt.Println(styles.RenderError("Failed to remove skill: " + err.Error()))
			os.Exit(1)
		}

		fmt.Println()
		fmt.Println(styles.RenderSuccess(fmt.Sprintf("Removed skill '%s'", name)))
		fmt.Println()
	},
}

func init() {
	uninstallCmd.Flags().BoolVarP(&uninstallForce, "force", "f", false, "Skip confirmation")
	rootCmd.AddCommand(uninstallCmd)
}
