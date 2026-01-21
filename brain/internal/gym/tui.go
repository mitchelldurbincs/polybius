// brain/internal/gym/tui.go
package gym

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
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

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Italic(true)

	correctStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	incorrectStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
)

// RevealState tracks the 3-state reveal progression for audio-first learning
type RevealState int

const (
	StateAudioOnly     RevealState = iota // Screenshot + audio, no text
	StateHanziRevealed                    // + Hanzi sentence visible
	StateFullRevealed                     // + Pinyin + Definition
)

// tickMsg is sent periodically to update the audio progress bar
type tickMsg time.Time

// tickInterval controls how often the progress bar updates
const tickInterval = 100 * time.Millisecond

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
	cards       []*ReviewCard
	currentIdx  int
	revealState RevealState
	quitting    bool
	completed   int
	ratings     []int // Track ratings for summary
	onRate      func(cardID int64, rating int) error
	imageWin    *ImageWindow
	audioPlayer *AudioPlayer
	progress    progress.Model
}

func NewModel(cards []*ReviewCard, onRate func(cardID int64, rating int) error) Model {
	// Create progress bar with a nice gradient
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)
	return Model{
		cards:       cards,
		revealState: StateAudioOnly,
		onRate:      onRate,
		ratings:     make([]int, 0),
		imageWin:    NewImageWindow(),
		audioPlayer: NewAudioPlayer(),
		progress:    prog,
	}
}

func (m Model) Init() tea.Cmd {
	// Play audio for first card if available
	if len(m.cards) > 0 {
		card := m.cards[0]
		if card.AudioFile != "" {
			m.audioPlayer.Play(card.AudioFile)
		}
	}
	// Start the tick for progress bar updates
	return tickCmd()
}

// tickCmd returns a command that sends a tick message after the interval
func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		// Continue ticking if not quitting
		if m.quitting {
			return m, nil
		}
		return m, tickCmd()

	case progress.FrameMsg:
		// Handle progress bar animation frames
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.audioPlayer.Close()
			m.quitting = true
			return m, tea.Quit

		case " ": // Space advances reveal state
			if m.currentIdx < len(m.cards) {
				switch m.revealState {
				case StateAudioOnly:
					m.revealState = StateHanziRevealed
				case StateHanziRevealed:
					m.revealState = StateFullRevealed
				case StateFullRevealed:
					// Already fully revealed, space does nothing
				}
			}
			return m, nil

		case "1", "2", "3", "4":
			// Can only rate after at least seeing Hanzi
			if m.revealState >= StateHanziRevealed && m.currentIdx < len(m.cards) {
				rating := int(msg.String()[0] - '0')
				card := m.cards[m.currentIdx]

				// Submit rating
				if m.onRate != nil {
					m.onRate(card.ID, rating)
				}

				m.ratings = append(m.ratings, rating)
				m.completed++
				m.currentIdx++
				m.revealState = StateAudioOnly // Reset for next card

				// Play audio for next card
				if m.currentIdx < len(m.cards) {
					nextCard := m.cards[m.currentIdx]
					if nextCard.AudioFile != "" {
						m.audioPlayer.Play(nextCard.AudioFile)
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
				card := m.cards[m.currentIdx]
				if card.AudioFile != "" {
					m.audioPlayer.Play(card.AudioFile)
				}
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

	// Card content varies by reveal state
	var content strings.Builder

	switch m.revealState {
	case StateAudioOnly:
		// Only show that audio is playing, no text - train listening!
		content.WriteString("Listen to the audio...\n\n")
		content.WriteString(m.renderAudioProgress())
		content.WriteString("\n\n")
		content.WriteString(hiddenStyle.Render("[Press Space to reveal what was said]"))

	case StateHanziRevealed:
		// Show the sentence with target highlighted - can self-grade now
		content.WriteString(fmt.Sprintf("Sentence: %s\n\n", highlightWord(card.Sentence, card.TargetWord)))
		content.WriteString(fmt.Sprintf("Target: %s\n", targetStyle.Render(card.TargetWord)))
		content.WriteString(fmt.Sprintf("Pinyin: %s\n", hiddenStyle.Render("[hidden]")))
		content.WriteString(fmt.Sprintf("Meaning: %s\n\n", hiddenStyle.Render("[hidden]")))
		content.WriteString(m.renderAudioProgress())
		content.WriteString("\n\n")
		content.WriteString(promptStyle.Render("Did you hear it correctly? Rate now or [Space] for meaning"))

	case StateFullRevealed:
		// Show everything
		content.WriteString(fmt.Sprintf("Sentence: %s\n\n", highlightWord(card.Sentence, card.TargetWord)))
		content.WriteString(fmt.Sprintf("Target: %s\n", targetStyle.Render(card.TargetWord)))
		content.WriteString(fmt.Sprintf("Pinyin: %s\n", card.Pinyin))
		content.WriteString(fmt.Sprintf("Meaning: %s\n\n", card.Definition))
		content.WriteString(m.renderAudioProgress())
	}

	cardView := cardStyle.Render(content.String())

	// Help text changes based on state
	var help string
	switch m.revealState {
	case StateAudioOnly:
		help = helpStyle.Render("[Space] Reveal Hanzi    [R] Replay    [Q] Quit")
	case StateHanziRevealed:
		help = helpStyle.Render("[1-4] Rate    [Space] Show Meaning    [R] Replay    [Q] Quit")
	case StateFullRevealed:
		help = helpStyle.Render("[1] Again  [2] Hard  [3] Good  [4] Easy    [R] Replay    [Q] Quit")
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

// renderAudioProgress renders the audio progress bar with time display
func (m Model) renderAudioProgress() string {
	if m.audioPlayer.HasError() {
		return "Audio: [Unavailable]"
	}

	duration := m.audioPlayer.Duration()
	if duration == 0 {
		return "Audio: [Loading...]"
	}

	position := m.audioPlayer.Position()
	progress := m.audioPlayer.Progress()
	isPlaying := m.audioPlayer.IsPlaying()

	// Format times as seconds with one decimal
	posSeconds := position.Seconds()
	durSeconds := duration.Seconds()
	timeStr := fmt.Sprintf("%.1fs / %.1fs", posSeconds, durSeconds)

	// Build the progress line
	var status string
	if !isPlaying && progress >= 1.0 {
		status = hiddenStyle.Render("[Finished - R to replay]")
	} else if isPlaying {
		status = ""
	} else {
		status = hiddenStyle.Render("[Stopped]")
	}

	// Use ViewAs for static rendering (no animation needed since we're polling)
	progressBar := m.progress.ViewAs(progress)

	if status != "" {
		return fmt.Sprintf("%s %s  %s", progressBar, timeStr, status)
	}
	return fmt.Sprintf("%s %s", progressBar, timeStr)
}

func highlightWord(sentence, word string) string {
	// Simple highlight by wrapping the target word
	return strings.ReplaceAll(sentence, word, targetStyle.Render(word))
}
