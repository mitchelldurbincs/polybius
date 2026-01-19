# Contextual SRS for Chinese Listening Comprehension

**Date:** 2026-01-19
**Status:** Design Complete
**Target:** Stage 6 - The Brain + The Gym

## Overview

A "Contextual SRS" system where captured moments (audio + screenshot + text) are the unit of review rather than isolated flashcards. Designed for Chinese listening comprehension at the advanced beginner / early intermediate level.

### The Problem

1. **Can't catch words** - Native audio goes too fast, hard to identify what was said
2. **Nothing sticks** - Looking up words doesn't lead to retention without systematic review
3. **Existing SRS (Skritter) is for writing** - Different skill than listening comprehension

### The Solution

Instead of flashcards, review the *captured moment* itself:
- Hear a native speaker say it in context
- See the scene where it happened
- Episodic memory (the scene) anchors semantic memory (the word)

---

## The "Moment" as Learning Unit

When you press Ctrl+Alt+2 while watching Chinese content, Polybius captures a "moment":
- Last 10 seconds of audio
- Screenshot (with subtitles visible)
- OCR'd text

This moment is self-contained context - everything you need to understand and remember.

### Review Flow

1. Open The Gym (`polybius gym`)
2. See screenshot with Chinese text - target word highlighted
3. Audio plays automatically (0.75x speed default)
4. Either:
   - **Know it** → Rate 1-4 (FSRS), moment scheduled for later
   - **Don't know it** → Press Space to reveal pinyin + definition, then rate

### What Makes This Different From Anki

- Not staring at "听" on a white card trying to remember
- Hearing a native speaker say it in a sentence
- Seeing the scene, remembering "this is when the guy answered the phone"
- Context helps memory stick

---

## Chinese Processing Pipeline

Raw OCR gives characters like `你听到了吗`. For learners, that's not enough.

### Step 1: Word Segmentation

Chinese has no spaces. Sentence must be segmented:

```
你听到了吗  →  你 | 听到 | 了 | 吗
```

**Implementation:** jieba (standard Chinese segmenter) via Go backend

### Step 2: Pinyin Generation

Each word gets pinyin:

```
你 | 听到 | 了 | 吗
nǐ | tīng dào | le | ma
```

**Implementation:** CC-CEDICT lookup (~120k entries, bundled offline)

### Step 3: Definition Lookup

Each word gets English definition from CC-CEDICT:

| Word | Pinyin | Definition |
|------|--------|------------|
| 你 | nǐ | you |
| 听到 | tīng dào | to hear |
| 了 | le | (completed action particle) |
| 吗 | ma | (question particle) |

### Step 4: Vocabulary Matching

Compare against user's known vocabulary:
- Words marked "known" → displayed normally
- Words never seen or marked "unknown" → highlighted as potential targets

**All processing happens automatically on capture.** Moment is enriched and ready for review.

---

## Review & Scheduling System

### Moment Filtering (i+1 Principle)

Not every capture becomes a review item. On capture:

1. **Auto-analysis** - Segment words, check against vocabulary
2. **i+1 scoring** - Calculate % of known words
3. **Filter:**

| Known % | Action |
|---------|--------|
| 100% | Archive (too easy, no learning value) |
| 80-99% | **Review queue** (i+1 sweet spot) |
| < 80% | Backlog (too hard for now, revisit later) |

### Target Word Selection

In each reviewable moment, unknown word(s) become "targets." If a sentence has 2 unknown words, it may create 2 review items from the same moment, each focusing on a different word.

### FSRS Scheduling

FSRS (Free Spaced Repetition Scheduler) - latest spec - schedules reviews:

| Rating | Meaning | Effect |
|--------|---------|--------|
| 1 (Again) | Didn't know | See again in minutes |
| 2 (Hard) | Struggled | Short interval |
| 3 (Good) | Knew it | Normal interval |
| 4 (Easy) | Instant recall | Longer interval |

