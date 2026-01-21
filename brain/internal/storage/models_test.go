// brain/internal/storage/models_test.go
package storage

import (
	"os"
	"testing"
	"time"
)

func TestInsertAndGetMoment(t *testing.T) {
	dbPath := "test_models.db"
	defer os.Remove(dbPath)

	db, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	moment := &Moment{
		Timestamp:      time.Now().Format(time.RFC3339),
		AudioFile:      "audio_123.wav",
		ScreenshotFile: "audio_123.png",
		RawText:        "你好世界",
		Status:         "pending",
	}

	id, err := db.InsertMoment(moment)
	if err != nil {
		t.Fatalf("Failed to insert moment: %v", err)
	}

	got, err := db.GetMoment(id)
	if err != nil {
		t.Fatalf("Failed to get moment: %v", err)
	}

	if got.RawText != moment.RawText {
		t.Errorf("RawText = %q, want %q", got.RawText, moment.RawText)
	}
}

func TestGetAllCards(t *testing.T) {
	dbPath := "test_getallcards.db"
	defer os.Remove(dbPath)

	db, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	momentID, err := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		RawText:   "test sentence",
		Status:    "processed",
	})
	if err != nil {
		t.Fatalf("Failed to insert moment: %v", err)
	}

	now := time.Now()
	future := now.Add(3 * time.Hour)

	// Insert cards with various states
	db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review", DueDate: &future, Stability: 10.0, Difficulty: 0.3})
	db.InsertCard(&Card{MomentID: momentID, TargetWord: "word2", State: "learning", Stability: 2.0, Difficulty: 0.5})
	db.InsertCard(&Card{MomentID: momentID, TargetWord: "word3", State: "draft"}) // Should be excluded

	cards, err := db.GetAllCards()
	if err != nil {
		t.Fatalf("GetAllCards failed: %v", err)
	}

	if len(cards) != 2 {
		t.Errorf("len(cards) = %d, want 2 (excluding drafts)", len(cards))
	}
}
