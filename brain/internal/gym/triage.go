// brain/internal/gym/triage.go
package gym

import (
	"fmt"
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mitchelldurbin/polybius/brain/internal/storage"
)

var (
	triageTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205"))

	momentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	selectedMomentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")).
				Bold(true)

	cardItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(4)

	statsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	triageHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

type MomentWithCards struct {
	Moment *storage.Moment
	Cards  []*storage.Card
}

type TriageModel struct {
	db          *storage.DB
	moments     []*MomentWithCards
	cursor      int
	expanded    map[int64]bool
	startReview bool
	quitting    bool
}

func NewTriageModel(db *storage.DB) TriageModel {
	return TriageModel{
		db:       db,
		expanded: make(map[int64]bool),
	}
}

func (m TriageModel) Init() tea.Cmd {
	return m.loadDrafts
}

func (m TriageModel) loadDrafts() tea.Msg {
	moments, err := m.db.GetDraftMoments()
	if err != nil {
		return err
	}

	var momentsWithCards []*MomentWithCards
	for _, moment := range moments {
		cards, err := m.db.GetDraftCardsByMoment(moment.ID)
		if err != nil {
			log.Printf("Warning: failed to load cards for moment %d: %v", moment.ID, err)
			continue
		}
		momentsWithCards = append(momentsWithCards, &MomentWithCards{
			Moment: moment,
			Cards:  cards,
		})
	}

	return momentsWithCards
}

type loadedMsg []*MomentWithCards

func (m TriageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case []*MomentWithCards:
		m.moments = msg
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.cursor < len(m.moments)-1 {
				m.cursor++
			}
			return m, nil

		case "enter", " ":
			// Toggle expand/collapse
			if m.cursor < len(m.moments) {
				momentID := m.moments[m.cursor].Moment.ID
				m.expanded[momentID] = !m.expanded[momentID]
			}
			return m, nil

		case "a":
			// Approve current moment
			if m.cursor < len(m.moments) {
				moment := m.moments[m.cursor]
				m.db.ApproveMoment(moment.Moment.ID)
				// Remove from list
				m.moments = append(m.moments[:m.cursor], m.moments[m.cursor+1:]...)
				if m.cursor >= len(m.moments) && m.cursor > 0 {
					m.cursor--
				}
			}
			return m, nil

		case "A":
			// Approve ALL moments
			for _, moment := range m.moments {
				m.db.ApproveMoment(moment.Moment.ID)
			}
			m.moments = nil
			m.cursor = 0
			return m, nil

		case "d":
			// Delete current moment
			if m.cursor < len(m.moments) {
				moment := m.moments[m.cursor]
				m.db.DeleteMoment(moment.Moment.ID)
				// Remove from list
				m.moments = append(m.moments[:m.cursor], m.moments[m.cursor+1:]...)
				if m.cursor >= len(m.moments) && m.cursor > 0 {
					m.cursor--
				}
			}
			return m, nil

		case "r":
			// Start review (if there are approved cards)
			m.startReview = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m TriageModel) View() string {
	if m.quitting {
		return "Triage complete.\n"
	}

	var sb strings.Builder

	// Count stats
	draftCount := 0
	for _, mwc := range m.moments {
		draftCount += len(mwc.Cards)
	}

	// Header
	header := triageTitleStyle.Render("TRIAGE")
	stats := statsStyle.Render(fmt.Sprintf("Draft: %d cards in %d moments", draftCount, len(m.moments)))
	sb.WriteString(fmt.Sprintf("%s    %s\n\n", header, stats))

	if len(m.moments) == 0 {
		sb.WriteString("No drafts to triage. All captures have been processed.\n\n")
		sb.WriteString(triageHelpStyle.Render("[R] Start Review    [Q] Quit"))
		return sb.String()
	}

	// List moments
	for i, mwc := range m.moments {
		prefix := "  "
		style := momentStyle
		if i == m.cursor {
			prefix = "> "
			style = selectedMomentStyle
		}

		// Expand indicator
		expandChar := "▸"
		if m.expanded[mwc.Moment.ID] {
			expandChar = "▾"
		}

		// Truncate sentence for display
		sentence := mwc.Moment.RawText
		if len(sentence) > 30 {
			sentence = sentence[:30] + "..."
		}

		line := fmt.Sprintf("%s%s [%d] %s", prefix, expandChar, len(mwc.Cards), sentence)
		sb.WriteString(style.Render(line))
		sb.WriteString("\n")

		// Show cards if expanded
		if m.expanded[mwc.Moment.ID] {
			for _, card := range mwc.Cards {
				cardLine := fmt.Sprintf("└─ %s (%s)", card.TargetWord, card.TargetPinyin)
				sb.WriteString(cardItemStyle.Render(cardLine))
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(triageHelpStyle.Render("[↑↓] Navigate  [Enter] Expand  [A] Approve  [D] Delete  [R] Review  [Q] Quit"))

	return sb.String()
}

func (m TriageModel) StartReview() bool {
	return m.startReview
}
