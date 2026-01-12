# Stage 5 Implementation Plan: The Brain (Go Backend)

**Goal:** Build the central API backend that receives artifacts from The Miner, processes them with NLP, and schedules reviews using FSRS.

---

## Overview

The Brain is the intelligence layer of Polybius. It transforms raw captures (audio + screenshot + OCR text) into reviewable flashcards with intelligent scheduling. This is where language learning magic happens:

1. **Ingest** artifacts from The Miner
2. **Segment** sentences using NLP (jieba for Chinese)
3. **Analyze** vocabulary to find "i+1" sentences
4. **Schedule** reviews using FSRS algorithm
5. **Serve** cards to The Gym for review sessions

---

## What The Brain Does

| Component | Purpose |
|-----------|---------|
| **API Server** | REST/gRPC endpoints for artifact upload and card retrieval |
| **NLP Pipeline** | Sentence segmentation, tokenization, word frequency analysis |
| **i+1 Engine** | Identifies sentences where user knows all words except one |
| **FSRS Scheduler** | Spaced repetition scheduling for optimal retention |
| **Storage** | SQLite database for cards, vocabulary, and review history |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           The Brain (Go)                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐              │
│  │  HTTP/gRPC   │    │     NLP      │    │    FSRS      │              │
│  │   Server     │───▶│   Pipeline   │───▶│  Scheduler   │              │
│  └──────────────┘    └──────────────┘    └──────────────┘              │
│         │                   │                   │                       │
│         │                   ▼                   │                       │
│         │           ┌──────────────┐           │                       │
│         │           │    i+1       │           │                       │
│         │           │   Engine     │           │                       │
│         │           └──────────────┘           │                       │
│         │                   │                   │                       │
│         ▼                   ▼                   ▼                       │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                      SQLite Database                             │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐            │   │
│  │  │  Cards  │  │ Vocab   │  │ Reviews │  │ Settings│            │   │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘            │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
         ▲                                              │
         │ Upload Artifacts                             │ Get Due Cards
         │                                              ▼
┌─────────────────┐                          ┌─────────────────┐
│   The Miner     │                          │    The Gym      │
│    (Rust)       │                          │     (TUI)       │
└─────────────────┘                          └─────────────────┘
```

---

## Technology Choices

### Language: Go

**Why Go for The Brain:**
- Excellent HTTP/gRPC server performance
- Simple deployment (single binary)
- Strong concurrency model for background processing
- Good ecosystem for NLP libraries
- Easy to self-host on home server or cloud

### Key Libraries

| Library | Purpose | Notes |
|---------|---------|-------|
| `github.com/gin-gonic/gin` | HTTP router | Fast, simple API framework |
| `github.com/yanyiwu/gojieba` | Chinese segmentation | Go bindings for jieba |
| `github.com/open-spaced-repetition/go-fsrs` | FSRS algorithm | Official Go implementation |
| `modernc.org/sqlite` | Database | Pure Go SQLite (no CGO) |
| `github.com/google/uuid` | IDs | UUID generation for cards |

---

## Database Schema

### Tables

```sql
-- Cards table: stores captured artifacts as reviewable cards
CREATE TABLE cards (
    id TEXT PRIMARY KEY,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    -- Artifact data
    audio_path TEXT NOT NULL,           -- Path to WAV file
    screenshot_path TEXT,               -- Path to PNG file
    ocr_text TEXT,                      -- Raw OCR text from screenshot

    -- Processed data
    sentence TEXT,                      -- Cleaned/segmented sentence
    target_word TEXT,                   -- The "unknown" word (i+1)
    context TEXT,                       -- Surrounding context
    source TEXT,                        -- Where it was captured (Netflix, etc.)

    -- FSRS fields
    fsrs_state TEXT,                    -- JSON blob of FSRS card state
    due_at DATETIME,                    -- When card is due for review

    -- Status
    status TEXT DEFAULT 'new'           -- new, learning, review, suspended
);

