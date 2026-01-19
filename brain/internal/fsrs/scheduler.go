// brain/internal/fsrs/scheduler.go
package fsrs

import (
	"time"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v3"
)

// Rating constants matching FSRS
const (
	Again = 1
	Hard  = 2
	Good  = 3
	Easy  = 4
)

// CardState represents the FSRS state of a card
type CardState struct {
	Stability  float64
	Difficulty float64
	Reps       int
	Lapses     int
	State      int // 0=New, 1=Learning, 2=Review, 3=Relearning
	LastReview time.Time
}

type Scheduler struct {
	params gofsrs.Parameters
}

func NewScheduler() *Scheduler {
	// Use default FSRS-6 parameters
	return &Scheduler{
		params: gofsrs.DefaultParam(),
	}
}

func NewSchedulerWithParams(params gofsrs.Parameters) *Scheduler {
	return &Scheduler{params: params}
}

func (s *Scheduler) Review(state CardState, rating int, now time.Time) (CardState, time.Time) {
	// Convert to go-fsrs Card
	card := gofsrs.Card{
		Due:        now,
		Stability:  state.Stability,
		Difficulty: state.Difficulty,
		Reps:       uint64(state.Reps),
		Lapses:     uint64(state.Lapses),
		State:      gofsrs.State(state.State),
		LastReview: state.LastReview,
	}

	// Create FSRS instance and get scheduling info
	f := gofsrs.NewFSRS(s.params)
	schedulingInfo := f.Repeat(card, now)

	// Get the result for the given rating
	result := schedulingInfo[gofsrs.Rating(rating)]

	return CardState{
		Stability:  result.Card.Stability,
		Difficulty: result.Card.Difficulty,
		Reps:       int(result.Card.Reps),
		Lapses:     int(result.Card.Lapses),
		State:      int(result.Card.State),
		LastReview: now,
	}, result.Card.Due
}

// GetAllSchedules returns the next due dates for all possible ratings
func (s *Scheduler) GetAllSchedules(state CardState, now time.Time) map[int]time.Time {
	card := gofsrs.Card{
		Due:        now,
		Stability:  state.Stability,
		Difficulty: state.Difficulty,
		Reps:       uint64(state.Reps),
		Lapses:     uint64(state.Lapses),
		State:      gofsrs.State(state.State),
		LastReview: state.LastReview,
	}

	f := gofsrs.NewFSRS(s.params)
	schedulingInfo := f.Repeat(card, now)

	result := make(map[int]time.Time)
	for rating := Again; rating <= Easy; rating++ {
		result[rating] = schedulingInfo[gofsrs.Rating(rating)].Card.Due
	}

	return result
}
