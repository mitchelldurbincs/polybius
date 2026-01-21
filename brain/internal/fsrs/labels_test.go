// brain/internal/fsrs/labels_test.go
package fsrs

import (
	"testing"
	"time"
)

func TestStabilityLabel(t *testing.T) {
	tests := []struct {
		stability float64
		want      string
	}{
		{0.5, "Fragile"},
		{2.0, "Building"},
		{10.0, "Solid"},
		{30.0, "Strong"},
		{100.0, "Strong"},
	}

	for _, tt := range tests {
		got := StabilityLabel(tt.stability)
		if got != tt.want {
			t.Errorf("StabilityLabel(%f) = %q, want %q", tt.stability, got, tt.want)
		}
	}
}

func TestDifficultyLabel(t *testing.T) {
	tests := []struct {
		difficulty float64
		want       string
	}{
		{0.1, "Easy"},
		{0.4, "Medium"},
		{0.7, "Hard"},
		{0.95, "Hard"},
	}

	for _, tt := range tests {
		got := DifficultyLabel(tt.difficulty)
		if got != tt.want {
			t.Errorf("DifficultyLabel(%f) = %q, want %q", tt.difficulty, got, tt.want)
		}
	}
}

func TestRelativeDue(t *testing.T) {
	now := time.Now()

	tests := []struct {
		due  time.Time
		want string
	}{
		{now.Add(-1 * time.Hour), "Now"},
		{now.Add(30 * time.Minute), "30m"},
		{now.Add(2 * time.Hour), "2h 0m"},
		{now.Add(25 * time.Hour), "Tomorrow"},
		{now.Add(3 * 24 * time.Hour), "3 days"},
		{now.Add(14 * 24 * time.Hour), "2 weeks"},
	}

	for _, tt := range tests {
		got := RelativeDue(tt.due)
		if got != tt.want {
			t.Errorf("RelativeDue(%v) = %q, want %q", tt.due.Sub(now), got, tt.want)
		}
	}
}

func TestStateLabel(t *testing.T) {
	tests := []struct {
		state int
		want  string
	}{
		{0, "New"},
		{1, "Learning"},
		{2, "Review"},
		{3, "Relearning"},
	}

	for _, tt := range tests {
		got := StateLabel(tt.state)
		if got != tt.want {
			t.Errorf("StateLabel(%d) = %q, want %q", tt.state, got, tt.want)
		}
	}
}
