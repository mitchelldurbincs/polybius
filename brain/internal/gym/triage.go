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

type MomentWithCards struct {
	Moment *storage.Moment
	Cards  []*storage.Card
}

type TriageModel struct {
	db           *storage.DB
	moments      []*MomentWithCards
	cursor       int
	expanded     map[int64]bool
	startReview  bool
	quitting     bool
	loadErrors   int // Track how many moments failed to load cards
	totalMoments int // Total moments attempted to load
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

// triageLoadResult carries loaded moments plus any error information
type triageLoadResult struct {
	moments      []*MomentWithCards
	loadErrors   int
	totalMoments int
}

func (m TriageModel) loadDrafts() tea.Msg {
	moments, err := m.db.GetDraftMoments()
	if err != nil {
		return err
	}

	result := triageLoadResult{
		totalMoments: len(moments),
	}

	for _, moment := range moments {
		cards, err := m.db.GetDraftCardsByMoment(moment.ID)
		if err != nil {
			log.Printf("Warning: failed to load cards for moment %d: %v", moment.ID, err)
			result.loadErrors++
			continue
		}
		result.moments = append(result.moments, &MomentWithCards{
			Moment: moment,
			Cards:  cards,
		})
	}

	return result
}

func (m TriageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case triageLoadResult:
		m.moments = msg.moments
		m.loadErrors = msg.loadErrors
		m.totalMoments = msg.totalMoments
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
				if err := m.db.ApproveMoment(moment.Moment.ID); err != nil {
					log.Printf("Failed to approve moment: %v", err)
				}
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
				if err := m.db.ApproveMoment(moment.Moment.ID); err != nil {
					log.Printf("Failed to approve moment: %v", err)
				}
			}
			m.moments = nil
			m.cursor = 0
			return m, nil

		case "d":
			// Delete current moment
			if m.cursor < len(m.moments) {
				moment := m.moments[m.cursor]
				if err := m.db.DeleteMoment(moment.Moment.ID); err != nil {
					log.Printf("Failed to delete moment: %v", err)
				}
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
	header := TitleStyle.Render("TRIAGE")
	stats := MutedStyle.Render(fmt.Sprintf("Draft: %d cards in %d moments", draftCount, len(m.moments)))
	sb.WriteString(fmt.Sprintf("%s    %s", header, stats))

	// Show load errors if any
	if m.loadErrors > 0 {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		sb.WriteString("    ")
		sb.WriteString(errorStyle.Render(fmt.Sprintf("(%d failed to load)", m.loadErrors)))
	}
	sb.WriteString("\n\n")

	if len(m.moments) == 0 {
		sb.WriteString("No drafts to triage. All captures have been processed.\n\n")
		sb.WriteString(MutedStyle.Render("[R] Start Review    [Q] Quit"))
		return sb.String()
	}

	// List moments
	for i, mwc := range m.moments {
		prefix := "  "
		style := TextStyle
		if i == m.cursor {
			prefix = "> "
			style = AccentStyle
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
				sb.WriteString(NestedItemStyle.Render(cardLine))
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(MutedStyle.Render("[↑↓] Navigate  [Enter] Expand  [A] Approve  [D] Delete  [R] Review  [Q] Quit"))

	return sb.String()
}

func (m TriageModel) StartReview() bool {
	return m.startReview
}
