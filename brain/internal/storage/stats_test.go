// brain/internal/storage/stats_test.go
package storage

import (
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	dbPath := "test_stats_" + t.Name() + ".db"
	t.Cleanup(func() { os.Remove(dbPath) })

	db, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestGetCardStats(t *testing.T) {
	db := setupTestDB(t)

	// Insert test moment
	momentID, _ := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		RawText:   "test",
		Status:    "processed",
	})

	// Insert cards with various states
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(3 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	// Card due now
	if _, err := db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review", DueDate: &past}); err != nil {
		t.Fatalf("Failed to insert card: %v", err)
	}
	// Card due later today
	if _, err := db.InsertCard(&Card{MomentID: momentID, TargetWord: "word2", State: "review", DueDate: &future}); err != nil {
		t.Fatalf("Failed to insert card: %v", err)
	}
	// Card due tomorrow
	if _, err := db.InsertCard(&Card{MomentID: momentID, TargetWord: "word3", State: "review", DueDate: &tomorrow}); err != nil {
		t.Fatalf("Failed to insert card: %v", err)
	}
	// Draft card (should not count)
	if _, err := db.InsertCard(&Card{MomentID: momentID, TargetWord: "word4", State: "draft"}); err != nil {
		t.Fatalf("Failed to insert card: %v", err)
	}

	stats, err := db.GetCardStats()
	if err != nil {
		t.Fatalf("GetCardStats failed: %v", err)
	}

	if stats.DueNow != 1 {
		t.Errorf("DueNow = %d, want 1", stats.DueNow)
	}
	if stats.DueToday != 2 { // includes DueNow
		t.Errorf("DueToday = %d, want 2", stats.DueToday)
	}
	if stats.TotalCards != 3 { // excludes drafts
		t.Errorf("TotalCards = %d, want 3", stats.TotalCards)
	}
}

func TestGetNextDueTime(t *testing.T) {
	db := setupTestDB(t)

	momentID, _ := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		Status:    "processed",
	})

	// No cards - should return nil
	nextDue, err := db.GetNextDueTime()
	if err != nil {
		t.Fatalf("GetNextDueTime failed: %v", err)
	}
	if nextDue != nil {
		t.Errorf("Expected nil for empty db, got %v", nextDue)
	}

	// Add a card due in the future
	future := time.Now().Add(3 * time.Hour)
	if _, err := db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review", DueDate: &future}); err != nil {
		t.Fatalf("Failed to insert card: %v", err)
	}

	nextDue, err = db.GetNextDueTime()
	if err != nil {
		t.Fatalf("GetNextDueTime failed: %v", err)
	}
	if nextDue == nil {
		t.Fatal("Expected non-nil next due time")
	}
	// Allow 1 second tolerance
	if nextDue.Sub(future).Abs() > time.Second {
		t.Errorf("NextDue = %v, want ~%v", nextDue, future)
	}
}

func TestGetReviewStats(t *testing.T) {
	db := setupTestDB(t)

	momentID, _ := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		Status:    "processed",
	})

	cardID, _ := db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review"})

	// Add some reviews
	if err := db.InsertReview(cardID, 3); err != nil { // Good
		t.Fatalf("Failed to insert review: %v", err)
	}
	if err := db.InsertReview(cardID, 4); err != nil { // Easy
		t.Fatalf("Failed to insert review: %v", err)
	}
	if err := db.InsertReview(cardID, 1); err != nil { // Again
		t.Fatalf("Failed to insert review: %v", err)
	}
	if err := db.InsertReview(cardID, 3); err != nil { // Good
		t.Fatalf("Failed to insert review: %v", err)
	}

	stats, err := db.GetReviewStats(30)
	if err != nil {
		t.Fatalf("GetReviewStats failed: %v", err)
	}

	if stats.TotalReviews != 4 {
		t.Errorf("TotalReviews = %d, want 4", stats.TotalReviews)
	}
	// Retention = (Good + Easy) / Total = 3/4 = 75%
	if stats.RetentionRate < 0.74 || stats.RetentionRate > 0.76 {
		t.Errorf("RetentionRate = %f, want ~0.75", stats.RetentionRate)
	}
}