-- Vocabulary table: tracks known/unknown words
CREATE TABLE vocabulary (
    word TEXT PRIMARY KEY,
    language TEXT NOT NULL,             -- zh-Hans, ja, etc.
    frequency INTEGER DEFAULT 0,        -- How often seen in cards
    known BOOLEAN DEFAULT FALSE,        -- User marks as known
    last_seen DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Reviews table: history of all reviews for analytics
CREATE TABLE reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    card_id TEXT NOT NULL,
    reviewed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    rating INTEGER NOT NULL,            -- 1=Again, 2=Hard, 3=Good, 4=Easy
    duration_ms INTEGER,                -- Time spent on review
    FOREIGN KEY (card_id) REFERENCES cards(id)
);

-- Settings table: user preferences
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Indexes for performance
CREATE INDEX idx_cards_due_at ON cards(due_at);
CREATE INDEX idx_cards_status ON cards(status);
CREATE INDEX idx_vocabulary_known ON vocabulary(known);
CREATE INDEX idx_reviews_card_id ON reviews(card_id);
```

---

## API Design

### REST Endpoints

```
POST   /api/v1/artifacts          Upload new artifact from Miner
GET    /api/v1/cards              List all cards (with filters)
GET    /api/v1/cards/:id          Get single card details
DELETE /api/v1/cards/:id          Delete a card
PATCH  /api/v1/cards/:id          Update card (edit sentence, target word)

GET    /api/v1/review/due         Get cards due for review
POST   /api/v1/review/:id         Submit review rating (1-4)

GET    /api/v1/vocabulary         List vocabulary with known status
PATCH  /api/v1/vocabulary/:word   Mark word as known/unknown

GET    /api/v1/stats              Get learning statistics
GET    /api/v1/health             Health check endpoint
```

### Request/Response Examples

#### Upload Artifact

```http
POST /api/v1/artifacts
Content-Type: multipart/form-data

audio: <binary wav data>
screenshot: <binary png data>  (optional)
metadata: {
  "version": "1.0",
  "timestamp": "2024-01-12T15:30:45Z",
  "audio": {
    "duration_seconds": 10.0,
    "sample_rate": 48000
  },
  "ocr": {
    "language": "zh-Hans",
    "text": "你好世界",
    "words": [...]
  }
}
```

```json
// Response
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "processing",
  "message": "Artifact received, processing NLP"
}
```

#### Get Due Cards

```http
GET /api/v1/review/due?limit=10
```

```json
// Response
{
  "cards": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "sentence": "我今天去了超市",
      "target_word": "超市",
      "audio_url": "/files/audio_1736700000.wav",
      "screenshot_url": "/files/audio_1736700000.png",
      "due_at": "2024-01-12T15:30:45Z"
    }
  ],
  "total_due": 42
}
```

#### Submit Review

```http
POST /api/v1/review/550e8400-e29b-41d4-a716-446655440000
Content-Type: application/json

{
  "rating": 3,
  "duration_ms": 5200
}
```

```json
// Response
{
  "next_due": "2024-01-15T10:00:00Z",
  "interval_days": 3,
  "ease_factor": 2.5
}
```

---

## NLP Pipeline

### Chinese Segmentation with jieba

```go
package nlp

import (
    "github.com/yanyiwu/gojieba"
)

type Segmenter struct {
    jieba *gojieba.Jieba
}

func NewSegmenter() *Segmenter {
    return &Segmenter{
        jieba: gojieba.NewJieba(),
    }
}

func (s *Segmenter) Segment(text string) []string {
    // Cut with accurate mode for better results
    return s.jieba.Cut(text, true)
}

func (s *Segmenter) SegmentForSearch(text string) []string {
    // Cut for search indexing (finer granularity)
    return s.jieba.CutForSearch(text)
}

// Example:
// Input: "我今天去了超市买东西"
// Output: ["我", "今天", "去", "了", "超市", "买", "东西"]
```

### Language Detection

```go
package nlp

