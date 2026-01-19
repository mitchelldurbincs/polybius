# Contextual SRS Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build The Brain (Go backend) and The Gym (TUI) to transform captured moments into reviewable flashcards with FSRS-6 scheduling, optimized for Chinese listening comprehension.

**Architecture:** File watcher ingests captures from The Miner, enriches with jieba segmentation and CC-CEDICT lookups, stores in SQLite with FSRS state. TUI queries due cards and displays with hybrid image window.

**Tech Stack:** Go 1.22+, SQLite (modernc.org/sqlite), jieba (gojieba), FSRS (go-fsrs), TUI (bubbletea), CC-CEDICT (bundled)

---

## Project Structure

```
polybius/
├── brain/                      # NEW: Go backend
│   ├── cmd/
│   │   └── polybius/
│   │       └── main.go         # Entry point (brain/gym modes)
│   ├── internal/
│   │   ├── brain/
│   │   │   ├── watcher.go      # File watcher for captures
│   │   │   ├── enricher.go     # Enrichment pipeline
│   │   │   └── service.go      # Brain service orchestration
│   │   ├── gym/
│   │   │   ├── tui.go          # Bubbletea TUI
│   │   │   ├── review.go       # Review session logic
│   │   │   └── imagewin.go     # Image window spawner
│   │   ├── nlp/
│   │   │   ├── segmenter.go    # Jieba wrapper
│   │   │   └── cedict.go       # CC-CEDICT parser
│   │   ├── fsrs/
│   │   │   └── scheduler.go    # FSRS-6 wrapper
│   │   ├── storage/
│   │   │   ├── db.go           # SQLite operations
│   │   │   └── models.go       # Data models
│   │   └── config/
│   │       └── config.go       # Configuration
│   ├── data/
│   │   └── cedict_ts.u8        # CC-CEDICT dictionary (downloaded)
│   ├── go.mod
│   ├── go.sum
│   └── Makefile
├── src/                        # EXISTING: Rust Miner
└── docs/plans/                 # Plans
```

---

## Phase 1: Go Project Setup

### Task 1.1: Initialize Go Module

**Files:**
- Create: `brain/go.mod`
- Create: `brain/cmd/polybius/main.go`
- Create: `brain/Makefile`

**Step 1: Create brain directory and initialize module**

```bash
cd C:\Users\mitchell.durbin\source\repos\polybius
mkdir -p brain/cmd/polybius
cd brain
go mod init github.com/mitchelldurbin/polybius/brain
```

**Step 2: Create minimal main.go**

```go
// brain/cmd/polybius/main.go
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: polybius <brain|gym>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "brain":
		fmt.Println("Starting The Brain...")
	case "gym":
		fmt.Println("Starting The Gym...")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
```

**Step 3: Create Makefile**

```makefile
# brain/Makefile
.PHONY: build run-brain run-gym test clean

build:
	go build -o bin/polybius ./cmd/polybius

run-brain: build
	./bin/polybius brain

run-gym: build
	./bin/polybius gym

test:
	go test ./...

clean:
	rm -rf bin/
```

**Step 4: Verify it builds**

Run: `cd brain && go build ./cmd/polybius`
Expected: No errors, binary created

**Step 5: Commit**

```bash
git add brain/
git commit -m "feat(brain): initialize Go module with CLI skeleton"
```

---

### Task 1.2: Add Core Dependencies

**Files:**
- Modify: `brain/go.mod`

**Step 1: Add dependencies**

```bash
cd brain
go get modernc.org/sqlite
go get github.com/fsnotify/fsnotify
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss
go get github.com/pelletier/go-toml/v2
```

**Step 2: Verify go.mod updated**

Run: `cat brain/go.mod`
Expected: Shows all dependencies listed

**Step 3: Commit**

```bash
git add brain/go.mod brain/go.sum
git commit -m "feat(brain): add core dependencies"
```

---

## Phase 2: SQLite Database

### Task 2.1: Create Database Schema

**Files:**
- Create: `brain/internal/storage/db.go`
- Create: `brain/internal/storage/db_test.go`

