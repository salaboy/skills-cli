package add

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-cli/pkg/oci"
	"github.com/salaboy/skills-cli/pkg/tui"
	"github.com/salaboy/skills-cli/pkg/tui/components"
)

type phase int

const (
	phaseInit phase = iota
	phaseResolving
	phasePulling
	phaseExtracting
	phaseDone
	phaseError
)

type pullResultMsg struct{ result *oci.PullResult }
type pullErrMsg struct{ err error }

// Model is the Bubble Tea model for the add/install workflow.
type Model struct {
	phase     phase
	spinner   spinner.Model
	ref       string
	outputDir string
	plainHTTP bool
	result    *oci.PullResult
	err       error
}

// NewModel creates a new add TUI model.
func NewModel(ref, outputDir string, plainHTTP bool) Model {
	return Model{
		phase:     phaseInit,
		spinner:   components.NewSpinner(),
		ref:       ref,
		outputDir: outputDir,
		plainHTTP: plainHTTP,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startPull(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case pullResultMsg:
		m.phase = phaseDone
		m.result = msg.result
		return m, tea.Quit

	case pullErrMsg:
		m.phase = phaseError
		m.err = msg.err
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(tui.TitleStyle.Render("  Skills CLI — Add"))
	b.WriteString("\n\n")

	phases := []struct {
		name  string
		phase phase
	}{
		{"Resolving reference", phaseResolving},
		{"Pulling artifact", phasePulling},
		{"Extracting skill", phaseExtracting},
	}

	for _, p := range phases {
		if m.phase > p.phase {
			b.WriteString(fmt.Sprintf("  %s %s\n", tui.CheckMark, p.name))
		} else if m.phase == p.phase {
			b.WriteString(fmt.Sprintf("  %s %s\n", m.spinner.View(), p.name))
		} else {
			b.WriteString(fmt.Sprintf("  %s %s\n", tui.MutedStyle.Render("○"), tui.MutedStyle.Render(p.name)))
		}
	}

	if m.phase == phaseDone && m.result != nil {
		b.WriteString("\n")
		b.WriteString(tui.SuccessStyle.Render("  ✓ Successfully installed!"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Name:       %s\n", m.result.Name))
		b.WriteString(fmt.Sprintf("  Version:    %s\n", m.result.Version))
		b.WriteString(fmt.Sprintf("  Digest:     %s\n", m.result.Digest))
		b.WriteString(fmt.Sprintf("  Extracted:  %s\n", m.result.ExtractTo))
	}

	if m.phase == phaseError && m.err != nil {
		b.WriteString("\n")
		b.WriteString(tui.ErrorStyle.Render(fmt.Sprintf("  ✗ Install failed: %s", m.err.Error())))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

// Err returns the error if the pull failed.
func (m Model) Err() error {
	return m.err
}

func (m Model) startPull() tea.Cmd {
	return func() tea.Msg {
		result, err := oci.Pull(context.Background(), oci.PullOptions{
			Reference: m.ref,
			OutputDir: m.outputDir,
			PlainHTTP: m.plainHTTP,
			OnStatus:  func(phase string) {},
		})
		if err != nil {
			return pullErrMsg{err: err}
		}
		return pullResultMsg{result: result}
	}
}