// Simple heuristic-based language detection
func DetectLanguage(text string) string {
    for _, r := range text {
        switch {
        case r >= 0x4E00 && r <= 0x9FFF:
            return "zh-Hans"  // CJK Unified Ideographs (Chinese)
        case r >= 0x3040 && r <= 0x309F:
            return "ja"       // Hiragana (Japanese)
        case r >= 0xAC00 && r <= 0xD7AF:
            return "ko"       // Hangul (Korean)
        }
    }
    return "en"  // Default to English
}
```

---

## i+1 Engine

The i+1 principle: optimal learning happens when you understand everything except one element.

### Algorithm

```go
package i1

type I1Engine struct {
    vocab VocabularyStore
}

type SentenceAnalysis struct {
    Sentence     string
    Words        []string
    KnownWords   []string
    UnknownWords []string
    Score        float64  // 0.0 = all unknown, 1.0 = all known
    IsI1         bool     // Exactly one unknown word
    TargetWord   string   // The unknown word (if i+1)
}

func (e *I1Engine) Analyze(sentence string, words []string) SentenceAnalysis {
    var known, unknown []string

    for _, word := range words {
        if e.vocab.IsKnown(word) {
            known = append(known, word)
        } else {
            unknown = append(unknown, word)
        }
    }

    score := float64(len(known)) / float64(len(words))
    isI1 := len(unknown) == 1

    var target string
    if isI1 {
        target = unknown[0]
    }

    return SentenceAnalysis{
        Sentence:     sentence,
        Words:        words,
        KnownWords:   known,
        UnknownWords: unknown,
        Score:        score,
        IsI1:         isI1,
        TargetWord:   target,
    }
}

// Priority scoring for card queue:
// 1. Perfect i+1 (one unknown) = highest priority
// 2. Near i+1 (2-3 unknown) = medium priority
// 3. Many unknowns = low priority (too hard)
// 4. All known = very low priority (no learning value)
func (e *I1Engine) PriorityScore(analysis SentenceAnalysis) float64 {
    unknownCount := len(analysis.UnknownWords)

    switch {
    case unknownCount == 1:
        return 1.0   // Perfect i+1
    case unknownCount == 2:
        return 0.7   // Almost i+1
    case unknownCount == 3:
        return 0.4   // Challenging but learnable
    case unknownCount > 3:
        return 0.1   // Too many unknowns
    default:
        return 0.05  // All known, minimal value
    }
}
```

### Vocabulary Learning Loop

```
┌─────────────────────────────────────────────────────────────┐
│                  Vocabulary Learning Loop                    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   1. User captures sentence                                  │
│          │                                                   │
│          ▼                                                   │
│   2. NLP segments: ["我", "今天", "去", "超市"]              │
│          │                                                   │
│          ▼                                                   │
│   3. i+1 check:                                             │
│      Known: ["我", "今天", "去"]                             │
│      Unknown: ["超市"]                                       │
│      → This is i+1! Target = "超市"                         │
│          │                                                   │
│          ▼                                                   │
│   4. Card created with "超市" as target                     │
│          │                                                   │
│          ▼                                                   │
│   5. User reviews, rates "Good"                             │
│          │                                                   │
│          ▼                                                   │
│   6. "超市" added to known vocabulary                        │
│          │                                                   │
│          ▼                                                   │
│   7. Future cards with "超市" now have one less unknown     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## FSRS Integration

### Using go-fsrs

