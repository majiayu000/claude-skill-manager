package cmd

import (
	"fmt"
	"os"

	"github.com/majiayu000/claude-skill-manager/pkg/styles"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sk",
	Short: "Claude Skills Manager",
	Long: styles.TitleStyle.Render(styles.Logo) + `
` + styles.SubtitleStyle.Render("The package manager for Claude Code skills") + `

` + styles.MutedStyle.Render("Commands:") + `
  ` + styles.SuccessStyle.Render("install") + `   Install a skill from GitHub
  ` + styles.SuccessStyle.Render("list") + `      List installed skills
  ` + styles.SuccessStyle.Render("uninstall") + ` Remove an installed skill
  ` + styles.SuccessStyle.Render("info") + `      Show skill details
  ` + styles.SuccessStyle.Render("update") + `    Update installed skills

` + styles.MutedStyle.Render("Examples:") + `
  sk install anthropics/skills/docx
  sk install https://github.com/user/repo
  sk list
  sk uninstall my-skill
`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, styles.RenderError(err.Error()))
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
