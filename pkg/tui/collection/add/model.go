package add

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-oci/pkg/oci"
	"github.com/salaboy/skills-oci/pkg/skill"
	"github.com/salaboy/skills-oci/pkg/tui"
	"github.com/salaboy/skills-oci/pkg/tui/components"
)

type phase int

const (
	phaseFetching phase = iota
	phaseInstalling
	phaseDone
	phaseError
)

type collectionAddResultMsg struct {
	collection *skill.FetchCollectionResult
	installed  int
}
type collectionAddErrMsg struct{ err error }

// Model is the Bubble Tea model for the collection add workflow.
type Model struct {
	phase      phase
	spinner    spinner.Model
	ref        string
	outputDir  string
	projectDir string
	skillsDir  string
	plainHTTP  bool
	collection *skill.FetchCollectionResult
	installed  int
	err        error
}

// NewModel creates a new collection add TUI model.
func NewModel(ref, outputDir, projectDir, skillsDir string, plainHTTP bool) Model {
	if projectDir == "" {
		projectDir = "."
	}
	return Model{
		phase:      phaseFetching,
		spinner:    components.NewSpinner(),
		ref:        ref,
		outputDir:  outputDir,
		projectDir: projectDir,
		skillsDir:  skillsDir,
		plainHTTP:  plainHTTP,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startAdd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case collectionAddResultMsg:
		m.phase = phaseDone
		m.collection = msg.collection
		m.installed = msg.installed
		return m, tea.Quit

	case collectionAddErrMsg:
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
	b.WriteString(tui.TitleStyle.Render("  Skills OCI — Collection Add"))
	b.WriteString("\n\n")

	phases := []struct {
		name  string
		phase phase
	}{
		{"Fetching collection index", phaseFetching},
		{"Installing skills", phaseInstalling},
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

	if m.phase == phaseDone && m.collection != nil {
		b.WriteString("\n")
		b.WriteString(tui.SuccessStyle.Render(fmt.Sprintf("  ✓ Installed %d skill(s) from collection %q", m.installed, m.collection.Name)))
		b.WriteString("\n")
		for _, s := range m.collection.Skills {
			name := s.Name
			if name == "" {
				name = s.Ref
			}
			b.WriteString(fmt.Sprintf("    %s %s\n", tui.CheckMark, name))
		}
	}

	if m.phase == phaseError && m.err != nil {
		b.WriteString("\n")
		b.WriteString(tui.ErrorStyle.Render(fmt.Sprintf("  ✗ Failed: %s", m.err.Error())))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

// Err returns the error if the operation failed.
func (m Model) Err() error {
	return m.err
}

func (m Model) startAdd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		collection, err := oci.FetchCollection(ctx, oci.FetchCollectionOptions{
			Reference: m.ref,
			PlainHTTP: m.plainHTTP,
			OnStatus:  func(phase string) {},
		})
		if err != nil {
			return collectionAddErrMsg{err: err}
		}

		installed := 0
		for _, s := range collection.Skills {
			skillRef := s.Ref
			if skillRef == "" {
				// Fall back to digest-pinned ref if original ref is not annotated.
				skillRef = s.Digest
			}

			result, err := oci.Pull(ctx, oci.PullOptions{
				Reference: skillRef,
				OutputDir: m.outputDir,
				PlainHTTP: m.plainHTTP,
				OnStatus:  func(phase string) {},
			})
			if err != nil {
				return collectionAddErrMsg{err: fmt.Errorf("installing skill %q: %w", skillRef, err)}
			}

			if err := updateManifest(m.projectDir, m.skillsDir, result); err != nil {
				return collectionAddErrMsg{err: fmt.Errorf("updating skills.json for %s: %w", result.Name, err)}
			}
			if err := updateLockFile(m.projectDir, m.skillsDir, result); err != nil {
				return collectionAddErrMsg{err: fmt.Errorf("updating skills.lock.json for %s: %w", result.Name, err)}
			}
			installed++
		}

		return collectionAddResultMsg{collection: collection, installed: installed}
	}
}

func updateManifest(projectDir, skillsDir string, result *oci.PullResult) error {
	m, err := skill.LoadManifest(projectDir)
	if err != nil {
		return err
	}
	skill.AddToManifest(m, result.Name, result.Source(), result.Version)
	return skill.SaveManifest(projectDir, m)
}

func updateLockFile(projectDir, skillsDir string, result *oci.PullResult) error {
	l, err := skill.LoadLock(projectDir)
	if err != nil {
		return err
	}

	extractPath := filepath.Join(skillsDir, result.Name)
	entry := skill.LockedSkill{
		Name: result.Name,
		Path: extractPath,
		Source: skill.LockSource{
			Registry:   result.Registry,
			Repository: result.Repository,
			Tag:        result.Tag,
			Digest:     result.Digest,
			Ref:        result.FullRef(),
		},
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}

	skill.AddToLock(l, entry)
	return skill.SaveLock(projectDir, l)
}
