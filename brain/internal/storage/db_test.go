// brain/internal/storage/db_test.go
package storage

import (
	"os"
	"testing"
)

func TestOpenDatabase(t *testing.T) {
	dbPath := "test_brain.db"
	defer os.Remove(dbPath)

	db, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	tables := []string{"moments", "cards", "vocabulary", "reviews"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s does not exist: %v", table, err)
		}
	}
}
