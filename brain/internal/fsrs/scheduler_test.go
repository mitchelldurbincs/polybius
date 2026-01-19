// brain/internal/fsrs/scheduler_test.go
package fsrs

import (
	"testing"
	"time"
)

func TestNewCardScheduling(t *testing.T) {
	s := NewScheduler()

	// New card, first review with "Good" rating
	state := CardState{}
	newState, nextDue := s.Review(state, Good, time.Now())

	if newState.Reps != 1 {
		t.Errorf("Expected 1 rep, got %d", newState.Reps)
	}

	if nextDue.Before(time.Now()) {
		t.Error("Next due should be in the future")
	}

	t.Logf("After Good rating: stability=%.2f, difficulty=%.2f, next due=%v",
		newState.Stability, newState.Difficulty, nextDue)
}

func TestAgainResetsProgress(t *testing.T) {
	s := NewScheduler()

	// Simulate a card in Review state (state=2) that's been reviewed a few times
	state := CardState{
		Stability:  10.0,
		Difficulty: 5.0,
		Reps:       5,
		State:      2, // Review state - lapses only count here
	}

	newState, _ := s.Review(state, Again, time.Now())

	if newState.Lapses != 1 {
		t.Errorf("Expected 1 lapse after Again on Review card, got %d", newState.Lapses)
	}

	// Stability should decrease significantly
	if newState.Stability >= state.Stability {
		t.Error("Stability should decrease after Again rating")
	}
}