**Note:** Use the most up-to-date FSRS specification (FSRS-5 or later) when implementing.

### Vocabulary Feedback Loop

When you rate a moment "Good" or "Easy" 3+ times, the target word gets marked "known" in your vocabulary. This means:
- Future captures with that word won't count it as unknown
- Old backlog items might now qualify as i+1 (because you learned a word)

---

## The Gym - TUI Review Interface

### Interface Layout

```
┌─────────────────────────────────────────────────────────────┐
│  POLYBIUS GYM                          Due: 12  │  New: 3   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Sentence: 你 听到 了 吗                                     │
│                ^^^^                                         │
│  Target: 听到                                                │
│  Pinyin: [hidden]                                           │
│  Meaning: [hidden]                                          │
│                                                             │
│  ───────────────────────────────────────────────────────    │
│  Audio: [▶ Playing... 0:03/0:10]  Speed: 0.75x              │
│  ───────────────────────────────────────────────────────    │
│                                                             │
│  [Space] Reveal  [R] Replay  [S] Slower  [F] Faster         │
│  [1] Again  [2] Hard  [3] Good  [4] Easy      [Q] Quit      │
└─────────────────────────────────────────────────────────────┘

+ Separate image window showing screenshot (auto-updates per card)
```

### Key Bindings

| Key | Action |
|-----|--------|
| Space | Reveal pinyin + definition |
| R | Replay audio from start |
| S | Slower playback (0.5x → 0.75x → 1.0x) |
| F | Faster playback (1.0x → 1.25x → 1.5x) |
| ← → | Scrub through audio |
| 1-4 | FSRS rating (after reveal or if known) |
| Q | Quit review session |

### Hybrid Display Approach

TUI handles text and controls. Screenshot displays in a **separate persistent window** that auto-updates when moving to the next card.

**Rationale:** Terminal image support is fragmented. Hybrid approach is reliable across all terminals and shows images at full quality.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        EXISTING (The Miner)                      │
├─────────────────────────────────────────────────────────────────┤
│  Rust daemon → Captures audio + screenshot + OCR on hotkey      │
│  Output: ~/Music/Miner/audio_*.{wav,png,json}                   │
└──────────────────────────┬──────────────────────────────────────┘
                           │ raw captures
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                        NEW (The Brain)                           │
├─────────────────────────────────────────────────────────────────┤
│  Go service (runs locally)                                       │
│                                                                  │
│  • Watches ~/Music/Miner for new captures                       │
│  • Enriches with: segmentation, pinyin, definitions             │
│  • Scores i+1 against vocabulary                                │
│  • Stores in SQLite: moments, vocabulary, FSRS state            │
│  • REST API for The Gym                                         │
└──────────────────────────┬──────────────────────────────────────┘
                           │ enriched moments
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                        NEW (The Gym)                             │
├─────────────────────────────────────────────────────────────────┤
│  Go TUI (same binary, different mode)                           │
│                                                                  │
│  • Queries Brain for due reviews                                │
│  • Displays: TUI controls + separate image window               │
│  • Plays audio (system audio)                                   │
│  • Sends ratings back to Brain                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | Matches Stage 5 plan, good for CLI/TUI |
| Binary | Single | `polybius brain` and `polybius gym` modes |
| Database | SQLite | Simple, portable, no server setup |
| Ingestion | File watcher | Auto-imports new captures |
| Dictionary | CC-CEDICT bundled | Offline-first, ~120k entries |
| Online | Optional enhancement | Works offline, can enhance when connected |

---

## Database Schema

### Tables

