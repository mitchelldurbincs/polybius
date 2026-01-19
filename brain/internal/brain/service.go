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

func (s *Service) handleNewCapture(jsonPath string) {
	// Wait for file to be fully written (race condition with Miner)
	time.Sleep(200 * time.Millisecond)

	log.Printf("New capture detected: %s", jsonPath)

	// Read metadata JSON
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		log.Printf("Failed to read metadata: %v", err)
		return
	}

	var metadata struct {
		Version   string `json:"version"`
		Timestamp string `json:"timestamp"`
		Audio     struct {
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
		s.db.UpsertVocabulary(&storage.Vocabulary{
			Word:       unknownWord,
			Pinyin:     pinyin,
			Definition: definition,
			Status:     "unknown",
			TimesSeen:  1,
		})

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
