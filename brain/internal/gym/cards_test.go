// brain/internal/gym/cards_test.go
package gym

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCardsModelView(t *testing.T) {
	now := time.Now()
	cards := []*CardListItem{
		{
			ID:         1,
			TargetWord: "超市",
			Sentence:   "我今天去了超市",
			DueDate:    timePtr(now.Add(3 * time.Hour)),
			Stability:  15.0,
			Difficulty: 0.2,
			Reps:       4,
			State:      "review",
		},
		{
			ID:         2,
			TargetWord: "电影院",
			Sentence:   "我们去电影院看电影",
			DueDate:    timePtr(now.Add(24 * time.Hour)),
			Stability:  5.0,
			Difficulty: 0.5,
			Reps:       2,
			State:      "learning",
		},
	}

	model := NewCardsModel(cards)
	view := model.View()

	if !strings.Contains(view, "超市") {
		t.Error("View should show first card's target word")
	}
	if !strings.Contains(view, "电影院") {
		t.Error("View should show second card's target word")
	}
	if !strings.Contains(view, "Stability:") {
		t.Error("View should show stability")
	}
	if !strings.Contains(view, "Difficulty:") {
		t.Error("View should show difficulty")
	}
}

func TestCardsModelNavigation(t *testing.T) {
	cards := []*CardListItem{
		{ID: 1, TargetWord: "word1"},
		{ID: 2, TargetWord: "word2"},
		{ID: 3, TargetWord: "word3"},
	}

	model := NewCardsModel(cards)

	// Move down
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	newModel, _ := model.Update(msg)
	cm := newModel.(CardsModel)
	if cm.cursor != 1 {
		t.Errorf("After j, cursor = %d, want 1", cm.cursor)
	}

	// Move up
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	newModel, _ = cm.Update(msg)
	cm = newModel.(CardsModel)
	if cm.cursor != 0 {
		t.Errorf("After k, cursor = %d, want 0", cm.cursor)
	}
}

func TestCardsModelEscape(t *testing.T) {
	cards := []*CardListItem{{ID: 1, TargetWord: "word1"}}
	model := NewCardsModel(cards)

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, cmd := model.Update(msg)
	cm := newModel.(CardsModel)

	if !cm.goBack {
		t.Error("Escape should set goBack to true")
	}
	if cmd == nil {
		t.Error("Escape should return tea.Quit command")
	}
}

func TestCardsModelEmpty(t *testing.T) {
	model := NewCardsModel(nil)
	view := model.View()

	if !strings.Contains(view, "No cards") {
		t.Error("Empty state should show 'No cards' message")
	}
}
