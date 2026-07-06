package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/majiayu000/claude-skill-manager/internal/skill"
	"github.com/majiayu000/claude-skill-manager/pkg/styles"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:     "info <skill-name>",
	Aliases: []string{"show", "view"},
	Short:   "Show skill details",
	Long:    `Display detailed information about an installed skill.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		s, err := skill.Get(name)
		if err != nil {
			fmt.Println(styles.RenderError("Failed to get skill: " + err.Error()))
			os.Exit(1)
		}
		if s == nil {
			fmt.Println(styles.RenderError(fmt.Sprintf("Skill '%s' is not installed.", name)))
			os.Exit(1)
		}

		fmt.Println()
		fmt.Println(styles.TitleStyle.Render(styles.IconPackage + " " + s.Name))
		fmt.Println()

		// Description
		if s.Description != "" {
			fmt.Println(styles.SkillDescStyle.Render(s.Description))
			fmt.Println()
		}

		// Details
		fmt.Println(styles.TableHeaderStyle.Render("Details"))
		fmt.Println()

		fmt.Printf("  %s  %s\n",
			styles.MutedStyle.Render("Path:"),
			s.Path,
		)

		if s.Source != "" {
			fmt.Printf("  %s  %s\n",
				styles.MutedStyle.Render("Source:"),
				s.Source,
			)
		}

		if !s.InstalledAt.IsZero() {
			fmt.Printf("  %s  %s\n",
				styles.MutedStyle.Render("Installed:"),
				s.InstalledAt.Format("2006-01-02 15:04:05"),
			)
		}

		// List files
		fmt.Println()
		fmt.Println(styles.TableHeaderStyle.Render("Files"))
		fmt.Println()

		filepath.Walk(s.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(s.Path, path)
			if rel == "." {
				return nil
			}
			if info.IsDir() {
				fmt.Printf("  %s %s/\n", styles.IconFolder, rel)
			} else {
				fmt.Printf("  %s %s\n", styles.IconFile, rel)
			}
			return nil
		})

		fmt.Println()
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
