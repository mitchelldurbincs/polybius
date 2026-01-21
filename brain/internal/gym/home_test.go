// brain/internal/gym/home_test.go
package gym

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHomeModelView(t *testing.T) {
	stats := &HomeStats{
		DueNow:        0,
		DueToday:      1,
		TotalCards:    5,
		NextDue:       timePtr(time.Now().Add(3 * time.Hour)),
		WordsLearned:  12,
		RetentionRate: 0.87,
		ReviewsToday:  3,
		Streak:        5,
		Last7Days:     []int{1, 3, 5, 2, 4, 1, 3},
		DraftCount:    2,
	}

	model := NewHomeModel(stats)
	view := model.View()

	// Check that key elements are present
	if !strings.Contains(view, "Due now: 0") {
		t.Error("View should show due now count")
	}
	if !strings.Contains(view, "Due today: 1") {
		t.Error("View should show due today count")
	}
	if !strings.Contains(view, "Total cards: 5") {
		t.Error("View should show total cards")
	}
	if !strings.Contains(view, "Words learned: 12") {
		t.Error("View should show words learned")
	}
	if !strings.Contains(view, "87%") {
		t.Error("View should show retention rate")
	}
	if !strings.Contains(view, "Streak: 5") {
		t.Error("View should show streak")
	}
	if !strings.Contains(view, "[r]") {
		t.Error("View should show review shortcut")
	}
	if !strings.Contains(view, "[t]") {
		t.Error("View should show triage shortcut")
	}
	if !strings.Contains(view, "(2 drafts)") {
		t.Error("View should show draft count")
	}
}

func TestHomeModelKeyHandling(t *testing.T) {
	stats := &HomeStats{DraftCount: 2}
	model := NewHomeModel(stats)

	tests := []struct {
		key      string
		wantMode HomeAction
	}{
		{"r", ActionReview},
		{"t", ActionTriage},
		{"c", ActionCards},
		{"q", ActionQuit},
	}

	for _, tt := range tests {
		m := model
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
		newModel, _ := m.Update(msg)
		hm := newModel.(HomeModel)

		if hm.action != tt.wantMode {
			t.Errorf("Key %q: action = %v, want %v", tt.key, hm.action, tt.wantMode)
		}
	}
}

func TestHomeModelEmptyState(t *testing.T) {
	stats := &HomeStats{
		TotalCards: 0,
		DraftCount: 0,
	}

	model := NewHomeModel(stats)
	view := model.View()

	if !strings.Contains(view, "No cards yet") {
		t.Error("Empty state should show 'No cards yet' message")
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
