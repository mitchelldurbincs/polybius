# Polybius 🏛️

**The Context-First Sentence Mining Engine for Engineers.**

> *"Skritter and Anki are unit tests for vocabulary. Polybius is the integration test."*

## 🧐 The Problem

Most language learning apps (Skritter, Duolingo) teach vocabulary in a vacuum. You learn that `对` means "Correct," but you miss the emotion, speed, and slur of a native speaker screaming it in a movie.

## 💡 The Solution: "The Time Machine"

Polybius is a local daemon that sits in your system tray while you watch Netflix, YouTube, or play games in your target language.

1. **The Watch:** You listen to native content.
2. **The Trigger:** You hear a sentence you want to learn.
3. **The Capture:** You hit a hotkey: `Ctrl+Alt+1` (10s), `Ctrl+Alt+2` (30s), or `Ctrl+Alt+3` (60s).
4. **The Artifact:** Polybius instantly saves the audio (buffered in RAM) and a **screenshot** of the scene (with subtitles) to your library.

No manual recording. No downloading video files. Zero friction.

## 🏗️ Architecture

Polybius is designed as a distributed system to support a "Capture Locally, Review Anywhere" workflow.

```mermaid
graph TD
    A[The Miner] -->|Captures| B(Artifact)
    B -->|Audio + Screenshot + OCR| C[The Brain]
    C -->|FSRS Scheduling| D[The Gym]
    
    subgraph Client [Desktop / Laptop]
    A[The Miner (Rust)]
    end
    
    subgraph Server [Home Lab / Cloud]
    C[The Brain (Go)]
    end
    
    subgraph Review [Terminal]
    D[The Gym (TUI)]
    end

```

### 1. The Miner (Current MVP)

A high-performance Rust binary running on Windows.

* **Audio:** Uses `cpal` to tap into WASAPI Loopback. Maintains lock-free ring buffers (`ringbuf`) for 10s, 30s, and 60s durations.
* **System Tray:** Lives in your system tray with context menu for capture, pause/resume, and settings.
* **Configuration:** TOML-based config file at platform-standard location (e.g., `%APPDATA%\miner\config.toml` on Windows).
* **Vision:** Uses Windows Native OCR to extract text from screenshots alongside audio capture.
* **Performance:** Zero allocations in the hot audio loop.

### 2. The Brain (Planned)

A central API written in Go.

* **NLP:** Segments sentences (using `jieba` for Chinese) to identify "i+1" sentences (where you know all words except one).
* **SRS:** Uses the **FSRS** (Free Spaced Repetition Scheduler) algorithm to schedule reviews.

### 3. The Gym (Planned)

A Terminal User Interface (TUI) for study sessions.

* Plays the actual audio clip you mined.
* Shows the screenshot with the target word blurred.

## 🚀 Getting Started (The Miner)

### Prerequisites

* Windows 10/11 (Required for WASAPI Loopback & OCR).
* Rust (Latest Stable).

### Installation

```bash
git clone https://github.com/yourusername/polybius.git
cd polybius
cargo run --release

```

### Usage

1. Run the app. It will appear in your system tray.
2. Start watching content in your target language (YouTube, Netflix, etc.).
3. When you hear a sentence you want to mine, press:
   - **`Ctrl + Alt + 1`** — Save last 10 seconds
   - **`Ctrl + Alt + 2`** — Save last 30 seconds
   - **`Ctrl + Alt + 3`** — Save last 60 seconds
4. Check `~/Music/Miner` (or your configured save directory) for captured `.wav` files.

### Configuration

The config file is created automatically on first run at:
- **Windows:** `%APPDATA%\miner\config.toml`
- **macOS:** `~/Library/Application Support/miner/config.toml`
- **Linux:** `~/.config/miner/config.toml`

You can customize hotkeys, save directory, buffer durations, and notification settings.

## 🛠️ Tech Stack

* **Core:** Rust
* **Audio:** `cpal`, `hound`, `ringbuf`
* **System Integration:** `global-hotkey`, `windows-rs`, `tray-icon`, `winit`
* **Configuration:** `serde`, `toml`, `directories`
* **Notifications:** `notify-rust`

## 🗺️ Roadmap

* [x] **Core Audio Engine:** Ring buffer recording without priority inversion.
* [x] **Hotkeys:** Global capture triggers (10s/30s/60s).
* [x] **System Tray:** Full tray integration with context menu.
* [x] **Multi-Duration Buffers:** Configurable 10s, 30s, and 60s buffers.
* [x] **Configuration:** TOML-based config with platform-standard paths.
* [x] **Vision Module:** Screenshot & OCR integration.
* [x] **Data Structure:** JSON metadata format for captured cards.
* [ ] **The Brain:** Go backend for FSRS scheduling.
* [ ] **The Gym:** TUI review interface.

## ⚖️ License

MIT License.
