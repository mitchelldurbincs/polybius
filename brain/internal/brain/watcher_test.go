// brain/internal/brain/watcher_test.go
package brain

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherDetectsNewFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "watcher_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	detected := make(chan string, 1)
	w, err := NewWatcher(tmpDir, func(path string) {
		detected <- path
	})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer w.Stop()

	go w.Start()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create a JSON file (simulating Miner output)
	testFile := filepath.Join(tmpDir, "audio_123456.json")
	if err := os.WriteFile(testFile, []byte(`{"test": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case path := <-detected:
		if filepath.Base(path) != "audio_123456.json" {
			t.Errorf("Expected audio_123456.json, got %s", filepath.Base(path))
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for file detection")
	}
}