**Step 1: Write the failing test**

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd brain && go test ./internal/storage/... -v`
Expected: FAIL - package not found

**Step 3: Create storage directory and write implementation**

```go
// brain/internal/storage/db.go
package storage

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func OpenDatabase(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func migrate(db *sql.DB) error {
	schema := `
	-- Captured moments with enrichment
	CREATE TABLE IF NOT EXISTS moments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		audio_file TEXT NOT NULL,
		screenshot_file TEXT,
		raw_text TEXT,
		segmented_json TEXT,
		i1_score REAL,
		status TEXT DEFAULT 'pending',
		created_at TEXT DEFAULT CURRENT_TIMESTAMP
	);

	-- Review cards (one per target word per moment)
	CREATE TABLE IF NOT EXISTS cards (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		moment_id INTEGER NOT NULL REFERENCES moments(id),
		target_word TEXT NOT NULL,
		target_pinyin TEXT,
		target_definition TEXT,
		stability REAL DEFAULT 0,
		difficulty REAL DEFAULT 0,
		due_date TEXT,
		last_review TEXT,
		reps INTEGER DEFAULT 0,
		lapses INTEGER DEFAULT 0,
		state TEXT DEFAULT 'new',
		created_at TEXT DEFAULT CURRENT_TIMESTAMP
	);

	-- Vocabulary knowledge
	CREATE TABLE IF NOT EXISTS vocabulary (
		word TEXT PRIMARY KEY,
		pinyin TEXT,
		definition TEXT,
		status TEXT DEFAULT 'unknown',
		times_seen INTEGER DEFAULT 0,
		times_correct INTEGER DEFAULT 0,
		updated_at TEXT DEFAULT CURRENT_TIMESTAMP
	);

	-- Review history
	CREATE TABLE IF NOT EXISTS reviews (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		card_id INTEGER NOT NULL REFERENCES cards(id),
		rating INTEGER NOT NULL,
		time_taken_ms INTEGER,
		reviewed_at TEXT DEFAULT CURRENT_TIMESTAMP
	);

	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_cards_due ON cards(due_date);
	CREATE INDEX IF NOT EXISTS idx_cards_moment ON cards(moment_id);
	CREATE INDEX IF NOT EXISTS idx_vocab_status ON vocabulary(status);
	`

	_, err := db.Exec(schema)
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `cd brain && go test ./internal/storage/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add brain/internal/storage/
git commit -m "feat(brain): add SQLite database with schema"
```

---

### Task 2.2: Add Data Models

**Files:**
- Create: `brain/internal/storage/models.go`
- Create: `brain/internal/storage/models_test.go`

**Step 1: Write the failing test**

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd brain && go test ./internal/storage/... -v`
Expected: FAIL - Moment type undefined

**Step 3: Write implementation**

```go
// brain/internal/storage/models.go
package storage

import (
	"database/sql"
	"encoding/json"
	"time"
)

type Moment struct {
	ID             int64
	Timestamp      string
	AudioFile      string
	ScreenshotFile string
	RawText        string
	SegmentedWords []string
	I1Score        float64
	Status         string
	CreatedAt      time.Time
}

type Card struct {
	ID               int64
	MomentID         int64
	TargetWord       string
	TargetPinyin     string
	TargetDefinition string
	Stability        float64
	Difficulty       float64
	DueDate          *time.Time
	LastReview       *time.Time
	Reps             int
	Lapses           int
	State            string
	CreatedAt        time.Time
}

type Vocabulary struct {
	Word         string
	Pinyin       string
	Definition   string
	Status       string
	TimesSeen    int
	TimesCorrect int
	UpdatedAt    time.Time
}

func (db *DB) InsertMoment(m *Moment) (int64, error) {
	segJSON, _ := json.Marshal(m.SegmentedWords)
	result, err := db.Exec(`
		INSERT INTO moments (timestamp, audio_file, screenshot_file, raw_text, segmented_json, i1_score, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.Timestamp, m.AudioFile, m.ScreenshotFile, m.RawText, string(segJSON), m.I1Score, m.Status,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) GetMoment(id int64) (*Moment, error) {
	m := &Moment{}
	var segJSON sql.NullString
	var screenshot sql.NullString
	var rawText sql.NullString
	var i1Score sql.NullFloat64

	err := db.QueryRow(`
		SELECT id, timestamp, audio_file, screenshot_file, raw_text, segmented_json, i1_score, status, created_at
		FROM moments WHERE id = ?`, id).Scan(
		&m.ID, &m.Timestamp, &m.AudioFile, &screenshot, &rawText, &segJSON, &i1Score, &m.Status, &m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	m.ScreenshotFile = screenshot.String
	m.RawText = rawText.String
	m.I1Score = i1Score.Float64

	if segJSON.Valid {
		json.Unmarshal([]byte(segJSON.String), &m.SegmentedWords)
	}

	return m, nil
}

func (db *DB) InsertCard(c *Card) (int64, error) {
	var dueDate, lastReview interface{}
	if c.DueDate != nil {
		dueDate = c.DueDate.Format(time.RFC3339)
	}
	if c.LastReview != nil {
		lastReview = c.LastReview.Format(time.RFC3339)
	}

	result, err := db.Exec(`
		INSERT INTO cards (moment_id, target_word, target_pinyin, target_definition, stability, difficulty, due_date, last_review, reps, lapses, state)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.MomentID, c.TargetWord, c.TargetPinyin, c.TargetDefinition,
		c.Stability, c.Difficulty, dueDate, lastReview, c.Reps, c.Lapses, c.State,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) GetDueCards(limit int) ([]*Card, error) {
	rows, err := db.Query(`
		SELECT c.id, c.moment_id, c.target_word, c.target_pinyin, c.target_definition,
		       c.stability, c.difficulty, c.due_date, c.last_review, c.reps, c.lapses, c.state
		FROM cards c
		WHERE c.due_date IS NULL OR c.due_date <= datetime('now')
		ORDER BY c.due_date ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []*Card
	for rows.Next() {
		c := &Card{}
		var dueDate, lastReview sql.NullString
		err := rows.Scan(&c.ID, &c.MomentID, &c.TargetWord, &c.TargetPinyin, &c.TargetDefinition,
			&c.Stability, &c.Difficulty, &dueDate, &lastReview, &c.Reps, &c.Lapses, &c.State)
		if err != nil {
			return nil, err
		}
		if dueDate.Valid {
			t, _ := time.Parse(time.RFC3339, dueDate.String)
			c.DueDate = &t
		}
		if lastReview.Valid {
			t, _ := time.Parse(time.RFC3339, lastReview.String)
			c.LastReview = &t
		}
		cards = append(cards, c)
	}
	return cards, nil
}

func (db *DB) GetVocabulary(word string) (*Vocabulary, error) {
	v := &Vocabulary{}
	err := db.QueryRow(`
		SELECT word, pinyin, definition, status, times_seen, times_correct, updated_at
		FROM vocabulary WHERE word = ?`, word).Scan(
		&v.Word, &v.Pinyin, &v.Definition, &v.Status, &v.TimesSeen, &v.TimesCorrect, &v.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (db *DB) UpsertVocabulary(v *Vocabulary) error {
	_, err := db.Exec(`
		INSERT INTO vocabulary (word, pinyin, definition, status, times_seen, times_correct, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(word) DO UPDATE SET
			pinyin = excluded.pinyin,
			definition = excluded.definition,
			status = excluded.status,
			times_seen = excluded.times_seen,
			times_correct = excluded.times_correct,
			updated_at = datetime('now')`,
		v.Word, v.Pinyin, v.Definition, v.Status, v.TimesSeen, v.TimesCorrect,
	)
	return err
}

func (db *DB) IsWordKnown(word string) bool {
	var status string
	err := db.QueryRow("SELECT status FROM vocabulary WHERE word = ?", word).Scan(&status)
	if err != nil {
		return false
	}
	return status == "known"
}
```

**Step 4: Run test to verify it passes**

Run: `cd brain && go test ./internal/storage/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add brain/internal/storage/
git commit -m "feat(brain): add data models with CRUD operations"
```

---

## Phase 3: CC-CEDICT Dictionary

### Task 3.1: Download and Parse CC-CEDICT

**Files:**
- Create: `brain/internal/nlp/cedict.go`
- Create: `brain/internal/nlp/cedict_test.go`
- Download: `brain/data/cedict_ts.u8`

**Step 1: Download CC-CEDICT**

```bash
mkdir -p brain/data
curl -L "https://www.mdbg.net/chinese/export/cedict/cedict_1_0_ts_utf-8_mdbg.txt.gz" | gunzip > brain/data/cedict_ts.u8
```

**Step 2: Write the failing test**

```go
// brain/internal/nlp/cedict_test.go
package nlp

import (
	"testing"
)

func TestCEDICTLookup(t *testing.T) {
	dict, err := LoadCEDICT("../../data/cedict_ts.u8")
	if err != nil {
		t.Fatalf("Failed to load CEDICT: %v", err)
	}

	// Test basic lookup
	entry, ok := dict.Lookup("你好")
	if !ok {
		t.Fatal("Failed to find 你好")
	}
	if entry.Pinyin == "" {
		t.Error("Pinyin should not be empty")
	}
	if entry.Definition == "" {
		t.Error("Definition should not be empty")
	}

	t.Logf("你好: %s - %s", entry.Pinyin, entry.Definition)
}

func TestCEDICTCharacterFallback(t *testing.T) {
	dict, err := LoadCEDICT("../../data/cedict_ts.u8")
	if err != nil {
		t.Fatalf("Failed to load CEDICT: %v", err)
	}

	// Test single character lookup
	entry, ok := dict.Lookup("你")
	if !ok {
		t.Fatal("Failed to find 你")
	}
	if entry.Pinyin == "" {
		t.Error("Pinyin should not be empty for single character")
	}
}
```

**Step 3: Run test to verify it fails**

Run: `cd brain && go test ./internal/nlp/... -v`
Expected: FAIL - package not found

**Step 4: Write implementation**

```go
// brain/internal/nlp/cedict.go
package nlp

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

type DictEntry struct {
	Traditional string
	Simplified  string
	Pinyin      string
	Definition  string
}

type CEDICT struct {
	entries map[string]*DictEntry // keyed by simplified
}

// LoadCEDICT loads the CC-CEDICT dictionary file
func LoadCEDICT(path string) (*CEDICT, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dict := &CEDICT{
		entries: make(map[string]*DictEntry),
	}

	// Pattern: 傳統 简体 [pin1 yin1] /definition 1/definition 2/
	linePattern := regexp.MustCompile(`^(\S+)\s+(\S+)\s+\[([^\]]+)\]\s+/(.+)/$`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		matches := linePattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		entry := &DictEntry{
			Traditional: matches[1],
			Simplified:  matches[2],
			Pinyin:      convertPinyinTones(matches[3]),
			Definition:  strings.ReplaceAll(matches[4], "/", "; "),
		}

		// Store by simplified (primary key for Chinese learners)
		dict.entries[entry.Simplified] = entry
	}

	return dict, scanner.Err()
}

// Lookup finds a word in the dictionary
func (d *CEDICT) Lookup(word string) (*DictEntry, bool) {
	entry, ok := d.entries[word]
	return entry, ok
}

// LookupWithFallback tries the word, then each character individually
func (d *CEDICT) LookupWithFallback(word string) (*DictEntry, bool) {
	if entry, ok := d.Lookup(word); ok {
		return entry, true
	}

	// Fallback: try to build from individual characters
	runes := []rune(word)
	if len(runes) == 1 {
		return nil, false
	}

	var pinyins, defs []string
	for _, r := range runes {
		char := string(r)
		if entry, ok := d.Lookup(char); ok {
			pinyins = append(pinyins, entry.Pinyin)
			defs = append(defs, entry.Definition)
		} else {
			return nil, false
		}
	}

	return &DictEntry{
		Simplified:  word,
		Pinyin:      strings.Join(pinyins, " "),
		Definition:  strings.Join(defs, "; "),
	}, true
}

// convertPinyinTones converts numbered pinyin (ni3 hao3) to tone marks (nǐ hǎo)
func convertPinyinTones(numbered string) string {
	// For now, keep numbered format - can add conversion later
	return strings.ToLower(numbered)
}

// Size returns the number of entries in the dictionary
func (d *CEDICT) Size() int {
	return len(d.entries)
}
```

**Step 5: Run test to verify it passes**

Run: `cd brain && go test ./internal/nlp/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add brain/internal/nlp/ brain/data/
echo "brain/data/cedict_ts.u8" >> .gitignore  # Large file, download separately
git add .gitignore
git commit -m "feat(brain): add CC-CEDICT dictionary parser"
```

---

### Task 3.2: Add Pinyin Tone Mark Conversion

**Files:**
- Modify: `brain/internal/nlp/cedict.go`
- Modify: `brain/internal/nlp/cedict_test.go`

**Step 1: Write the failing test**

```go
// Add to brain/internal/nlp/cedict_test.go

func TestPinyinToneConversion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ni3 hao3", "nǐ hǎo"},
		{"zhong1 guo2", "zhōng guó"},
		{"ma1 ma2 ma3 ma4 ma5", "mā má mǎ mà ma"},
		{"nv3", "nǚ"},
		{"lv4", "lǜ"},
	}

	for _, tt := range tests {
		got := convertPinyinTones(tt.input)
		if got != tt.expected {
			t.Errorf("convertPinyinTones(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd brain && go test ./internal/nlp/... -v -run TestPinyinTone`
Expected: FAIL - output doesn't match

**Step 3: Update implementation**

```go
// Replace convertPinyinTones in brain/internal/nlp/cedict.go

var toneMarks = map[rune][]rune{
	'a': {'ā', 'á', 'ǎ', 'à', 'a'},
	'e': {'ē', 'é', 'ě', 'è', 'e'},
	'i': {'ī', 'í', 'ǐ', 'ì', 'i'},
	'o': {'ō', 'ó', 'ǒ', 'ò', 'o'},
	'u': {'ū', 'ú', 'ǔ', 'ù', 'u'},
	'ü': {'ǖ', 'ǘ', 'ǚ', 'ǜ', 'ü'},
}

// convertPinyinTones converts numbered pinyin (ni3 hao3) to tone marks (nǐ hǎo)
func convertPinyinTones(numbered string) string {
	words := strings.Fields(strings.ToLower(numbered))
	var result []string

	for _, word := range words {
		result = append(result, convertSyllable(word))
	}

	return strings.Join(result, " ")
}

func convertSyllable(syllable string) string {
	// Handle ü written as v
	syllable = strings.ReplaceAll(syllable, "v", "ü")

	// Find the tone number (1-5) at the end
	if len(syllable) == 0 {
		return syllable
	}

	lastChar := syllable[len(syllable)-1]
	if lastChar < '1' || lastChar > '5' {
		return syllable
	}

	tone := int(lastChar - '1') // 0-4
	base := syllable[:len(syllable)-1]

	// Find the vowel to mark (rule: a/e always marked, otherwise last vowel)
	runes := []rune(base)
	markIndex := -1

	for i, r := range runes {
		if r == 'a' || r == 'e' {
			markIndex = i
			break
		}
		if r == 'i' || r == 'o' || r == 'u' || r == 'ü' {
			markIndex = i
		}
	}

	if markIndex == -1 {
		return base
	}

	// Apply tone mark
	vowel := runes[markIndex]
	if marks, ok := toneMarks[vowel]; ok && tone < len(marks) {
		runes[markIndex] = marks[tone]
	}

	return string(runes)
}
```

**Step 4: Run test to verify it passes**

Run: `cd brain && go test ./internal/nlp/... -v -run TestPinyinTone`
Expected: PASS

**Step 5: Commit**

```bash
git add brain/internal/nlp/
git commit -m "feat(brain): add pinyin tone mark conversion"
```

---

## Phase 4: Jieba Word Segmentation

### Task 4.1: Integrate Jieba

**Files:**
- Create: `brain/internal/nlp/segmenter.go`
- Create: `brain/internal/nlp/segmenter_test.go`

**Note:** gojieba requires CGO. On Windows, you need MinGW-w64 or use WSL. Alternative: use a pure-Go segmenter like `github.com/go-ego/gse` if CGO is problematic.

**Step 1: Add dependency**

```bash
cd brain
go get github.com/go-ego/gse
```

**Step 2: Write the failing test**

```go
// brain/internal/nlp/segmenter_test.go
package nlp

import (
	"testing"
)

func TestSegmentChinese(t *testing.T) {
	seg, err := NewSegmenter()
	if err != nil {
		t.Fatalf("Failed to create segmenter: %v", err)
	}

	tests := []struct {
		input    string
		expected []string
	}{
		{"你好世界", []string{"你好", "世界"}},
		{"我今天去超市", []string{"我", "今天", "去", "超市"}},
	}

	for _, tt := range tests {
		got := seg.Segment(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("Segment(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("Segment(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}
```

**Step 3: Run test to verify it fails**

Run: `cd brain && go test ./internal/nlp/... -v -run TestSegment`
Expected: FAIL - NewSegmenter undefined

**Step 4: Write implementation**

```go
// brain/internal/nlp/segmenter.go
package nlp

import (
	"github.com/go-ego/gse"
)

type Segmenter struct {
	seg gse.Segmenter
}

func NewSegmenter() (*Segmenter, error) {
	var seg gse.Segmenter
	// Load default dictionary (embedded)
	seg.LoadDict()
	return &Segmenter{seg: seg}, nil
}

func (s *Segmenter) Segment(text string) []string {
	// Use accurate mode
	return s.seg.CutAll(text)
}

func (s *Segmenter) SegmentSearch(text string) []string {
	// Use search mode (finer granularity)
	return s.seg.CutSearch(text)
}
```

**Step 5: Run test to verify it passes**

Run: `cd brain && go test ./internal/nlp/... -v -run TestSegment`
Expected: PASS (results may vary slightly based on dictionary)

**Step 6: Commit**

```bash
git add brain/internal/nlp/segmenter*.go brain/go.*
git commit -m "feat(brain): add Chinese word segmentation with gse"
```

---

## Phase 5: Enrichment Pipeline

### Task 5.1: Create Enricher Service

**Files:**
- Create: `brain/internal/brain/enricher.go`
- Create: `brain/internal/brain/enricher_test.go`

**Step 1: Write the failing test**

```go
// brain/internal/brain/enricher_test.go
package brain

import (
	"testing"
)

func TestEnrichText(t *testing.T) {
	e, err := NewEnricher("../../data/cedict_ts.u8")
	if err != nil {
		t.Fatalf("Failed to create enricher: %v", err)
	}

	result := e.Enrich("你好世界")

	if len(result.Words) == 0 {
		t.Error("Expected words to be segmented")
	}

	// Check that we got pinyin and definitions
	found := false
	for _, w := range result.Words {
		if w.Word == "你好" {
			found = true
			if w.Pinyin == "" {
				t.Error("Expected pinyin for 你好")
			}
			if w.Definition == "" {
				t.Error("Expected definition for 你好")
			}
		}
	}
	if !found {
		t.Error("Expected to find 你好 in segmented words")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd brain && go test ./internal/brain/... -v`
Expected: FAIL - package not found

**Step 3: Write implementation**

```go
// brain/internal/brain/enricher.go
package brain

import (
	"github.com/mitchelldurbin/polybius/brain/internal/nlp"
)

type EnrichedWord struct {
	Word       string
	Pinyin     string
	Definition string
}

type EnrichedText struct {
	RawText string
	Words   []EnrichedWord
}

type Enricher struct {
	segmenter *nlp.Segmenter
	dict      *nlp.CEDICT
}

func NewEnricher(cedictPath string) (*Enricher, error) {
	seg, err := nlp.NewSegmenter()
	if err != nil {
		return nil, err
	}

	dict, err := nlp.LoadCEDICT(cedictPath)
	if err != nil {
		return nil, err
	}

	return &Enricher{
		segmenter: seg,
		dict:      dict,
	}, nil
}

func (e *Enricher) Enrich(text string) *EnrichedText {
	words := e.segmenter.Segment(text)

	var enriched []EnrichedWord
	for _, word := range words {
		ew := EnrichedWord{Word: word}

		if entry, ok := e.dict.LookupWithFallback(word); ok {
			ew.Pinyin = entry.Pinyin
			ew.Definition = entry.Definition
		}

		enriched = append(enriched, ew)
	}

	return &EnrichedText{
		RawText: text,
		Words:   enriched,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd brain && go test ./internal/brain/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add brain/internal/brain/
git commit -m "feat(brain): add text enrichment pipeline"
```

---

### Task 5.2: Add i+1 Scoring

**Files:**
- Modify: `brain/internal/brain/enricher.go`
- Modify: `brain/internal/brain/enricher_test.go`

**Step 1: Write the failing test**

```go
// Add to brain/internal/brain/enricher_test.go

func TestI1Scoring(t *testing.T) {
	e, err := NewEnricher("../../data/cedict_ts.u8")
	if err != nil {
		t.Fatalf("Failed to create enricher: %v", err)
	}

	// Mock vocabulary: user knows 你好 but not 世界
	knownWords := map[string]bool{"你好": true}
	isKnown := func(word string) bool { return knownWords[word] }

	result := e.EnrichWithVocab("你好世界", isKnown)

	if result.I1Score < 0.4 || result.I1Score > 0.6 {
		t.Errorf("Expected i+1 score around 0.5 (1 of 2 known), got %f", result.I1Score)
	}

	if len(result.UnknownWords) != 1 {
		t.Errorf("Expected 1 unknown word, got %d", len(result.UnknownWords))
	}

	if result.UnknownWords[0] != "世界" {
		t.Errorf("Expected unknown word to be 世界, got %s", result.UnknownWords[0])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd brain && go test ./internal/brain/... -v -run TestI1`
Expected: FAIL - EnrichWithVocab undefined

**Step 3: Update implementation**

```go
// Add to brain/internal/brain/enricher.go

type EnrichedTextWithVocab struct {
	*EnrichedText
	KnownWords   []string
	UnknownWords []string
	I1Score      float64
}

func (e *Enricher) EnrichWithVocab(text string, isKnown func(string) bool) *EnrichedTextWithVocab {
	base := e.Enrich(text)

	var known, unknown []string
	for _, w := range base.Words {
		// Skip punctuation and single characters that aren't Chinese
		if len([]rune(w.Word)) == 0 {
			continue
		}

		if isKnown(w.Word) {
			known = append(known, w.Word)
		} else {
			unknown = append(unknown, w.Word)
		}
	}

	total := len(known) + len(unknown)
	var score float64
	if total > 0 {
		score = float64(len(known)) / float64(total)
	}

	return &EnrichedTextWithVocab{
		EnrichedText: base,
		KnownWords:   known,
		UnknownWords: unknown,
		I1Score:      score,
	}
}

// IsI1 returns true if there's exactly one unknown word (ideal learning condition)
func (e *EnrichedTextWithVocab) IsI1() bool {
	return len(e.UnknownWords) == 1
}

// IsLearnable returns true if 80-99% of words are known
func (e *EnrichedTextWithVocab) IsLearnable() bool {
	return e.I1Score >= 0.8 && e.I1Score < 1.0
}
```

**Step 4: Run test to verify it passes**

Run: `cd brain && go test ./internal/brain/... -v -run TestI1`
Expected: PASS

**Step 5: Commit**

```bash
git add brain/internal/brain/
git commit -m "feat(brain): add i+1 vocabulary scoring"
```

---

## Phase 6: File Watcher

### Task 6.1: Watch for New Captures

**Files:**
- Create: `brain/internal/brain/watcher.go`
- Create: `brain/internal/brain/watcher_test.go`

**Step 1: Write the failing test**

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd brain && go test ./internal/brain/... -v -run TestWatcher`
Expected: FAIL - NewWatcher undefined

**Step 3: Write implementation**

```go
// brain/internal/brain/watcher.go
package brain

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	dir      string
	watcher  *fsnotify.Watcher
	onNewFile func(path string)
	done     chan struct{}
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
```

**Step 4: Run test to verify it passes**

Run: `cd brain && go test ./internal/brain/... -v -run TestWatcher`
Expected: PASS

**Step 5: Commit**

```bash
git add brain/internal/brain/watcher*.go
git commit -m "feat(brain): add file watcher for capture directory"
```

---

## Phase 7: FSRS-6 Integration

### Task 7.1: Verify and Integrate FSRS

**Files:**
- Create: `brain/internal/fsrs/scheduler.go`
- Create: `brain/internal/fsrs/scheduler_test.go`

**Step 1: Add FSRS dependency and check version**

```bash
cd brain
go get github.com/open-spaced-repetition/go-fsrs/v3@latest
```

**Step 2: Write the failing test**

```go
// brain/internal/fsrs/scheduler_test.go
package fsrs

import (
	"testing"
	"time"
)

func TestNewCardScheduling(t *testing.T) {
	s := NewScheduler()

	// New card, first review with "Good" rating
	state := CardState{}
	newState, nextDue := s.Review(state, Good, time.Now())

	if newState.Reps != 1 {
		t.Errorf("Expected 1 rep, got %d", newState.Reps)
	}

	if nextDue.Before(time.Now()) {
		t.Error("Next due should be in the future")
	}

	t.Logf("After Good rating: stability=%.2f, difficulty=%.2f, next due=%v",
		newState.Stability, newState.Difficulty, nextDue)
}

func TestAgainResetsProgress(t *testing.T) {
	s := NewScheduler()

	// Simulate a card that's been reviewed a few times
	state := CardState{
		Stability:  10.0,
		Difficulty: 5.0,
		Reps:       5,
	}

	newState, _ := s.Review(state, Again, time.Now())

	if newState.Lapses != 1 {
		t.Errorf("Expected 1 lapse after Again, got %d", newState.Lapses)
	}

	// Stability should decrease significantly
	if newState.Stability >= state.Stability {
		t.Error("Stability should decrease after Again rating")
	}
}
```

**Step 3: Run test to verify it fails**

Run: `cd brain && go test ./internal/fsrs/... -v`
Expected: FAIL - package not found

**Step 4: Write implementation**

```go
// brain/internal/fsrs/scheduler.go
package fsrs

import (
	"time"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v3"
)

// Rating constants matching FSRS
const (
	Again = 1
	Hard  = 2
	Good  = 3
	Easy  = 4
)

// CardState represents the FSRS state of a card
type CardState struct {
	Stability  float64
	Difficulty float64
	Reps       int
	Lapses     int
	State      int // 0=New, 1=Learning, 2=Review, 3=Relearning
	LastReview time.Time
}

type Scheduler struct {
	params gofsrs.Parameters
}

func NewScheduler() *Scheduler {
	// Use default FSRS-6 parameters
	return &Scheduler{
		params: gofsrs.DefaultParam(),
	}
}

func NewSchedulerWithParams(params gofsrs.Parameters) *Scheduler {
	return &Scheduler{params: params}
}

func (s *Scheduler) Review(state CardState, rating int, now time.Time) (CardState, time.Time) {
	// Convert to go-fsrs Card
	card := gofsrs.Card{
		Due:        now,
		Stability:  state.Stability,
		Difficulty: state.Difficulty,
		Reps:       uint64(state.Reps),
		Lapses:     uint64(state.Lapses),
		State:      gofsrs.State(state.State),
		LastReview: state.LastReview,
	}

	// Create FSRS instance and get scheduling info
	f := gofsrs.NewFSRS(s.params)
	schedulingInfo := f.Repeat(card, now)

	// Get the result for the given rating
	result := schedulingInfo[gofsrs.Rating(rating)]

	return CardState{
		Stability:  result.Card.Stability,
		Difficulty: result.Card.Difficulty,
		Reps:       int(result.Card.Reps),
		Lapses:     int(result.Card.Lapses),
		State:      int(result.Card.State),
		LastReview: now,
	}, result.Card.Due
}

// GetAllSchedules returns the next due dates for all possible ratings
func (s *Scheduler) GetAllSchedules(state CardState, now time.Time) map[int]time.Time {
	card := gofsrs.Card{
		Due:        now,
		Stability:  state.Stability,
		Difficulty: state.Difficulty,
		Reps:       uint64(state.Reps),
		Lapses:     uint64(state.Lapses),
		State:      gofsrs.State(state.State),
		LastReview: state.LastReview,
	}

	f := gofsrs.NewFSRS(s.params)
	schedulingInfo := f.Repeat(card, now)

	result := make(map[int]time.Time)
	for rating := Again; rating <= Easy; rating++ {
		result[rating] = schedulingInfo[gofsrs.Rating(rating)].Card.Due
	}

	return result
}
```

**Step 5: Run test to verify it passes**

Run: `cd brain && go test ./internal/fsrs/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add brain/internal/fsrs/
git commit -m "feat(brain): integrate FSRS-6 scheduler"
```

---

## Phase 8: Brain Service

### Task 8.1: Wire Everything Together

**Files:**
- Create: `brain/internal/brain/service.go`
- Modify: `brain/cmd/polybius/main.go`

**Step 1: Create service that orchestrates components**

```go
// brain/internal/brain/service.go
package brain

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchelldurbin/polybius/brain/internal/fsrs"
	"github.com/mitchelldurbin/polybius/brain/internal/storage"
)

type Service struct {
	db       *storage.DB
	enricher *Enricher
	scheduler *fsrs.Scheduler
	watcher  *Watcher
	minerDir string
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

func (s *Service) handleNewCapture(jsonPath string) {
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

		// Create review card
		now := time.Now()
		card := &storage.Card{
			MomentID:         momentID,
			TargetWord:       unknownWord,
			TargetPinyin:     pinyin,
			TargetDefinition: definition,
			State:            "new",
			DueDate:          &now,
		}

		if _, err := s.db.InsertCard(card); err != nil {
			log.Printf("Failed to insert card: %v", err)
		}
	}

	log.Printf("Processed: %d words, %d unknown, i+1 score: %.2f",
		len(enriched.Words), len(enriched.UnknownWords), enriched.I1Score)
}
```

**Step 2: Update main.go**

```go
// brain/cmd/polybius/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mitchelldurbin/polybius/brain/internal/brain"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: polybius <brain|gym>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "brain":
		runBrain()
	case "gym":
		runGym()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runBrain() {
	// Default paths - should come from config
	homeDir, _ := os.UserHomeDir()
	minerDir := filepath.Join(homeDir, "Music", "Miner")
	dbPath := filepath.Join(homeDir, ".polybius", "brain.db")
	cedictPath := filepath.Join(homeDir, ".polybius", "cedict_ts.u8")

	// Ensure directories exist
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	cfg := brain.Config{
		DBPath:     dbPath,
		CEDICTPath: cedictPath,
		MinerDir:   minerDir,
	}

	svc, err := brain.NewService(cfg)
	if err != nil {
		log.Fatalf("Failed to start Brain: %v", err)
	}

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go svc.Start()

	fmt.Println("The Brain is running. Press Ctrl+C to stop.")
	<-sigChan

	fmt.Println("\nShutting down...")
	svc.Stop()
}

func runGym() {
	fmt.Println("The Gym - Coming soon!")
	// TODO: Implement TUI
}
```

**Step 3: Build and verify**

Run: `cd brain && go build ./cmd/polybius`
Expected: Binary builds successfully

**Step 4: Commit**

```bash
git add brain/
git commit -m "feat(brain): wire up service with file watcher and enrichment"
```

---

## Phase 9: The Gym TUI

### Task 9.1: Basic TUI Shell with Bubbletea

**Files:**
- Create: `brain/internal/gym/tui.go`
- Create: `brain/internal/gym/review.go`

**Step 1: Write basic TUI**

```go
// brain/internal/gym/tui.go
package gym

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	cardStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(60)

	targetStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212"))

	hiddenStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
)

type Model struct {
	cards       []*ReviewCard
	currentIdx  int
	revealed    bool
	quitting    bool
	err         error
}

type ReviewCard struct {
	ID         int64
	Sentence   string
	TargetWord string
	Pinyin     string
	Definition string
	AudioFile  string
	ImageFile  string
}

func NewModel(cards []*ReviewCard) Model {
	return Model{
		cards:      cards,
		currentIdx: 0,
		revealed:   false,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case " ": // Space to reveal
			m.revealed = true
			return m, nil

		case "1", "2", "3", "4":
			if m.revealed && m.currentIdx < len(m.cards) {
				// Process rating
				// rating := int(msg.String()[0] - '0')
				// TODO: Submit rating to Brain

				// Move to next card
				m.currentIdx++
				m.revealed = false

				if m.currentIdx >= len(m.cards) {
					m.quitting = true
					return m, tea.Quit
				}
			}
			return m, nil

		case "r": // Replay audio
			// TODO: Play audio
			return m, nil
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return "Session complete!\n"
	}

	if len(m.cards) == 0 {
		return "No cards due for review.\n"
	}

	card := m.cards[m.currentIdx]

	// Header
	header := titleStyle.Render(fmt.Sprintf("POLYBIUS GYM    Due: %d", len(m.cards)-m.currentIdx))

	// Card content
	var content string
	content += fmt.Sprintf("Sentence: %s\n\n", highlightWord(card.Sentence, card.TargetWord))
	content += fmt.Sprintf("Target: %s\n", targetStyle.Render(card.TargetWord))

	if m.revealed {
		content += fmt.Sprintf("Pinyin: %s\n", card.Pinyin)
		content += fmt.Sprintf("Meaning: %s\n", card.Definition)
	} else {
		content += fmt.Sprintf("Pinyin: %s\n", hiddenStyle.Render("[hidden]"))
		content += fmt.Sprintf("Meaning: %s\n", hiddenStyle.Render("[hidden]"))
	}

	cardView := cardStyle.Render(content)

	// Help
	var help string
	if m.revealed {
		help = helpStyle.Render("[1] Again  [2] Hard  [3] Good  [4] Easy    [Q] Quit")
	} else {
		help = helpStyle.Render("[Space] Reveal  [R] Replay    [Q] Quit")
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s\n", header, cardView, help)
}

func highlightWord(sentence, word string) string {
	// Simple highlight - could use more sophisticated styling
	return sentence // TODO: highlight the target word
}
```

**Step 2: Create review session logic**

```go
// brain/internal/gym/review.go
package gym

import (
	"github.com/mitchelldurbin/polybius/brain/internal/storage"
)

type Session struct {
	db *storage.DB
}

func NewSession(db *storage.DB) *Session {
	return &Session{db: db}
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
```

**Step 3: Update main.go to run Gym**

```go
// Update runGym() in brain/cmd/polybius/main.go

func runGym() {
	homeDir, _ := os.UserHomeDir()
	dbPath := filepath.Join(homeDir, ".polybius", "brain.db")

	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	session := gym.NewSession(db)
	cards, err := session.GetDueCards(20)
	if err != nil {
		log.Fatalf("Failed to get cards: %v", err)
	}

	model := gym.NewModel(cards)
	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running TUI: %v", err)
	}
}
```

**Step 4: Build and test**

Run: `cd brain && go build ./cmd/polybius && ./bin/polybius gym`
Expected: TUI appears (may show "No cards due" if empty)

**Step 5: Commit**

```bash
git add brain/
git commit -m "feat(gym): add basic TUI with bubbletea"
```

---

### Task 9.2: Add Image Window

**Files:**
- Create: `brain/internal/gym/imagewin.go`

**Step 1: Create image window spawner**

```go
// brain/internal/gym/imagewin.go
package gym

import (
	"os/exec"
	"runtime"
)

type ImageWindow struct {
	cmd *exec.Cmd
}

func NewImageWindow() *ImageWindow {
	return &ImageWindow{}
}

// Show opens the image in the system default viewer
// This is a simple approach - for better UX, could use a dedicated window
func (w *ImageWindow) Show(imagePath string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", imagePath)
	case "darwin":
		cmd = exec.Command("open", imagePath)
	default: // linux
		cmd = exec.Command("xdg-open", imagePath)
	}

	return cmd.Start()
}

// For a more sophisticated approach, could use fyne or similar
// to create a dedicated always-on-top image window that updates
```

**Step 2: Integrate into TUI**

Update the TUI Update function to show image when card changes:

```go
// Add to Model struct
imageWin *ImageWindow

// In NewModel:
imageWin: NewImageWindow(),

// In Update, after moving to next card:
if card.ImageFile != "" {
    m.imageWin.Show(card.ImageFile)
}
```

**Step 3: Commit**

```bash
git add brain/internal/gym/
git commit -m "feat(gym): add image window support"
```

---

## Phase 10: Audio Playback

### Task 10.1: Add Audio Player

**Files:**
- Create: `brain/internal/gym/audio.go`

**Step 1: Add audio dependency**

```bash
cd brain
go get github.com/gopxl/beep/v2
go get github.com/gopxl/beep/v2/wav
go get github.com/gopxl/beep/v2/speaker
```

**Step 2: Create audio player**

```go
// brain/internal/gym/audio.go
package gym

import (
	"os"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

type AudioPlayer struct {
	initialized bool
	sampleRate  beep.SampleRate
}

func NewAudioPlayer() *AudioPlayer {
	return &AudioPlayer{}
}

func (p *AudioPlayer) Play(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}

	streamer, format, err := wav.Decode(f)
	if err != nil {
		f.Close()
		return err
	}

	if !p.initialized {
		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
		p.initialized = true
		p.sampleRate = format.SampleRate
	}

	// Resample if needed
	var toPlay beep.Streamer = streamer
	if format.SampleRate != p.sampleRate {
		toPlay = beep.Resample(4, format.SampleRate, p.sampleRate, streamer)
	}

	done := make(chan bool)
	speaker.Play(beep.Seq(toPlay, beep.Callback(func() {
		done <- true
	})))

	<-done
	streamer.Close()
	f.Close()

	return nil
}

// PlayAsync plays audio without blocking
func (p *AudioPlayer) PlayAsync(filePath string) {
	go p.Play(filePath)
}
```

**Step 3: Integrate into TUI**

Add audio player to Model and trigger on 'r' key and when card loads.

**Step 4: Commit**

```bash
git add brain/
git commit -m "feat(gym): add audio playback support"
```

---

## Final Phase: Integration Testing

### Task 11.1: End-to-End Test

**Manual testing checklist:**

1. Start The Brain: `./polybius brain`
2. Verify it's watching the Miner directory
3. Trigger a capture from The Miner (Ctrl+Alt+2)
4. Check Brain logs for "New capture detected"
5. Verify card created in database
6. Start The Gym: `./polybius gym`
7. Verify card appears
8. Test Space to reveal
9. Test 1-4 ratings
10. Verify scheduling works

**Step: Commit final integration**

```bash
git add .
git commit -m "feat: complete Brain + Gym integration"
```

---

## Summary

| Phase | Tasks | Description |
|-------|-------|-------------|
| 1 | 1.1-1.2 | Go project setup |
| 2 | 2.1-2.2 | SQLite database |
| 3 | 3.1-3.2 | CC-CEDICT dictionary |
| 4 | 4.1 | Jieba segmentation |
| 5 | 5.1-5.2 | Enrichment + i+1 |
| 6 | 6.1 | File watcher |
| 7 | 7.1 | FSRS-6 scheduler |
| 8 | 8.1 | Brain service |
| 9 | 9.1-9.2 | Gym TUI + images |
| 10 | 10.1 | Audio playback |
| 11 | 11.1 | Integration testing |

**Total: ~25 bite-sized tasks**
