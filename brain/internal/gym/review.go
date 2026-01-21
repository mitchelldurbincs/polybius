// brain/internal/gym/review.go
package gym

import (
	"os"
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

// DueCardsResult contains the review cards and any skip information
type DueCardsResult struct {
	Cards          []*ReviewCard
	SkippedMoment  int // Cards skipped because moment couldn't be loaded
	SkippedMissing int // Cards skipped because audio file is missing
}

// fileExists checks if a file exists and is not a directory
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func (s *Session) GetDueCards(limit int) ([]*ReviewCard, error) {
	result, err := s.GetDueCardsWithInfo(limit)
	if err != nil {
		return nil, err
	}
	return result.Cards, nil
}

func (s *Session) GetDueCardsWithInfo(limit int) (*DueCardsResult, error) {
	cards, err := s.store.GetDueCards(limit)
	if err != nil {
		return nil, err
	}

	result := &DueCardsResult{}
	for _, c := range cards {
		moment, err := s.store.GetMoment(c.MomentID)
		if err != nil {
			result.SkippedMoment++
			continue
		}

		// Validate that audio file exists (required for review)
		if !fileExists(moment.AudioFile) {
			result.SkippedMissing++
			continue
		}

		result.Cards = append(result.Cards, &ReviewCard{
			ID:         c.ID,
			Sentence:   moment.RawText,
			TargetWord: c.TargetWord,
			Pinyin:     c.TargetPinyin,
			Definition: c.TargetDefinition,
			AudioFile:  moment.AudioFile,
			ImageFile:  moment.ScreenshotFile,
		})
	}

	return result, nil
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

// HomeStatsStore abstracts database operations for home screen stats
type HomeStatsStore interface {
	GetCardStats() (*storage.CardStats, error)
	GetNextDueTime() (*time.Time, error)
	GetReviewStats(days int) (*storage.ReviewStats, error)
	GetStreak() (int, error)
	GetReviewsPerDay(days int) ([]int, error)
	CountLearnedWords() (int, error)
	CountDraftCards() (int, error)
}

// LoadHomeStats loads all statistics needed for the home screen
func (s *Session) LoadHomeStats() (*HomeStats, error) {
	store := s.store.(HomeStatsStore)

	cardStats, err := store.GetCardStats()
	if err != nil {
		return nil, err
	}

	nextDue, err := store.GetNextDueTime()
	if err != nil {
		return nil, err
	}

	reviewStats, err := store.GetReviewStats(30)
	if err != nil {
		return nil, err
	}

	streak, err := store.GetStreak()
	if err != nil {
		return nil, err
	}

	last7Days, err := store.GetReviewsPerDay(7)
	if err != nil {
		return nil, err
	}

	wordsLearned, err := store.CountLearnedWords()
	if err != nil {
		return nil, err
	}

	draftCount, err := store.CountDraftCards()
	if err != nil {
		return nil, err
	}

	return &HomeStats{
		DueNow:        cardStats.DueNow,
		DueToday:      cardStats.DueToday,
		TotalCards:    cardStats.TotalCards,
		NextDue:       nextDue,
		WordsLearned:  wordsLearned,
		RetentionRate: reviewStats.RetentionRate,
		ReviewsToday:  reviewStats.ReviewsToday,
		Streak:        streak,
		Last7Days:     last7Days,
		DraftCount:    draftCount,
	}, nil
}

// GetAllCardsForList returns cards formatted for the cards list view
func (s *Session) GetAllCardsForList() ([]*CardListItem, error) {
	type allCardsStore interface {
		GetAllCards() ([]*storage.Card, error)
		GetMoment(id int64) (*storage.Moment, error)
	}

	store := s.store.(allCardsStore)

	cards, err := store.GetAllCards()
	if err != nil {
		return nil, err
	}

	var items []*CardListItem
	for _, c := range cards {
		moment, err := store.GetMoment(c.MomentID)
		sentence := ""
		if err == nil {
			sentence = moment.RawText
		}

		items = append(items, &CardListItem{
			ID:         c.ID,
			TargetWord: c.TargetWord,
			Sentence:   sentence,
			DueDate:    c.DueDate,
			Stability:  c.Stability,
			Difficulty: c.Difficulty,
			Reps:       c.Reps,
			State:      c.State,
		})
	}

	return items, nil
}
