# The Brain - Architecture Guide

This document explains how The Brain component works for developers new to the Polybius codebase.

## Overview

The Brain is a Go service that processes captures from the Miner and manages spaced repetition scheduling. It has two main responsibilities:

1. **File Watching** - Detects new captures and processes them through the NLP pipeline
2. **FSRS Scheduling** - Manages spaced repetition for vocabulary review

## Entry Points

The Brain is invoked via `brain/cmd/polybius/main.go`:

```bash
polybius brain   # Start the file watcher service
polybius gym     # Start the TUI review interface
```

## Core Components

### Service (`brain/internal/brain/service.go`)

The main service composes all subsystems:

```go
type Service struct {
    db        *storage.DB      // SQLite database
    enricher  *Enricher        // NLP pipeline
    scheduler *fsrs.Scheduler  // Spaced repetition
    watcher   *Watcher         // File monitor
    minerDir  string
}
```

### File Watcher (`brain/internal/brain/watcher.go`)

Uses `fsnotify` to monitor `~/Music/Miner/` for new captures:

- Watches for file creation events
- Filters for `.json` metadata files (ignores `.wav` and `.png`)
- Triggers `handleNewCapture()` callback when metadata arrives
- Expects companion files with the same basename: `audio_1736700000.{json,wav,png}`

### Enricher (`brain/internal/brain/enricher.go`)

Processes captured text through the NLP pipeline:

```
Raw OCR Text
    |
    v
Segmentation (GSE Chinese word segmentation)
    |
    v
Dictionary Lookup (CEDICT - 120k+ entries)
    |
    v
Vocabulary Classification (Known vs Unknown words)
    |
    v
i+1 Score Calculation
```

**Key Methods:**

- `Enrich(text)` - Segments text, looks up words in dictionary, returns enriched words with pinyin and definitions
- `EnrichWithVocab(text, isKnown)` - Classifies words as known/unknown and calculates i+1 score

### NLP (`brain/internal/nlp/`)

**Segmenter (`segmenter.go`):**
- Uses GSE (Go Search Engine) library for Chinese word segmentation
- Two modes: accurate (`Cut`) and search (`CutSearch`)

**Dictionary (`cedict.go`):**
- Loads CC-CEDICT open-source Chinese-English dictionary
- ~120k simplified to traditional character mappings
- `LookupWithFallback()` tries exact match, then character-by-character fallback

### FSRS Scheduler (`brain/internal/fsrs/scheduler.go`)

Implements FSRS-6 (Free Spaced Repetition Scheduler):

**Review Ratings:**
- `1 = Again` - Failed, reschedule sooner
- `2 = Hard` - Difficult, moderate interval
- `3 = Good` - Correct, standard interval
- `4 = Easy` - Very easy, longer interval

**Card State Tracking:**
```go
CardState {
    Stability   // Memory strength [0.0, infinity)
    Difficulty  // Card difficulty [0.0, 10.0]
    Reps        // Review count
    Lapses      // Forgetting count
    State       // 0=New, 1=Learning, 2=Review, 3=Relearning
    LastReview
}
```

### Storage (`brain/internal/storage/`)

SQLite database with four main tables:

| Table | Purpose |
|-------|---------|
| `moments` | Captured content (audio, screenshot, OCR text, i+1 score) |
| `cards` | Flashcards with FSRS state and scheduling |
| `vocabulary` | User's known/unknown words |
| `reviews` | Review history audit trail |

## Data Flow

### End-to-End Pipeline

```
MINER CAPTURE
    |
    v
~/Music/Miner/audio_1736700000.{json, wav, png}
    |
    v
BRAIN (File Watcher)
    |
    v
Parse JSON metadata
    |
    v
ENRICHMENT PIPELINE
  1. Segment Chinese text (GSE)
  2. Lookup words in CEDICT
  3. Compare against Vocabulary DB
  4. Calculate i+1 score
    |
    v
CREATE DATABASE RECORDS
  - Moment (raw text, file paths, i+1 score)
  - Vocabulary entries (unknown words)
  - Cards (state="draft", no scheduling yet)
    |
    v
GYM (Triage Phase)
  - User reviews draft cards
  - Approve or reject
  - Approved cards: state="new", due_date set
    |
    v
FSRS SCHEDULING
  - Due cards queued for review
  - User submits ratings (1-4)
  - Scheduler recalculates stability, difficulty, next due date
    |
    v
DATABASE UPDATE
  - Card state advanced
  - Ready for next review cycle
```

### Capture Processing (`handleNewCapture`)

When a new `.json` file is detected:

1. Wait 200ms (handles race condition with Miner file writes)
2. Parse JSON metadata (OCR text, audio path, screenshot path)
3. Create Moment record
4. Enrich with vocabulary - identify unknown words
5. Upsert vocabulary entries
6. Create draft Cards for each unknown word

## i+1 Learning Theory

The enricher calculates an **i+1 score** = known words / total words.

- **Ideal (i+1):** Exactly 1 unknown word - perfect learning condition
- **Learnable:** 80-99% known words
- **Too hard:** <80% known words - cognitive overload

This score helps prioritize sentences where the learner knows enough context to acquire new vocabulary naturally.

## Card State Machine

Cards progress through states based on FSRS scheduling:

```
draft --> new --> learning --> review <--> relearning
```

- **draft** - Newly created, awaiting triage approval
- **new** - Approved, ready for first review
- **learning** - Initial learning phase
- **review** - Graduated to regular review schedule
- **relearning** - Failed review, re-entering learning

## Key Files Reference

| File | Purpose |
|------|---------|
| `brain/cmd/polybius/main.go` | CLI entry point |
| `brain/internal/brain/service.go` | Service orchestration |
| `brain/internal/brain/watcher.go` | File system monitoring |
| `brain/internal/brain/enricher.go` | NLP pipeline coordination |
| `brain/internal/nlp/segmenter.go` | Chinese word segmentation |
| `brain/internal/nlp/cedict.go` | Dictionary loading and lookup |
| `brain/internal/fsrs/scheduler.go` | Spaced repetition scheduling |
| `brain/internal/storage/db.go` | Database operations |
| `brain/internal/storage/models.go` | Data models |
| `brain/internal/gym/review.go` | Review session logic |

## Configuration

- **Database:** `~/.polybius/brain.db`
- **Dictionary:** `~/.polybius/cedict_ts.u8` (auto-downloaded on first run)
- **Watch directory:** `~/Music/Miner`
