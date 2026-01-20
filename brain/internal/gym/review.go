// brain/internal/gym/review.go
package gym

import (
	"time"

	"github.com/mitchelldurbin/polybius/brain/internal/fsrs"
	"github.com/mitchelldurbin/polybius/brain/internal/storage"
)

// CardStore abstracts database operations for review sessions
type CardStore interface {
	GetDueCards(limit int) ([]*storage.Card, error)
	GetMoment(id int64) (*storage.Moment, error)
	GetCard(id int64) (*storage.Card, error)
	UpdateCardAfterReview(id int64, update storage.CardUpdate) error
}

// ReviewScheduler abstracts spaced repetition calculations
type ReviewScheduler interface {
	Review(state fsrs.CardState, rating int, now time.Time) (fsrs.CardState, time.Time)
}

type Session struct {
	store     CardStore
	scheduler ReviewScheduler
}

func NewSession(db *storage.DB) *Session {
	return &Session{
		store:     db,
		scheduler: fsrs.NewScheduler(),
	}
}

func (s *Session) GetDueCards(limit int) ([]*ReviewCard, error) {
	cards, err := s.store.GetDueCards(limit)
	if err != nil {
		return nil, err
	}

	var reviewCards []*ReviewCard
	for _, c := range cards {
		moment, err := s.store.GetMoment(c.MomentID)
		if err != nil {
			continue
		}

		reviewCards = append(reviewCards, &ReviewCard{
			ID:         c.ID,
			Sentence:   moment.RawText,
			TargetWord: c.TargetWord,
			Pinyin:     c.TargetPinyin,
			Definition: c.TargetDefinition,
			AudioFile:  moment.AudioFile,
			ImageFile:  moment.ScreenshotFile,
		})
	}

	return reviewCards, nil
}

func (s *Session) SubmitRating(cardID int64, rating int) error {
	// Get current card state from database
	card, err := s.store.GetCard(cardID)
	if err != nil {
		return err
	}

	// Convert to FSRS state
	state := fsrs.CardState{
		Stability:  card.Stability,
		Difficulty: card.Difficulty,
		Reps:       card.Reps,
		Lapses:     card.Lapses,
		State:      cardStateToInt(card.State),
	}
	if card.LastReview != nil {
		state.LastReview = *card.LastReview
	}

	// Calculate new state
	now := time.Now()
	newState, nextDue := s.scheduler.Review(state, rating, now)

	// Update card in database
	return s.store.UpdateCardAfterReview(cardID, storage.CardUpdate{
		Stability:  newState.Stability,
		Difficulty: newState.Difficulty,
		Reps:       newState.Reps,
		Lapses:     newState.Lapses,
		State:      intToCardState(newState.State),
		DueDate:    nextDue,
		LastReview: now,
	})
}

func cardStateToInt(state string) int {
	switch state {
	case "new":
		return 0
	case "learning":
		return 1
	case "review":
		return 2
	case "relearning":
		return 3
	default:
		return 0
	}
}

func intToCardState(state int) string {
	switch state {
	case 0:
		return "new"
	case 1:
		return "learning"
	case 2:
		return "review"
	case 3:
		return "relearning"
	default:
		return "new"
	}
}
