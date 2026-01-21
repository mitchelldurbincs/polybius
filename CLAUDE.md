# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Polybius is a context-first sentence mining engine for language learners, focused on Chinese. It captures moments (audio + screenshot + OCR) from native content to enable vocabulary retention with rich episodic context.

**Three-Component Architecture:**
- **The Miner** (Rust) - Captures audio and screenshots from native content
- **The Brain** (Go) - Processes captures with NLP and schedules reviews using FSRS-6
- **The Gym** (Go TUI) - Terminal UI for audio-first spaced repetition review

## DO's and DONT's (PAY ATTENTION)
**MAKE SURE TO TEST YOUR CHANGES / MAKE SURE YOU DIDN'T BREAK ANYTHING PLEASE :P**
**Use quotations since this is PS - like so - cd "C:/Users/mitchell.durbin/source/repos/polybius/miner"**

## Build and Development Commands

### The Miner (Rust)
```bash
cargo build                      # Debug build
cargo build --release            # Optimized release build
cargo test                       # Run tests
RUST_LOG=debug cargo run         # Run with debug logging
```

### The Brain & Gym (Go)
```bash
cd brain
make build        # Build to bin/polybius
make run-brain    # Build and run brain (file watcher + NLP processing)
make run-gym      # Build and run gym (TUI review interface)
make test         # Run all Go tests (go test ./...)
make clean        # Clean build artifacts
```

### Running Individual Tests
```bash
# Rust
cargo test test_name

# Go
go test ./internal/nlp -run TestSegmenter
go test ./internal/fsrs -run TestScheduler
```

### Vocabulary Management
```bash
# Import known vocabulary from Skritter TSV export
polybius vocab import skritter-export.tsv
```

Imported words are marked as "known" in the database, preventing card creation for words you've already learned elsewhere.

## Architecture

### Data Flow
```
Miner (hotkey capture) → ~/Music/Miner/*.{wav,png,json}
                              ↓
Brain (file watcher) → SQLite DB (~/.polybius/brain.db)
                              ↓
Gym (TUI) → Triage drafts → Review with FSRS scheduling
```

### The Miner (miner/src/)
- `audio.rs` - WASAPI loopback capture with lock-free ring buffers (5s/10s/15s)
- `hotkeys.rs` - Global hotkeys (Ctrl+Alt+1/2/3 for audio, Ctrl+Alt+S screenshot, Ctrl+Alt+R region)
- `region_overlay.rs` - Custom region selection UI with pre-rendered darkened bitmap
- `vision.rs` - Screenshot capture + Windows native OCR integration
- `config.rs` - TOML configuration at platform-standard location

### The Brain (brain/internal/)
- `brain/` - Service orchestration: file watcher, enricher pipeline
- `nlp/` - Chinese segmentation (GSE) and CC-CEDICT dictionary (~120k entries)
- `fsrs/` - FSRS-6 spaced repetition scheduling
- `storage/` - SQLite operations, Card/Moment/Vocabulary models
- `gym/` - Bubbletea TUI: triage mode, review mode, audio playback
- `vocab/` - Vocabulary import from external sources (Skritter TSV)

### Key Data Models
- **Moment** - Captured artifact (audio, screenshot, OCR text, segmented words, i+1 score)
- **Card** - Flashcard with FSRS state (stability, difficulty, due date)
- **Vocabulary** - Known words tracking for i+1 calculation

## Configuration

### Miner Config Location
- Windows: `%APPDATA%\miner\config.toml`
- macOS: `~/Library/Application Support/miner/config.toml`
- Linux: `~/.config/miner/config.toml`

### Brain Paths (hardcoded)
- Database: `~/.polybius/brain.db`
- Dictionary: `~/.polybius/cedict_ts.u8` (auto-downloaded)
- Watch directory: `~/Music/Miner`

## Output Format

Each capture creates three files in `~/Music/Miner/`:
```
audio_1736700000.wav   # 16-bit PCM, 48kHz, Stereo
audio_1736700000.png   # Screenshot
audio_1736700000.json  # Metadata with OCR text and word bounding boxes
```

## Key Patterns

- **Ring Buffer Pattern** - Lock-free audio buffering for zero-allocation capture
- **Draft/Active Workflow** - New cards enter as drafts, require triage approval
- **i+1 Learning** - Targets sentences where user knows all words except one
- **Audio-First Review** - Audio plays first, screenshot shown, text revealed in stages
