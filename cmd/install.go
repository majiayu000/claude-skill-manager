package cmd

import (
	"fmt"
	"os"

	"github.com/majiayu000/claude-skill-manager/internal/github"
	"github.com/majiayu000/claude-skill-manager/internal/registry"
	"github.com/majiayu000/claude-skill-manager/internal/skill"
	"github.com/majiayu000/claude-skill-manager/internal/ui"
	"github.com/majiayu000/claude-skill-manager/pkg/styles"
	"github.com/spf13/cobra"
)

var (
	installName  string // custom name for the skill
	installForce bool   // force reinstall
)

var installCmd = &cobra.Command{
	Use:     "install <source>",
	Aliases: []string{"i", "add"},
	Short:   "Install a skill from GitHub",
	Long: `Install a Claude Code skill from GitHub.

Supported formats:
  <skill-name>                  Install from registry by name
  owner/repo                     Install entire repo
  owner/repo/path/to/skill       Install skill from subdirectory
  https://github.com/owner/repo  Full GitHub URL
`,
	Example: `  sk install anthropics/skills/docx
  sk install docx
  sk install obra/superpowers
  sk install https://github.com/user/repo`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		source := args[0]

		// Parse GitHub URL or resolve from registry by name
		info, err := github.ParseGitHubURL(source)
		if err != nil {
			install, regSource, regErr := registry.ResolveInstall(source)
			if regErr != nil {
				fmt.Println(styles.RenderError(err.Error()))
				fmt.Println(styles.MutedStyle.Render("Also tried registry lookup: " + regErr.Error()))
				os.Exit(1)
			}
			source = install
			info, err = github.ParseGitHubURL(source)
			if err != nil {
				fmt.Println(styles.RenderError(err.Error()))
				os.Exit(1)
			}
			printRegistrySource(regSource)
		}

		// Determine skill name
		skillName := installName
		if skillName == "" {
			skillName = github.GetSkillName(info)
		}

		// Resolve once: Exists/Get scan and parse every installed SKILL.md.
		// Prefer the matched directory path so --force replaces an aliased
		// install (dir basename != front-matter name) instead of creating a duplicate.
		existing, existingErr := skill.Get(skillName)
		if existingErr != nil {
			fmt.Println(styles.RenderError("Failed to check installed skills: " + existingErr.Error()))
			os.Exit(1)
		}
		alreadyInstalled := existing != nil

		if alreadyInstalled && !installForce {
			fmt.Println(styles.RenderWarning(fmt.Sprintf("Skill '%s' is already installed.", skillName)))
			fmt.Println(styles.MutedStyle.Render("Use --force to reinstall."))
			os.Exit(1)
		}

		opts := github.ExtractOptions{}
		if alreadyInstalled && installForce {
			opts.FinalDir = existing.Path
			opts.AllowReplace = true
		}

		// Do not remove the existing skill before download/extract. DownloadAndExtract
		// stages into a temp directory and swaps only after SKILL.md validates, so a
		// failed --force reinstall leaves the working install intact.

		fmt.Println()
		fmt.Printf("%s Installing %s\n", styles.SpinnerStyle.Render("⠋"), styles.CodeStyle.Render(skillName))
		fmt.Printf("  %s %s\n", styles.MutedStyle.Render("from"), info.FullURL)
		fmt.Println()

		installPath := ""
		// Download and install with spinner
		err = ui.RunWithSpinner("Downloading...", func() (string, error) {
			path, err := github.DownloadAndExtract(info, skillName, opts)
			if err != nil {
				return "", err
			}
			installPath = path

			// Get installed skill info
			s, _ := skill.Get(skillName)
			result := styles.RenderSuccess(fmt.Sprintf("Installed %s", styles.CodeStyle.Render(skillName)))
			if s != nil && s.Description != "" {
				result += "\n  " + styles.SkillDescStyle.Render(s.Description)
			}
			return result, nil
		})

		if err != nil {
			fmt.Println(styles.RenderError(err.Error()))
			os.Exit(1)
		}

		if installPath == "" {
			installPath = skill.GetSkillDir(skillName)
		}
		fmt.Println()
		fmt.Println(styles.MutedStyle.Render("  Skill installed to: ") + installPath)
		fmt.Println()
	},
}

func init() {
	installCmd.Flags().StringVarP(&installName, "name", "n", "", "Custom name for the skill")
	installCmd.Flags().BoolVarP(&installForce, "force", "f", false, "Force reinstall if already exists")
	rootCmd.AddCommand(installCmd)
}
