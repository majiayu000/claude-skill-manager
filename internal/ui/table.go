package ui

import (
	"fmt"
	"strings"

	"github.com/majiayu000/claude-skill-manager/internal/skill"
	"github.com/majiayu000/claude-skill-manager/pkg/styles"
)

// RenderSkillTable renders skills as a table.
func RenderSkillTable(skills []skill.Skill) string {
	if len(skills) == 0 {
		return styles.MutedStyle.Render("No skills installed yet.\n\nRun ") +
			styles.CodeStyle.Render("sk install <github-url>") +
			styles.MutedStyle.Render(" to install your first skill.")
	}

	var b strings.Builder

	header := fmt.Sprintf("  %-25s  %-50s", "NAME", "DESCRIPTION")
	b.WriteString(styles.TableHeaderStyle.Render(header))
	b.WriteString("\n")

	for _, s := range skills {
		name := s.Name
		if len(name) > 25 {
			name = name[:22] + "..."
		}

		desc := s.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		if desc == "" {
			desc = styles.MutedStyle.Render("(no description)")
		}

		row := fmt.Sprintf("  %-25s  %-50s",
			styles.SuccessStyle.Render(name),
			styles.SkillDescStyle.Render(desc),
		)
		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.MutedStyle.Render(fmt.Sprintf("  %d skill(s) installed", len(skills))))

	return b.String()
}
