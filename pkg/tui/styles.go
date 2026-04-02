package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	Primary   = lipgloss.Color("#7C3AED") // Purple
	Secondary = lipgloss.Color("#06B6D4") // Cyan
	Success   = lipgloss.Color("#10B981") // Green
	Error     = lipgloss.Color("#EF4444") // Red
	Muted     = lipgloss.Color("#6B7280") // Gray

	// Text styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Secondary)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error)

	MutedStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// Layout
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(1, 2)

	// Status indicators
	CheckMark = SuccessStyle.Render("✓")
	CrossMark = ErrorStyle.Render("✗")
)
