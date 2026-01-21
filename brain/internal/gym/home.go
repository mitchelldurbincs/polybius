// brain/internal/gym/home.go
package gym

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mitchelldurbin/polybius/brain/internal/fsrs"
)

// HomeAction represents actions that can be triggered from the home screen
type HomeAction int

const (
	ActionNone HomeAction = iota
	ActionReview
	ActionTriage
	ActionCards
	ActionQuit
)

var (
	homeTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Width(30)

	boxTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	statLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	statValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	navStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	navKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Bold(true)

	emptyStateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
)

// HomeStats contains all statistics displayed on the home screen
type HomeStats struct {
	DueNow        int
	DueToday      int
	TotalCards    int
	NextDue       *time.Time
	WordsLearned  int
	RetentionRate float64
	ReviewsToday  int
	Streak        int
	Last7Days     []int
	DraftCount    int
}

// HomeModel is the Bubbletea model for the home screen
type HomeModel struct {
	stats    *HomeStats
	action   HomeAction
	quitting bool
}

// NewHomeModel creates a new home screen model with the given stats
func NewHomeModel(stats *HomeStats) HomeModel {
	return HomeModel{
		stats:  stats,
		action: ActionNone,
	}
}

// Init implements tea.Model
func (m HomeModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m HomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.action = ActionReview
			return m, tea.Quit
		case "t":
			m.action = ActionTriage
			return m, tea.Quit
		case "c":
			m.action = ActionCards
			return m, tea.Quit
		case "q", "ctrl+c":
			m.action = ActionQuit
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View implements tea.Model
func (m HomeModel) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	var sb strings.Builder

	// Title
	sb.WriteString(homeTitleStyle.Render("POLYBIUS GYM"))
	sb.WriteString("\n\n")

	// Empty state check
	if m.stats.TotalCards == 0 && m.stats.DraftCount == 0 {
		sb.WriteString(emptyStateStyle.Render("No cards yet. Capture some moments with the Miner to get started!"))
		sb.WriteString("\n\n")
		sb.WriteString(m.renderNavigation())
		return sb.String()
	}

	// Reviews box
	reviewsBox := m.renderReviewsBox()

	// Progress box
	progressBox := m.renderProgressBox()

	// Render boxes side by side
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, reviewsBox, "  ", progressBox))
	sb.WriteString("\n\n")

	// Navigation
	sb.WriteString(m.renderNavigation())

	return sb.String()
}

func (m HomeModel) renderReviewsBox() string {
	var content strings.Builder

	content.WriteString(boxTitleStyle.Render("Reviews"))
	content.WriteString("\n")
	content.WriteString(m.renderStat("Due now", fmt.Sprintf("%d", m.stats.DueNow)))
	content.WriteString(m.renderStat("Due today", fmt.Sprintf("%d", m.stats.DueToday)))
	content.WriteString(m.renderStat("Total cards", fmt.Sprintf("%d", m.stats.TotalCards)))

	// Next review time
	nextReviewStr := "None"
	if m.stats.NextDue != nil {
		nextReviewStr = fsrs.RelativeDue(*m.stats.NextDue)
	}
	content.WriteString(m.renderStat("Next review", nextReviewStr))

	return boxStyle.Render(content.String())
}

func (m HomeModel) renderProgressBox() string {
	var content strings.Builder

	content.WriteString(boxTitleStyle.Render("Progress"))
	content.WriteString("\n")
	content.WriteString(m.renderStat("Words learned", fmt.Sprintf("%d", m.stats.WordsLearned)))
	content.WriteString(m.renderStat("Retention", fmt.Sprintf("%.0f%%", m.stats.RetentionRate*100)))
	content.WriteString(m.renderStat("Reviews today", fmt.Sprintf("%d", m.stats.ReviewsToday)))
	content.WriteString(m.renderStat("Streak", fmt.Sprintf("%d", m.stats.Streak)))

	// Sparkline for last 7 days
	if len(m.stats.Last7Days) > 0 {
		sparkline := m.renderSparkline(m.stats.Last7Days)
		content.WriteString(m.renderStat("Last 7 days", sparkline))
	}

	return boxStyle.Render(content.String())
}

func (m HomeModel) renderStat(label, value string) string {
	return fmt.Sprintf("%s: %s\n", statLabelStyle.Render(label), statValueStyle.Render(value))
}

func (m HomeModel) renderSparkline(values []int) string {
	if len(values) == 0 {
		return ""
	}

	// Find max for scaling
	max := 0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		max = 1
	}

	// Sparkline characters from low to high
	sparkChars := []rune{'_', '.', ',', '-', '~', ':', ';', '!', '|'}

	var result strings.Builder
	for _, v := range values {
		// Scale value to 0-8 range
		scaled := (v * 8) / max
		if scaled > 8 {
			scaled = 8
		}
		result.WriteRune(sparkChars[scaled])
	}

	return result.String()
}

func (m HomeModel) renderNavigation() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("%s Review", navKeyStyle.Render("[r]")))

	triageLabel := "Triage"
	if m.stats.DraftCount > 0 {
		triageLabel = fmt.Sprintf("Triage (%d drafts)", m.stats.DraftCount)
	}
	parts = append(parts, fmt.Sprintf("%s %s", navKeyStyle.Render("[t]"), triageLabel))

	parts = append(parts, fmt.Sprintf("%s Cards", navKeyStyle.Render("[c]")))
	parts = append(parts, fmt.Sprintf("%s Quit", navKeyStyle.Render("[q]")))

	return navStyle.Render(strings.Join(parts, "    "))
}

// Action returns the action selected by the user
func (m HomeModel) Action() HomeAction {
	return m.action
}