```go
package scheduler

import (
    "time"
    "github.com/open-spaced-repetition/go-fsrs"
)

type Scheduler struct {
    fsrs *fsrs.FSRS
}

func NewScheduler() *Scheduler {
    // Default FSRS parameters (can be tuned per-user)
    params := fsrs.DefaultParam()
    return &Scheduler{
        fsrs: fsrs.NewFSRS(params),
    }
}

type CardState struct {
    Difficulty float64   `json:"difficulty"`
    Stability  float64   `json:"stability"`
    Reps       int       `json:"reps"`
    Lapses     int       `json:"lapses"`
    State      int       `json:"state"`
    LastReview time.Time `json:"last_review"`
}

// Rating constants
const (
    Again = 1  // Complete failure
    Hard  = 2  // Significant difficulty
    Good  = 3  // Correct with some effort
    Easy  = 4  // Perfect recall
)

func (s *Scheduler) Review(state CardState, rating int) (CardState, time.Time) {
    card := fsrs.Card{
        Difficulty: state.Difficulty,
        Stability:  state.Stability,
        Reps:       state.Reps,
        Lapses:     state.Lapses,
        State:      fsrs.State(state.State),
        LastReview: state.LastReview,
    }

    now := time.Now()
    result := s.fsrs.Review(card, now, fsrs.Rating(rating))

    newState := CardState{
        Difficulty: result.Card.Difficulty,
        Stability:  result.Card.Stability,
        Reps:       result.Card.Reps,
        Lapses:     result.Card.Lapses,
        State:      int(result.Card.State),
        LastReview: now,
    }

    nextDue := result.Card.Due
    return newState, nextDue
}
```

### FSRS States

| State | Description | Typical Interval |
|-------|-------------|------------------|
| `New` | Never reviewed | Due immediately |
| `Learning` | In initial learning phase | Minutes to hours |
| `Review` | In long-term review | Days to months |
| `Relearning` | Failed after being in Review | Back to minutes |

---

## Project Structure

```
brain/
├── cmd/
│   └── brain/
│       └── main.go              # Entry point
├── internal/
│   ├── api/
│   │   ├── router.go            # HTTP router setup
│   │   ├── handlers.go          # Request handlers
│   │   ├── middleware.go        # Auth, logging, etc.
│   │   └── responses.go         # Response helpers
│   ├── models/
│   │   ├── card.go              # Card model
│   │   ├── vocabulary.go        # Vocabulary model
│   │   └── review.go            # Review model
│   ├── nlp/
│   │   ├── segmenter.go         # Text segmentation
│   │   ├── language.go          # Language detection
│   │   └── tokenizer.go         # Word tokenization
│   ├── i1/
│   │   ├── engine.go            # i+1 analysis
│   │   └── priority.go          # Card prioritization
│   ├── scheduler/
│   │   ├── fsrs.go              # FSRS wrapper
│   │   └── queue.go             # Review queue management
│   ├── storage/
│   │   ├── sqlite.go            # Database operations
│   │   ├── migrations.go        # Schema migrations
│   │   └── files.go             # File storage for audio/images
│   └── config/
│       └── config.go            # Configuration loading
├── migrations/
│   └── 001_initial.sql          # Initial schema
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Implementation Phases

### Phase 1: Project Setup & API Skeleton

**Goal:** Basic HTTP server with health endpoint and database connection.

**Steps:**
1. Initialize Go module: `go mod init github.com/yourusername/polybius-brain`
2. Set up project structure
3. Implement config loading (TOML or YAML)
4. Create SQLite database with schema migrations
5. Implement basic HTTP server with Gin
6. Add health check endpoint
7. Add CORS middleware for local development

**Deliverable:** Running server that responds to `/api/v1/health`

### Phase 2: Artifact Ingestion

**Goal:** Accept artifact uploads from The Miner.

**Steps:**
1. Implement multipart file upload handler
2. Create file storage system (local directory structure)
3. Parse metadata JSON from upload
4. Store card record in database
5. Return card ID to client
6. Add basic validation (file types, sizes)

**Deliverable:** Can upload artifact and see it in database

### Phase 3: NLP Pipeline

**Goal:** Segment and analyze uploaded text.

**Steps:**
1. Integrate gojieba for Chinese segmentation
2. Implement language detection
3. Create background job to process new cards
4. Extract and store individual words in vocabulary table
5. Calculate word frequencies
6. Handle multiple languages (Chinese, Japanese)

**Deliverable:** Uploaded artifacts are automatically segmented

### Phase 4: i+1 Engine

**Goal:** Identify optimal cards for learning.

**Steps:**
1. Implement vocabulary known/unknown tracking
2. Create i+1 analysis algorithm
3. Add priority scoring for cards
4. Create endpoint to mark words as known
5. Implement "smart queue" that prioritizes i+1 cards
6. Add vocabulary statistics endpoint

**Deliverable:** Cards are ranked by learning value

### Phase 5: FSRS Scheduler

**Goal:** Implement spaced repetition scheduling.

**Steps:**
1. Integrate go-fsrs library
2. Store FSRS state per card
3. Implement review submission endpoint
4. Calculate and store next due date
5. Create endpoint to fetch due cards
6. Add review history tracking

**Deliverable:** Full SRS loop working

### Phase 6: Polish & Production Readiness

**Goal:** Make it deployable and robust.

**Steps:**
1. Add request logging middleware
2. Implement graceful shutdown
3. Add rate limiting
4. Create Docker container
5. Add configuration for different environments
6. Write API documentation
7. Add basic authentication (API key)

**Deliverable:** Production-ready deployable binary

---

## Configuration

### Config File (config.toml)

```toml
[server]
host = "0.0.0.0"
port = 8080
mode = "release"  # "debug" or "release"

