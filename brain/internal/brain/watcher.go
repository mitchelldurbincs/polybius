// brain/internal/brain/watcher.go
package brain

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	dir       string
	watcher   *fsnotify.Watcher
	onNewFile func(path string)
	done      chan struct{}
}

func NewWatcher(dir string, onNewFile func(path string)) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, err
	}

	return &Watcher{
		dir:       dir,
		watcher:   watcher,
		onNewFile: onNewFile,
		done:      make(chan struct{}),
	}, nil
}

func (w *Watcher) Start() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			// Only process new JSON files (metadata files from Miner)
			if event.Op&fsnotify.Create == fsnotify.Create {
				if strings.HasSuffix(event.Name, ".json") {
					w.onNewFile(event.Name)
				}
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		case <-w.done:
			return
		}
	}
}

func (w *Watcher) Stop() {
	close(w.done)
	w.watcher.Close()
}

// ParseMinerFilename extracts the timestamp from Miner output files
// Format: audio_1737312000.json -> returns base path without extension
func ParseMinerFilename(jsonPath string) (basePath string, ok bool) {
	if !strings.HasSuffix(jsonPath, ".json") {
		return "", false
	}

	base := strings.TrimSuffix(jsonPath, ".json")
	dir := filepath.Dir(jsonPath)

	// Verify companion files exist pattern
	// audio_123456.json -> audio_123456.wav, audio_123456.png
	return filepath.Join(dir, filepath.Base(base)), true
}
