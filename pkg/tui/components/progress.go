package components

import (
	"github.com/charmbracelet/bubbles/progress"
)

// NewProgress creates a styled progress bar for use in TUI models.
func NewProgress() progress.Model {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)
	p.FullColor = "#7C3AED"
	p.EmptyColor = "#3C3C3C"
	p.SetSpringOptions(1.0, 1.0)
	return p
}