[database]
path = "./data/brain.db"

[storage]
# Where to store uploaded audio/screenshots
artifacts_dir = "./data/artifacts"
# Maximum file sizes
max_audio_size_mb = 50
max_image_size_mb = 10

[nlp]
# Default language for segmentation
default_language = "zh-Hans"
# Path to jieba dictionary (optional, uses default if not set)
jieba_dict_path = ""

[fsrs]
# FSRS parameters (use defaults if not set)
# See: https://github.com/open-spaced-repetition/fsrs4anki/wiki/The-Algorithm
request_retention = 0.9
maximum_interval = 36500
weights = []  # Empty = use defaults

[auth]
# Simple API key authentication
enabled = false
api_key = ""
```

---

## Communication with The Miner

### Option A: Direct HTTP Upload (Recommended for MVP)

The Miner uploads artifacts directly to The Brain's API.

**Pros:**
- Simple to implement
- Real-time processing
- No intermediate storage needed

**Cons:**
- Requires network connectivity
- Brain must be running when capturing

### Option B: File Sync + Watch (Alternative)

The Miner saves to a synced folder (Dropbox/Syncthing), Brain watches for new files.

**Pros:**
- Works offline
- Decoupled architecture

**Cons:**
- More complex setup
- Delayed processing

### Implementation (Option A)

Add to The Miner's config:

```toml
[brain]
enabled = true
url = "http://localhost:8080/api/v1"
api_key = ""  # Optional
upload_immediately = true  # vs. batch upload
retry_on_failure = true
max_retries = 3
```

Add upload module to The Miner:

```rust
// src/upload.rs
pub async fn upload_artifact(
    client: &reqwest::Client,
    brain_url: &str,
    audio_path: &Path,
    screenshot_path: Option<&Path>,
    metadata: &Metadata,
) -> Result<String, UploadError> {
    let form = multipart::Form::new()
        .file("audio", audio_path)?
        .text("metadata", serde_json::to_string(metadata)?);

    let form = if let Some(screenshot) = screenshot_path {
        form.file("screenshot", screenshot)?
    } else {
        form
    };

    let response = client
        .post(&format!("{}/artifacts", brain_url))
        .multipart(form)
        .send()
        .await?;

    let result: UploadResponse = response.json().await?;
    Ok(result.id)
}
```

---

## Deployment Options

### 1. Local Development

```bash
cd brain
go run ./cmd/brain
# Runs on localhost:8080
```

### 2. Home Server (Docker)

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o brain ./cmd/brain

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/brain /usr/local/bin/
EXPOSE 8080
CMD ["brain"]
```

```bash
docker build -t polybius-brain .
docker run -p 8080:8080 -v ./data:/data polybius-brain
```

### 3. Cloud (Fly.io / Railway)

Single-command deployment to edge:

```bash
fly launch
fly deploy
```

---

## Testing Plan

### Unit Tests

