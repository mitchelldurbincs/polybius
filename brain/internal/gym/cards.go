// brain/internal/gym/cards.go
package gym

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mitchelldurbin/polybius/brain/internal/fsrs"
)

var (
	cardsListTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205"))

	cardsListItemStyle = lipgloss.NewStyle().
				PaddingLeft(2)

	cardsListSelectedStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("229"))

	cardsListCursorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205"))

	cardsListWordStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("212"))

	cardsListSentenceStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	cardsListMetaStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))

	cardsListEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Italic(true)

	cardsListNavStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))

	cardsListNavKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Bold(true)
)

// CardListItem represents a card in the cards list view
type CardListItem struct {
	ID         int64
	TargetWord string
	Sentence   string
	DueDate    *time.Time
	Stability  float64
	Difficulty float64
	Reps       int
	State      string
}

// CardsModel is the Bubbletea model for the cards list view
type CardsModel struct {
	cards  []*CardListItem
	cursor int
	goBack bool
}

// NewCardsModel creates a new cards list model
func NewCardsModel(cards []*CardListItem) CardsModel {
	return CardsModel{
		cards:  cards,
		cursor: 0,
		goBack: false,
	}
}

// Init implements tea.Model
func (m CardsModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m CardsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.cards)-1 {
				m.cursor++
			}
		case "esc":
			m.goBack = true
			return m, tea.Quit
		case "q", "ctrl+c":
			m.goBack = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View implements tea.Model
func (m CardsModel) View() string {
	var sb strings.Builder

	// Title
	sb.WriteString(cardsListTitleStyle.Render("ALL CARDS"))
	sb.WriteString("\n\n")

	// Empty state
	if len(m.cards) == 0 {
		sb.WriteString(cardsListEmptyStyle.Render("No cards yet. Create some cards through triage first!"))
		sb.WriteString("\n\n")
		sb.WriteString(m.renderNavigation())
		return sb.String()
	}

	// Render each card
	for i, card := range m.cards {
		sb.WriteString(m.renderCardItem(i, card))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(m.renderNavigation())

	return sb.String()
}

func (m CardsModel) renderCardItem(index int, card *CardListItem) string {
	var sb strings.Builder

	// Cursor indicator
	cursor := "  "
	if index == m.cursor {
		cursor = cardsListCursorStyle.Render("> ")
	}
	sb.WriteString(cursor)

	// Target word
	sb.WriteString(cardsListWordStyle.Render(card.TargetWord))
	sb.WriteString("  ")

	// Sentence preview (truncated)
	sentence := truncateSentence(card.Sentence, 35)
	sb.WriteString(cardsListSentenceStyle.Render(sentence))
	sb.WriteString("\n")

	// Meta line (indented to align with content)
	meta := m.renderCardMeta(card)
	if index == m.cursor {
		sb.WriteString(cardsListSelectedStyle.Render("    " + meta))
	} else {
		sb.WriteString(cardsListItemStyle.Render("  " + meta))
	}

	return sb.String()
}

func (m CardsModel) renderCardMeta(card *CardListItem) string {
	var parts []string

	// Due time
	dueStr := "Not scheduled"
	if card.DueDate != nil {
		dueStr = "Due: " + fsrs.RelativeDue(*card.DueDate)
	}
	parts = append(parts, dueStr)

	// Stability
	stabilityLabel := fsrs.StabilityLabel(card.Stability)
	parts = append(parts, fmt.Sprintf("Stability: %s", stabilityLabel))

	// Difficulty
	difficultyLabel := fsrs.DifficultyLabel(card.Difficulty)
	parts = append(parts, fmt.Sprintf("Difficulty: %s", difficultyLabel))

	// Reviews count
	parts = append(parts, fmt.Sprintf("Reviews: %d", card.Reps))

	return cardsListMetaStyle.Render(strings.Join(parts, " | "))
}

func (m CardsModel) renderNavigation() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("%s/%s Navigate", cardsListNavKeyStyle.Render("j"), cardsListNavKeyStyle.Render("k")))
	parts = append(parts, fmt.Sprintf("%s Back", cardsListNavKeyStyle.Render("[esc]")))

	return cardsListNavStyle.Render(strings.Join(parts, "    "))
}

// GoBack returns true if the user pressed escape to go back
func (m CardsModel) GoBack() bool {
	return m.goBack
}

// truncateSentence truncates a sentence to maxLen characters, adding ellipsis if needed
func truncateSentence(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}
