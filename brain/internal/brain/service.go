// brain/internal/brain/service.go
package brain

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/mitchelldurbin/polybius/brain/internal/fsrs"
	"github.com/mitchelldurbin/polybius/brain/internal/storage"
)

type Service struct {
	db        *storage.DB
	enricher  *Enricher
	scheduler *fsrs.Scheduler
	watcher   *Watcher
	minerDir  string
}

type Config struct {
	DBPath     string
	CEDICTPath string
	MinerDir   string
}

func NewService(cfg Config) (*Service, error) {
	db, err := storage.OpenDatabase(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	enricher, err := NewEnricher(cfg.CEDICTPath)
	if err != nil {
		return nil, err
	}

	svc := &Service{
		db:        db,
		enricher:  enricher,
		scheduler: fsrs.NewScheduler(),
		minerDir:  cfg.MinerDir,
	}

	watcher, err := NewWatcher(cfg.MinerDir, svc.handleNewCapture)
	if err != nil {
		return nil, err
	}
	svc.watcher = watcher

	return svc, nil
}

func (s *Service) Start() {
	log.Printf("Brain started, watching %s for new captures", s.minerDir)
	s.watcher.Start()
}

func (s *Service) Stop() {
	s.watcher.Stop()
	s.db.Close()
}

func (s *Service) DB() *storage.DB {
	return s.db
}

// waitForFileStable waits until a file's size stops changing, indicating the write is complete.
// Returns an error if the file doesn't stabilize within maxWait.
func waitForFileStable(path string, checkInterval, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	var lastSize int64 = -1

	for time.Now().Before(deadline) {
		info, err := os.Stat(path)
		if err != nil {
			// File might not exist yet, keep waiting
			time.Sleep(checkInterval)
			continue
		}

		currentSize := info.Size()
		if currentSize == lastSize && currentSize > 0 {
			// Size hasn't changed and file has content - it's stable
			return nil
		}

		lastSize = currentSize
		time.Sleep(checkInterval)
	}

	return os.ErrDeadlineExceeded
}

func (s *Service) handleNewCapture(jsonPath string) {
	// Wait for file to be fully written by polling file size
	if err := waitForFileStable(jsonPath, 50*time.Millisecond, 5*time.Second); err != nil {
		log.Printf("Timeout waiting for file to stabilize: %s", jsonPath)
		return
	}

	log.Printf("New capture detected: %s", jsonPath)

	// Read metadata JSON
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		log.Printf("Failed to read metadata: %v", err)
		return
	}

	// Supported schema versions
	const maxSupportedSchemaVersion = 1

	var metadata struct {
		SchemaVersion int    `json:"schema_version"`
		Version       string `json:"version"`
		Timestamp     string `json:"timestamp"`
		Audio         struct {
			File string `json:"file"`
		} `json:"audio"`
		Screenshot struct {
			File string `json:"file"`
		} `json:"screenshot"`
		OCR struct {
			Text string `json:"text"`
		} `json:"ocr"`
	}

	if err := json.Unmarshal(data, &metadata); err != nil {
		log.Printf("Failed to parse metadata: %v", err)
		return
	}

	// Validate schema version (0 means field wasn't present - backwards compatible)
	if metadata.SchemaVersion > maxSupportedSchemaVersion {
		log.Printf("Unsupported schema version %d (max supported: %d), skipping: %s",
			metadata.SchemaVersion, maxSupportedSchemaVersion, jsonPath)
		return
	}

	// Skip if no OCR text
	if metadata.OCR.Text == "" {
		log.Printf("No OCR text, skipping")
		return
	}

	dir := filepath.Dir(jsonPath)

	// Create moment record
	moment := &storage.Moment{
		Timestamp:      metadata.Timestamp,
		AudioFile:      filepath.Join(dir, metadata.Audio.File),
		ScreenshotFile: filepath.Join(dir, metadata.Screenshot.File),
		RawText:        metadata.OCR.Text,
		Status:         "pending",
	}

	momentID, err := s.db.InsertMoment(moment)
	if err != nil {
		log.Printf("Failed to insert moment: %v", err)
		return
	}

	// Enrich with vocabulary
	isKnown := func(word string) bool { return s.db.IsWordKnown(word) }
	enriched := s.enricher.EnrichWithVocab(metadata.OCR.Text, isKnown)

	// Update moment with enrichment
	var words []string
	for _, w := range enriched.Words {
		words = append(words, w.Word)
	}
	moment.SegmentedWords = words
	moment.I1Score = enriched.I1Score

	// Create cards for unknown words
	for _, unknownWord := range enriched.UnknownWords {
		// Find the enriched word data
		var pinyin, definition string
		for _, w := range enriched.Words {
			if w.Word == unknownWord {
				pinyin = w.Pinyin
				definition = w.Definition
				break
			}
		}

		// Upsert vocabulary entry
		if err := s.db.UpsertVocabulary(&storage.Vocabulary{
			Word:       unknownWord,
			Pinyin:     pinyin,
			Definition: definition,
			Status:     "unknown",
			TimesSeen:  1,
		}); err != nil {
			log.Printf("Failed to upsert vocabulary: %v", err)
		}

		// Create draft card (requires triage before entering review queue)
		card := &storage.Card{
			MomentID:         momentID,
			TargetWord:       unknownWord,
			TargetPinyin:     pinyin,
			TargetDefinition: definition,
			State:            "draft",
			DueDate:          nil, // No due date until approved
		}

		if _, err := s.db.InsertCard(card); err != nil {
			log.Printf("Failed to insert card: %v", err)
		}
	}

	log.Printf("Processed: %d words, %d unknown, i+1 score: %.2f",
		len(enriched.Words), len(enriched.UnknownWords), enriched.I1Score)
}
