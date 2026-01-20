# Polybius

**The Context-First Sentence Mining Engine for Engineers.**

> *"Skritter and Anki are unit tests for vocabulary. Polybius is the integration test."*

## The Problem

Most language learning apps (Skritter, Duolingo) teach vocabulary in a vacuum. You learn that `对` means "Correct," but you miss the emotion, speed, and slur of a native speaker screaming it in a movie.

## The Solution: "The Time Machine"

Polybius captures the *moment* — audio, screenshot, and text — so you retain vocabulary with rich episodic context. It's a distributed system with three components:

1. **The Miner** captures audio and screenshots from native content
2. **The Brain** processes captures with NLP and schedules reviews using FSRS
3. **The Gym** provides an audio-first review experience in your terminal

No manual recording. No downloading video files. Zero friction.

---

## Table of Contents

- [Architecture](#architecture)
- [System Requirements](#system-requirements)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Workflow](#workflow)
- [Configuration](#configuration)
- [Output Files](#output-files)
- [OCR Language Setup](#ocr-language-setup)
- [Troubleshooting](#troubleshooting)
- [Tech Stack](#tech-stack)
- [Roadmap](#roadmap)
- [Development](#development)
- [License](#license)

---

## Architecture

Polybius is designed as a distributed system to support a "Capture Locally, Review Anywhere" workflow.

```
Desktop/Laptop              Home Lab/Cloud              Terminal
┌──────────────────┐      ┌─────────────────────┐    ┌──────────────┐
│   The Miner      │      │    The Brain        │    │   The Gym    │
│   (Rust, v0.4)   │  ──> │    (Go)             │ ──>│   (Go TUI)   │
│                  │      │                     │    │              │
│ • Audio capture  │      │ • File watcher      │    │ • Triage     │
│ • Screenshots    │      │ • NLP segmentation  │    │ • Review     │
│ • OCR extraction │      │ • CEDICT lookup     │    │ • Audio play │
│ • Hotkey triggers│      │ • FSRS scheduling   │    │ • 3-state    │
│ • System tray    │      │ • SQLite storage    │    │   reveal     │
└──────────────────┘      └─────────────────────┘    └──────────────┘
```

### 1. The Miner (v0.4.0 - Complete)

A high-performance Rust binary running on Windows.

* **Audio:** Uses `cpal` to tap into WASAPI Loopback. Maintains lock-free ring buffers (`ringbuf`) for 5s, 10s, and 15s durations.
* **Vision:** Captures screenshots (foreground window or full screen) and extracts text using Windows Native OCR.
* **System Tray:** Lives in your system tray with context menu for capture, pause/resume, and settings.
* **Configuration:** TOML-based config file at platform-standard location.
* **Performance:** Zero allocations in the hot audio loop.

### 2. The Brain (In Development - ~90% Complete)

A central processing engine written in Go.

* **File Watcher:** Monitors capture directory and auto-ingests new artifacts.
* **NLP Pipeline:** Segments Chinese text using GSE, looks up definitions in CC-CEDICT (~120k entries).
* **SRS Scheduling:** Uses FSRS-6 (Free Spaced Repetition Scheduler) for optimal review intervals.
* **Draft System:** New captures enter as "draft" cards until triaged to prevent queue bloat.
* **Storage:** SQLite database for cards, moments, vocabulary, and review history.

### 3. The Gym (In Development - ~80% Complete)

A Terminal User Interface (TUI) for study sessions.

* **Triage Mode:** Review draft cards and approve/delete before they enter your review queue.
* **Audio-First Review:** Cards play audio and show screenshot first, with text hidden.
* **3-State Reveal:** Press Space to reveal hanzi, then pinyin + definition.
* **Speed Control:** Playback at 0.75x, 1x, or 1.25x speed.

---

## System Requirements

| Requirement | The Miner | The Brain / Gym |
|-------------|-----------|-----------------|
| **OS** | Windows 10/11 | Windows, macOS, or Linux |
| **Runtime** | Rust 1.70+ (to build) | Go 1.24+ |
| **RAM** | ~16 MB (default buffers) | ~50 MB |
| **Disk** | ~500 KB per capture | ~50 MB (with dictionary) |

### Memory Footprint by Buffer Configuration (Miner)

| Buffer | Memory Usage |
|--------|--------------|
| 5 seconds | ~2 MB |
| 10 seconds | ~4 MB |
| 15 seconds | ~6 MB |
| **Default (5s + 10s)** | **~6 MB** |

---

## Installation

### The Miner (Rust)

```bash
# Clone the repository
git clone https://github.com/mitchelldurbincs/polybius.git
cd polybius

# Build release binary (optimized)
cargo build --release

# The binary will be at: target/release/miner.exe
```

### The Brain & Gym (Go)

```bash
cd polybius/brain

# Build the binary
go build -o bin/polybius ./cmd/polybius

# Or install to your GOPATH
go install ./cmd/polybius
```

The CC-CEDICT dictionary will be auto-downloaded on first run to `~/.polybius/cedict_ts.u8`.

---

## Quick Start

### 1. Start The Miner

```bash
./target/release/miner.exe
```

Look for the system tray icon. The Miner runs in the background, continuously buffering audio.

### 2. Watch Content & Capture

Start watching content in your target language (YouTube, Netflix, games). When you hear a sentence you want to mine:

| Hotkey | Duration | Use Case |
|--------|----------|----------|
| `Ctrl + Alt + 1` | 5 seconds | Short phrases, single words |
| `Ctrl + Alt + 2` | 10 seconds | Full sentences, dialogue |
| `Ctrl + Alt + 3` | 15 seconds | Extended context, conversations |

A notification confirms each capture. Files are saved to `~/Music/Miner` by default.

### 3. Start The Brain

```bash
./brain/bin/polybius brain --watch ~/Music/Miner
```

The Brain watches your capture directory and automatically processes new files:
- Parses the OCR text
- Segments into words
- Looks up pinyin and definitions
- Creates draft cards in the database

### 4. Review in The Gym

```bash
./brain/bin/polybius gym
```

**Triage Mode (for new captures):**
- See draft cards from recent captures
- Press `A` to approve (enters review queue)
- Press `D` to delete (removes capture)

**Review Mode:**
1. Audio plays automatically + screenshot shown
2. Hanzi text is hidden (test your listening!)
3. Press `Space` → Hanzi revealed
4. Press `Space` → Pinyin + English definition
5. Rate: `1` Again, `2` Hard, `3` Good, `4` Easy

---

## Workflow

```
┌─────────────────────────────────────────────────────────────────┐
│                        USER WORKFLOW                            │
└─────────────────────────────────────────────────────────────────┘

1. WATCH       You're watching a Chinese drama on Netflix
                        ↓
2. HEAR        "你听到了吗" — you recognize most words but not "听到"
                        ↓
3. CAPTURE     Press Ctrl+Alt+2 (10 seconds)
                        ↓
4. MINER       Saves audio.wav + screenshot.png + metadata.json
               → Notification: "Captured! 10s saved"
                        ↓
5. BRAIN       Auto-detects new file, parses OCR text
               → Segments: ["你", "听到", "了", "吗"]
               → Enriches: 听到 (tīng dào) - to hear
               → Creates DRAFT card
                        ↓
6. TRIAGE      Open Gym, see 5 new draft cards
               → Preview each, delete bad captures
               → Approve 3 good ones → enter FSRS queue
                        ↓
7. REVIEW      Next day, card is due
               → Audio plays: 你听到了吗
               → You try to recall without reading
               → Space → "你听到了吗" (Did I hear right?)
               → Space → "nǐ tīng dào le ma / Did you hear?"
               → Rate: Good (3)
                        ↓
8. FSRS        Updates card: next review in 3 days
```

---

## Configuration

### The Miner Configuration

Config file created automatically on first run.

| Platform | Path |
|----------|------|
| **Windows** | `%APPDATA%\miner\config.toml` |
| **macOS** | `~/Library/Application Support/miner/config.toml` |
| **Linux** | `~/.config/miner/config.toml` |

```toml
[general]
# Where to save captured audio, screenshots, and metadata
save_directory = "~/Music/Miner"

# Show desktop notifications on capture
notifications = true

[hotkeys]
# Hotkey format: MODIFIER+MODIFIER+KEY
# Modifiers: CTRL, ALT, SHIFT, SUPER (Windows key)
save_5s = "CTRL+ALT+1"
save_10s = "CTRL+ALT+2"
save_15s = "CTRL+ALT+3"

# Additional hotkeys
screenshot = "CTRL+ALT+S"      # Screenshot only (no audio)
region_select = "CTRL+ALT+R"   # Select custom region for capture

[audio]
# Enable/disable individual ring buffers
# Disable unused buffers to save memory
buffer_5s = true       # ~2 MB memory
buffer_10s = true      # ~4 MB memory
buffer_15s = false     # ~6 MB memory (disabled by default)

[vision]
# Master switch for all vision features
enabled = true

# Capture screenshots on save
screenshot_enabled = true

# Screenshot capture mode:
# - "screen": Entire primary monitor
# - "foreground": Currently focused window (recommended)
# - "window": Specific window by title
capture_mode = "foreground"

# Enable OCR text extraction from screenshots
ocr_enabled = true

# OCR language (BCP-47 language tag)
# Common values: "en-US", "zh-Hans", "zh-Hant", "ja", "ko"
ocr_language = "zh-Hans"

# Generate JSON metadata file alongside audio/screenshot
metadata_enabled = true
```

### Configuration Examples

**Chinese Learner (Simplified):**
```toml
[vision]
ocr_enabled = true
ocr_language = "zh-Hans"
```

**Japanese Learner:**
```toml
[vision]
ocr_enabled = true
ocr_language = "ja"
```

**Minimal Memory (~2 MB):**
```toml
[audio]
buffer_5s = true
buffer_10s = false
buffer_15s = false
```

---

## Output Files

Each capture creates three files with matching timestamps:

```
~/Music/Miner/
├── audio_1736700000.wav      # Audio recording
├── audio_1736700000.png      # Screenshot
└── audio_1736700000.json     # Metadata
```

### Audio File (.wav)

- **Format:** 16-bit PCM WAV
- **Sample Rate:** 48000 Hz
- **Channels:** Stereo (2 channels)
- **Size:** ~500 KB per 5 seconds

### Screenshot File (.png)

- **Format:** PNG (lossless)
- **Resolution:** Matches your screen/window
- **Size:** ~100-500 KB depending on content

### Metadata File (.json)

```json
{
  "version": "1.0",
  "timestamp": "2024-01-12T15:30:45Z",
  "audio": {
    "file": "audio_1736700000.wav",
    "duration_seconds": 10.0,
    "sample_rate": 48000,
    "channels": 2
  },
  "screenshot": {
    "file": "audio_1736700000.png",
    "width": 1920,
    "height": 1080
  },
  "ocr": {
    "language": "zh-Hans",
    "text": "你好世界",
    "words": [
      {
        "text": "你好",
        "bbox": [100, 200, 50, 30]
      },
      {
        "text": "世界",
        "bbox": [160, 200, 50, 30]
      }
    ]
  }
}
```

---

## OCR Language Setup

Polybius uses Windows' built-in OCR engine, which requires language packs.

### Installing Language Packs

1. Open **Windows Settings** → Time & Language → Language & Region
2. Click **Add a language** and search for your target language
3. Ensure **"Optical character recognition"** is checked during installation
4. Update `ocr_language` in your config.toml

### Supported Language Tags

| Language | Tag |
|----------|-----|
| Chinese (Simplified) | `zh-Hans` |
| Chinese (Traditional) | `zh-Hant` |
| Japanese | `ja` |
| Korean | `ko` |
| English (US) | `en-US` |
| Spanish | `es-ES` |
| French | `fr-FR` |
| German | `de-DE` |

### Verifying Installed Languages

```powershell
Get-WindowsCapability -Online | Where-Object { $_.Name -Like 'Language.OCR*' }
```

---

## Troubleshooting

### No Audio Being Captured

1. Ensure audio is playing through your default output device
2. Check that "Stereo Mix" or audio loopback is enabled in Windows Sound settings
3. Restart the application after starting audio playback

### Hotkeys Not Working

1. Check if another application registered the same hotkeys
2. Run Polybius as Administrator (some games require elevated privileges)
3. Check the console for "Hotkey conflict" messages

### OCR Not Extracting Text

1. Verify the language pack is installed (see [OCR Language Setup](#ocr-language-setup))
2. Check that `ocr_language` matches your installed language pack
3. Ensure subtitles are visible and not too small

### Brain Not Processing Files

1. Ensure the Brain is running with `--watch` pointing to your save directory
2. Check that JSON metadata files are being created (metadata_enabled = true)
3. Wait 200ms after capture (Brain debounces file detection)

### Gym Not Finding Cards

1. Verify the Brain has processed captures (check database exists: `~/.polybius/brain.db`)
2. Check triage mode for draft cards that need approval
3. Ensure cards are marked as "Active" not "Draft"

---

## Tech Stack

### The Miner (Rust)

| Category | Libraries |
|----------|-----------|
| **Audio** | `cpal` (WASAPI), `hound` (WAV), `ringbuf` (lock-free buffers) |
| **System** | `global-hotkey`, `tray-icon`, `winit`, `notify-rust` |
| **Windows** | `windows-rs` (WinRT APIs), `win-screenshot` |
| **Vision** | `image` (PNG encoding), Windows.Media.Ocr |
| **Config** | `serde`, `toml`, `directories` |

### The Brain & Gym (Go)

| Category | Libraries |
|----------|-----------|
| **NLP** | `github.com/go-ego/gse` (Chinese segmentation) |
| **SRS** | `github.com/open-spaced-repetition/go-fsrs/v3` |
| **Database** | `modernc.org/sqlite` (pure Go SQLite) |
| **TUI** | `github.com/charmbracelet/bubbletea`, `lipgloss` |
| **Audio** | `github.com/gopxl/beep/v2` |

---

## Roadmap

### Completed

- [x] **Stage 1:** Core Audio Engine - Ring buffer recording without priority inversion
- [x] **Stage 2:** Hotkeys - Global capture triggers
- [x] **Stage 3:** System Tray - Full tray integration with context menu
- [x] **Stage 3:** Multi-Duration Buffers - Configurable 5s, 10s, and 15s buffers
- [x] **Stage 4:** Vision Module - Screenshot capture & OCR integration
- [x] **Stage 4:** Metadata - JSON metadata format for captured artifacts
- [x] **Stage 4:** Region Selection - Custom capture regions with overlay UI

### In Progress

- [x] **Stage 5:** The Brain - Go backend with NLP pipeline
  - [x] File watcher for auto-ingestion
  - [x] Chinese word segmentation (GSE)
  - [x] CC-CEDICT dictionary integration
  - [x] FSRS-6 scheduling algorithm
  - [x] SQLite storage
  - [x] Draft/Active card workflow
  - [ ] REST API endpoints

- [x] **Stage 6:** The Gym - TUI review interface
  - [x] Triage mode for draft approval
  - [x] Audio playback with speed control
  - [x] 3-state reveal (audio → hanzi → pinyin)
  - [x] FSRS rating integration
  - [ ] Image window display
  - [ ] Progress analytics

### Planned

- [ ] Cross-platform audio capture (macOS, Linux)
- [ ] Cloud sync for multi-device access
- [ ] Mobile companion app

---

## Development

### Project Structure

```
polybius/
├── src/                        # The Miner (Rust)
│   ├── main.rs                 # Entry point, event loop
│   ├── audio.rs                # WASAPI loopback, ring buffers
│   ├── config.rs               # TOML configuration
│   ├── hotkeys.rs              # Global hotkey registration
│   ├── tray.rs                 # System tray icon and menu
│   ├── vision.rs               # Screenshot + OCR interface
│   ├── screenshot.rs           # Screen/window capture
│   ├── ocr.rs                  # Windows native OCR
│   ├── metadata.rs             # JSON metadata generation
│   ├── notifications.rs        # Desktop notifications
│   ├── region_overlay.rs       # Region selection UI
│   ├── clipboard.rs            # Clipboard integration
│   ├── window_utils.rs         # Window title/focus utilities
│   └── wav.rs                  # WAV file writing
│
├── brain/                      # The Brain & Gym (Go)
│   ├── cmd/polybius/main.go    # CLI entry point
│   └── internal/
│       ├── brain/
│       │   ├── service.go      # Main orchestration
│       │   ├── enricher.go     # NLP enrichment pipeline
│       │   └── watcher.go      # File system watcher
│       ├── storage/
│       │   ├── db.go           # SQLite connection
│       │   └── models.go       # Card, Moment, Vocabulary
│       ├── nlp/
│       │   ├── segmenter.go    # Chinese word segmentation
│       │   └── cedict.go       # Dictionary parsing
│       ├── fsrs/
│       │   └── scheduler.go    # FSRS-6 algorithm
│       └── gym/
│           ├── tui.go          # TUI main loop
│           ├── review.go       # Review session
│           ├── triage.go       # Draft approval
│           └── audio.go        # Audio playback
│
├── Cargo.toml                  # Rust dependencies
└── README.md                   # This file
```

### Building & Testing

**The Miner:**
```bash
# Debug build
cargo build

# Release build (optimized)
cargo build --release

# Run tests
cargo test

# Run with logging
RUST_LOG=debug cargo run
```

**The Brain & Gym:**
```bash
cd brain

# Build
go build -o bin/polybius ./cmd/polybius

# Run tests
go test ./...

# Run with verbose logging
./bin/polybius brain --verbose
```

### Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes and commit: `git commit -m "Add my feature"`
4. Push to your fork: `git push origin feature/my-feature`
5. Open a Pull Request

---

## License

MIT License.

---

## Acknowledgments

- [cpal](https://github.com/RustAudio/cpal) - Cross-platform audio I/O
- [FSRS](https://github.com/open-spaced-repetition/fsrs4anki) - Free Spaced Repetition Scheduler
- [GSE](https://github.com/go-ego/gse) - Go efficient multilingual NLP
- [Bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [CC-CEDICT](https://cc-cedict.org/) - Chinese-English dictionary
- The language learning community for inspiration
