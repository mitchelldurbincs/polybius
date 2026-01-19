// brain/internal/gym/review.go
package gym

import (
	"time"

	"github.com/mitchelldurbin/polybius/brain/internal/fsrs"
	"github.com/mitchelldurbin/polybius/brain/internal/storage"
)

type Session struct {
	db        *storage.DB
	scheduler *fsrs.Scheduler
}

func NewSession(db *storage.DB) *Session {
	return &Session{
		db:        db,
		scheduler: fsrs.NewScheduler(),
	}
}

func (s *Session) GetDueCards(limit int) ([]*ReviewCard, error) {
	cards, err := s.db.GetDueCards(limit)
	if err != nil {
		return nil, err
	}

	var reviewCards []*ReviewCard
	for _, c := range cards {
		moment, err := s.db.GetMoment(c.MomentID)
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
	card, err := s.db.GetCard(cardID)
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
	return s.db.UpdateCardAfterReview(cardID, storage.CardUpdate{
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