func TestGetStreak(t *testing.T) {
	db := setupTestDB(t)

	momentID, _ := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		Status:    "processed",
	})

	cardID, _ := db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review"})

	// Reviews today
	if err := db.InsertReview(cardID, 3); err != nil {
		t.Fatalf("Failed to insert review: %v", err)
	}
	// Reviews yesterday (need to insert with specific time)
	if err := db.insertReviewAt(cardID, 3, time.Now().Add(-24*time.Hour)); err != nil {
		t.Fatalf("Failed to insert review: %v", err)
	}
	// Reviews 2 days ago
	if err := db.insertReviewAt(cardID, 3, time.Now().Add(-48*time.Hour)); err != nil {
		t.Fatalf("Failed to insert review: %v", err)
	}

	streak, err := db.GetStreak()
	if err != nil {
		t.Fatalf("GetStreak failed: %v", err)
	}

	if streak != 3 {
		t.Errorf("Streak = %d, want 3", streak)
	}
}

func TestGetStreak_NoReviewToday(t *testing.T) {
	db := setupTestDB(t)

	momentID, _ := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		Status:    "processed",
	})

	cardID, _ := db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review"})

	// NO review today - but reviewed yesterday and day before
	if err := db.insertReviewAt(cardID, 3, time.Now().Add(-24*time.Hour)); err != nil {
		t.Fatalf("Failed to insert review: %v", err)
	}
	if err := db.insertReviewAt(cardID, 3, time.Now().Add(-48*time.Hour)); err != nil {
		t.Fatalf("Failed to insert review: %v", err)
	}

	streak, err := db.GetStreak()
	if err != nil {
		t.Fatalf("GetStreak failed: %v", err)
	}

	// Streak should be 2 (yesterday + day before), not 0
	if streak != 2 {
		t.Errorf("Streak = %d, want 2 (streak should count from yesterday if no review today)", streak)
	}
}

func TestGetReviewsPerDay(t *testing.T) {
	db := setupTestDB(t)

	momentID, _ := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		Status:    "processed",
	})

	cardID, _ := db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review"})

	// Add reviews on different days
	if err := db.InsertReview(cardID, 3); err != nil {
		t.Fatalf("Failed to insert review: %v", err)
	}
	if err := db.InsertReview(cardID, 3); err != nil {
		t.Fatalf("Failed to insert review: %v", err)
	}
	if err := db.insertReviewAt(cardID, 3, time.Now().Add(-24*time.Hour)); err != nil {
		t.Fatalf("Failed to insert review: %v", err)
	}

	counts, err := db.GetReviewsPerDay(7)
	if err != nil {
		t.Fatalf("GetReviewsPerDay failed: %v", err)
	}

	if len(counts) != 7 {
		t.Errorf("len(counts) = %d, want 7", len(counts))
	}
	// Today should have 2 reviews
	if counts[6] != 2 {
		t.Errorf("Today's count = %d, want 2", counts[6])
	}
	// Yesterday should have 1
	if counts[5] != 1 {
		t.Errorf("Yesterday's count = %d, want 1", counts[5])
	}
}

func TestCountLearnedWords(t *testing.T) {
	db := setupTestDB(t)

	// Add known words
	if err := db.UpsertVocabulary(&Vocabulary{Word: "word1", Status: "known"}); err != nil {
		t.Fatalf("Failed to upsert vocabulary: %v", err)
	}
	if err := db.UpsertVocabulary(&Vocabulary{Word: "word2", Status: "known"}); err != nil {
		t.Fatalf("Failed to upsert vocabulary: %v", err)
	}
	if err := db.UpsertVocabulary(&Vocabulary{Word: "word3", Status: "unknown"}); err != nil {
		t.Fatalf("Failed to upsert vocabulary: %v", err)
	}

	count, err := db.CountLearnedWords()
	if err != nil {
		t.Fatalf("CountLearnedWords failed: %v", err)
	}

	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}
