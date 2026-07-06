package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/majiayu000/claude-skill-manager/pkg/styles"
	"golang.org/x/term"
)

// SpinnerModel is a simple spinner for async operations
type SpinnerModel struct {
	spinner  spinner.Model
	message  string
	done     bool
	err      error
	result   string
	quitting bool
}

// NewSpinner creates a new spinner with a message
func NewSpinner(message string) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Primary)

	return SpinnerModel{
		spinner: s,
		message: message,
	}
}

func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case DoneMsg:
		m.done = true
		m.result = msg.Result
		m.err = msg.Err
		return m, tea.Quit
	}

	return m, nil
}

func (m SpinnerModel) View() string {
	if m.quitting {
		return ""
	}

	if m.done {
		if m.err != nil {
			return styles.RenderError(m.err.Error()) + "\n"
		}
		return m.result + "\n"
	}

	return fmt.Sprintf("%s %s\n", m.spinner.View(), m.message)
}

// DoneMsg signals the spinner to stop
type DoneMsg struct {
	Result string
	Err    error
}

// RunWithSpinner runs a function with a spinner
func RunWithSpinner(message string, fn func() (string, error)) error {
	// Check if we're in a TTY
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		// Non-TTY mode: simple output
		fmt.Printf("  %s %s\n", styles.SpinnerStyle.Render("⠋"), message)
		result, err := fn()
		if err != nil {
			fmt.Println(styles.RenderError(err.Error()))
			return err
		}
		fmt.Println(result)
		return nil
	}

	m := NewSpinner(message)
	p := tea.NewProgram(m)

	go func() {
		result, err := fn()
		p.Send(DoneMsg{Result: result, Err: err})
	}()

	_, err := p.Run()
	return err
}
