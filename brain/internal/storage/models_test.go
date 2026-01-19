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