```go
// internal/nlp/segmenter_test.go
func TestChineseSegmentation(t *testing.T) {
    s := NewSegmenter()
    words := s.Segment("我今天去了超市")
    expected := []string{"我", "今天", "去", "了", "超市"}
    assert.Equal(t, expected, words)
}

// internal/i1/engine_test.go
func TestI1Detection(t *testing.T) {
    vocab := NewMockVocab(map[string]bool{
        "我": true, "今天": true, "去": true,
    })
    engine := NewI1Engine(vocab)

    analysis := engine.Analyze("我今天去超市",
        []string{"我", "今天", "去", "超市"})

    assert.True(t, analysis.IsI1)
    assert.Equal(t, "超市", analysis.TargetWord)
}
```

### Integration Tests

```go
func TestArtifactUploadFlow(t *testing.T) {
    // 1. Start test server
    // 2. Upload test artifact
    // 3. Verify card created in database
    // 4. Verify NLP processing completed
    // 5. Verify vocabulary updated
}

func TestReviewFlow(t *testing.T) {
    // 1. Create test card
    // 2. Submit review with rating=Good
    // 3. Verify FSRS state updated
    // 4. Verify next due date calculated
    // 5. Verify review history recorded
}
```

### Manual Testing Checklist

- [ ] Upload artifact via curl
- [ ] Verify files stored correctly
- [ ] Check card appears in database
- [ ] Verify Chinese segmentation works
- [ ] Test i+1 detection with known vocabulary
- [ ] Submit review and verify scheduling
- [ ] Get due cards and verify order
- [ ] Mark word as known and verify i+1 updates

---

## Success Criteria

1. **API functional:** All endpoints return correct responses
2. **Upload working:** Can upload artifact from curl or Miner
3. **NLP working:** Chinese text is correctly segmented
4. **i+1 working:** Cards are ranked by learning value
5. **FSRS working:** Reviews update scheduling correctly
6. **Performance:** API responds in <100ms for typical requests
7. **Reliability:** Graceful handling of malformed input

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| jieba dictionary size (~50MB) | Embed in binary or download on first run |
| SQLite concurrency limits | Use WAL mode, consider Postgres for multi-user |
| Large audio files | Implement upload size limits, stream to disk |
| API authentication | Start with API key, add OAuth later if needed |
| NLP accuracy | Allow manual correction of segmentation |
| FSRS parameter tuning | Expose parameters in config, add per-user tuning later |

---

## Future Enhancements (Stage 6+)

- [ ] Multiple user support with authentication
- [ ] Audio analysis (auto-detect speech boundaries)
- [ ] Integration with external dictionaries (CC-CEDICT)
- [ ] Export to Anki format
- [ ] Web dashboard for statistics
- [ ] Backup/restore functionality
- [ ] Real-time sync across devices

---

## Dependencies Summary

```go
// go.mod
module github.com/yourusername/polybius-brain

go 1.22

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/yanyiwu/gojieba v1.3.0
    github.com/open-spaced-repetition/go-fsrs v1.0.0
    modernc.org/sqlite v1.28.0
    github.com/google/uuid v1.5.0
    github.com/pelletier/go-toml/v2 v2.1.0
)
```

---

## Questions Before Implementation

1. **Authentication:**
   - Start with no auth (local only)?
   - Simple API key?
   - Full user system from the start?

2. **Multi-language support:**
   - Chinese only for MVP?
   - Include Japanese (MeCab) from the start?

3. **Deployment target:**
   - Local-only for now?
   - Design for self-hosted from the start?

4. **Miner integration timing:**
   - Build upload into Miner as part of this stage?
   - Or add in a separate "Stage 5b" PR?

---

## Implementation Order (Recommended)

1. **Week 1:** Phase 1 (Setup) + Phase 2 (Ingestion)
2. **Week 2:** Phase 3 (NLP Pipeline)
3. **Week 3:** Phase 4 (i+1 Engine)
4. **Week 4:** Phase 5 (FSRS) + Phase 6 (Polish)

Each phase is independently testable and provides incremental value.

---

Ready to start building The Brain!
