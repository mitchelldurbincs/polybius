// brain/internal/gym/styles.go
package gym

import "github.com/charmbracelet/lipgloss"

// Color palette - semantic naming for consistency
const (
	colorAccent    = lipgloss.Color("205") // Pink/magenta - titles, cursor
	colorHighlight = lipgloss.Color("212") // Light pink - target words, emphasis
	colorSelected  = lipgloss.Color("229") // Yellow - selected items, nav keys
	colorMuted     = lipgloss.Color("241") // Gray - meta, help, labels
	colorText      = lipgloss.Color("252") // Light gray - primary text content
)

// Text styles
var (
	// TitleStyle for screen headers (e.g., "POLYBIUS GYM", "TRIAGE", "ALL CARDS")
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	// AccentStyle for emphasized content (target words, box titles, selected moments)
	AccentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorHighlight)

	// TextStyle for primary content (sentences, values, moment text)
	TextStyle = lipgloss.NewStyle().
			Foreground(colorText)

	// MutedStyle for secondary content (meta info, labels, stats, help text)
	MutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// EmptyStyle for empty state messages
	EmptyStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)
)

// Navigation styles
var (
	// NavStyle for navigation bar container
	NavStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// NavKeyStyle for keyboard shortcut indicators
	NavKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSelected)
)

// List styles
var (
	// CursorStyle for list cursor indicator
	CursorStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	// ItemStyle for regular list items
	ItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	// SelectedItemStyle for selected list items
	SelectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(colorSelected)

	// NestedItemStyle for nested/child items in lists
	NestedItemStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			PaddingLeft(4)
)

// Box styles
var (
	// BoxStyle for bordered content boxes
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Width(30)
)
