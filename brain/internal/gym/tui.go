// brain/internal/gym/tui.go
package gym

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Width(60)

	targetStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	hiddenStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	correctStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	incorrectStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
)

type ReviewCard struct {
	ID         int64
	Sentence   string
	TargetWord string
	Pinyin     string
	Definition string
	AudioFile  string
	ImageFile  string
}

type Model struct {
	cards      []*ReviewCard
	currentIdx int
	revealed   bool
	quitting   bool
	completed  int
	ratings    []int // Track ratings for summary
	onRate     func(cardID int64, rating int) error
	imageWin   *ImageWindow
}

func NewModel(cards []*ReviewCard, onRate func(cardID int64, rating int) error) Model {
	return Model{
		cards:    cards,
		onRate:   onRate,
		ratings:  make([]int, 0),
		imageWin: NewImageWindow(),
	}
}

func (m Model) Init() tea.Cmd {
	// Show first card's image if available
	if len(m.cards) > 0 && m.cards[0].ImageFile != "" {
		m.imageWin.Show(m.cards[0].ImageFile)
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case " ": // Space to reveal
			if !m.revealed && m.currentIdx < len(m.cards) {
				m.revealed = true
			}
			return m, nil

		case "1", "2", "3", "4":
			if m.revealed && m.currentIdx < len(m.cards) {
				rating := int(msg.String()[0] - '0')
				card := m.cards[m.currentIdx]

				// Submit rating
				if m.onRate != nil {
					m.onRate(card.ID, rating)
				}

				m.ratings = append(m.ratings, rating)
				m.completed++
				m.currentIdx++
				m.revealed = false

				// Show next card's image
				if m.currentIdx < len(m.cards) {
					nextCard := m.cards[m.currentIdx]
					if nextCard.ImageFile != "" {
						m.imageWin.Show(nextCard.ImageFile)
					}
				}

				if m.currentIdx >= len(m.cards) {
					m.quitting = true
					return m, tea.Quit
				}
			}
			return m, nil

		case "r": // Replay audio
			if m.currentIdx < len(m.cards) {
				// Audio playback will be handled by caller
			}
			return m, nil
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return m.summaryView()
	}

	if len(m.cards) == 0 {
		return "No cards due for review.\n\nPress q to quit.\n"
	}

	card := m.cards[m.currentIdx]

	// Header
	header := titleStyle.Render(fmt.Sprintf("POLYBIUS GYM    Card %d/%d", m.currentIdx+1, len(m.cards)))

	// Card content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("Sentence: %s\n\n", highlightWord(card.Sentence, card.TargetWord)))
	content.WriteString(fmt.Sprintf("Target: %s\n", targetStyle.Render(card.TargetWord)))

	if m.revealed {
		content.WriteString(fmt.Sprintf("Pinyin: %s\n", card.Pinyin))
		content.WriteString(fmt.Sprintf("Meaning: %s\n", card.Definition))
	} else {
		content.WriteString(fmt.Sprintf("Pinyin: %s\n", hiddenStyle.Render("[press space to reveal]")))
		content.WriteString(fmt.Sprintf("Meaning: %s\n", hiddenStyle.Render("[press space to reveal]")))
	}

	cardView := cardStyle.Render(content.String())

	// Help
	var help string
	if m.revealed {
		help = helpStyle.Render("[1] Again  [2] Hard  [3] Good  [4] Easy    [R] Replay  [Q] Quit")
	} else {
		help = helpStyle.Render("[Space] Reveal    [R] Replay  [Q] Quit")
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s\n", header, cardView, help)
}

func (m Model) summaryView() string {
	if m.completed == 0 {
		return "Session ended. No cards reviewed.\n"
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("SESSION COMPLETE"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Cards reviewed: %d\n\n", m.completed))

	// Count ratings
	counts := make(map[int]int)
	for _, r := range m.ratings {
		counts[r]++
	}

	if counts[1] > 0 {
		sb.WriteString(incorrectStyle.Render(fmt.Sprintf("  Again: %d\n", counts[1])))
	}
	if counts[2] > 0 {
		sb.WriteString(fmt.Sprintf("  Hard:  %d\n", counts[2]))
	}
	if counts[3] > 0 {
		sb.WriteString(correctStyle.Render(fmt.Sprintf("  Good:  %d\n", counts[3])))
	}
	if counts[4] > 0 {
		sb.WriteString(correctStyle.Render(fmt.Sprintf("  Easy:  %d\n", counts[4])))
	}

	sb.WriteString("\nGreat work! See you next time.\n")
	return sb.String()
}

func highlightWord(sentence, word string) string {
	// Simple highlight by wrapping the target word
	return strings.ReplaceAll(sentence, word, targetStyle.Render(word))
}