```sql
-- Captured moments with enrichment
CREATE TABLE moments (
    id INTEGER PRIMARY KEY,
    timestamp TEXT NOT NULL,
    audio_file TEXT NOT NULL,
    screenshot_file TEXT NOT NULL,
    raw_text TEXT,
    segmented_text TEXT,  -- JSON array of words
    i1_score REAL,        -- % of known words
    status TEXT DEFAULT 'pending',  -- pending, active, archived
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Individual review items (one per target word per moment)
CREATE TABLE cards (
    id INTEGER PRIMARY KEY,
    moment_id INTEGER REFERENCES moments(id),
    target_word TEXT NOT NULL,
    target_pinyin TEXT,
    target_definition TEXT,
    -- FSRS fields
    stability REAL DEFAULT 0,
    difficulty REAL DEFAULT 0,
    due_date TEXT,
    last_review TEXT,
    reps INTEGER DEFAULT 0,
    lapses INTEGER DEFAULT 0,
    state TEXT DEFAULT 'new',  -- new, learning, review, relearning
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- User's vocabulary knowledge
CREATE TABLE vocabulary (
    word TEXT PRIMARY KEY,
    pinyin TEXT,
    definition TEXT,
    status TEXT DEFAULT 'unknown',  -- unknown, learning, known
    times_seen INTEGER DEFAULT 0,
    times_correct INTEGER DEFAULT 0,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Review history for analytics
CREATE TABLE reviews (
    id INTEGER PRIMARY KEY,
    card_id INTEGER REFERENCES cards(id),
    rating INTEGER NOT NULL,  -- 1-4
    time_taken_ms INTEGER,
    reviewed_at TEXT DEFAULT CURRENT_TIMESTAMP
);
```

---

## Data Flow

1. **Capture:** User presses Ctrl+Alt+2 while watching content
   - Miner saves: `audio_1737312000.wav`, `.png`, `.json`

2. **Ingest:** Brain detects new file via file watcher
   - Reads OCR text from JSON
   - Segments with jieba
   - Looks up pinyin/definitions in CC-CEDICT
   - Calculates i+1 score against vocabulary
   - Stores enriched moment in SQLite
   - Creates card(s) for unknown target word(s)

3. **Review:** User runs `polybius gym`
   - Queries Brain API for due cards
   - Displays TUI + image window
   - Plays audio
   - User rates 1-4

4. **Update:** Brain processes rating
   - FSRS calculates next due date
   - Updates card state
   - If word mastered (3+ Good/Easy), marks as "known" in vocabulary
   - Re-scores backlog moments (some may now be i+1)

---

## Implementation Phases

### Phase 1: The Brain Core
- [ ] Go project setup with SQLite
- [ ] CC-CEDICT parser and bundling
- [ ] Jieba integration for segmentation
- [ ] File watcher for ~/Music/Miner
- [ ] Moment ingestion and enrichment pipeline
- [ ] Basic vocabulary tracking

### Phase 2: FSRS Integration
- [ ] Research latest FSRS spec (FSRS-5)
- [ ] Implement FSRS scheduler
- [ ] Card creation from moments
- [ ] i+1 scoring and filtering
- [ ] Due card queries

### Phase 3: The Gym TUI
- [ ] TUI framework setup (bubbletea or similar)
- [ ] Review interface layout
- [ ] Audio playback with speed control
- [ ] Image window (separate process)
- [ ] Keyboard navigation
- [ ] Rating submission

### Phase 4: Polish
- [ ] Vocabulary feedback loop (learning → known)
- [ ] Backlog re-scoring when vocabulary changes
- [ ] Statistics and progress tracking
- [ ] Config file for preferences

---

## Future Considerations (Not in Scope)

- Multi-language support (Japanese, Korean)
- Cloud sync
- Mobile companion app
- Skritter/Anki export
- Tone visualization / pronunciation training

---

## Summary

**What this gives the user:**

| Problem | Solution |
|---------|----------|
| Can't catch words | Replay audio at slower speeds, unlimited times |
| Nothing sticks | Context-rich review with FSRS scheduling |
| SRS overload | Separate system for listening (doesn't add to Skritter) |

**The key insight:** The captured moment IS the flashcard. Rich context helps memory stick better than isolated vocabulary drilling.
