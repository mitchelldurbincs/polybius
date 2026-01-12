# Vision Module Plan

## Overview

The Vision Module extends The Miner to capture **screenshots** and extract **text via OCR** alongside audio. When a user presses a capture hotkey, they'll get:

1. `audio_1736700000.wav` — The audio buffer (existing)
2. `audio_1736700000.png` — Screenshot of the current screen
3. `audio_1736700000.json` — Metadata including OCR text (future)

This completes the "Artifact" capture flow described in the architecture.

---

## Goals

- **Zero additional friction** — Screenshot capture happens automatically with audio
- **Fast** — Screenshot + OCR should add < 200ms to save operation
- **Configurable** — Users can disable screenshot/OCR if they only want audio
- **Windows-first** — Use native Windows APIs for best performance and OCR quality

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                       handle_save()                         │
│                                                             │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐       │
│  │ Audio Save  │   │ Screenshot  │   │  OCR Text   │       │
│  │  (existing) │ → │   Capture   │ → │  Extraction │       │
│  └─────────────┘   └─────────────┘   └─────────────┘       │
│                           │                 │               │
│                           ▼                 ▼               │
│                    screenshot.rs       ocr.rs               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### New Files

| File | Purpose |
|------|---------|
| `src/screenshot.rs` | Screen/window capture using Windows Graphics Capture API |
| `src/ocr.rs` | Text extraction using Windows.Media.Ocr |
| `src/vision.rs` | Unified interface combining screenshot + OCR |

---

## Implementation Phases

### Phase 1: Screenshot Capture

**Goal:** Capture the screen when a hotkey is pressed and save as PNG alongside audio.

#### Approach Options

| Option | Crate | Pros | Cons |
|--------|-------|------|------|
| A. `screenshots` | `screenshots` | Cross-platform, simple API | Additional dependency |
| B. `win-screenshot` | `win-screenshot` | Windows-optimized, can capture specific windows | Windows-only |
| C. Native `windows-rs` | `windows` (existing) | No new deps, full control | More code to write |

**Recommendation:** Option B (`win-screenshot`) for Windows builds, with Option A as fallback for cross-platform testing.

#### Implementation Steps

1. Add dependencies to `Cargo.toml`:
   ```toml
   [target.'cfg(windows)'.dependencies]
   win-screenshot = "4"

   [target.'cfg(not(windows))'.dependencies]
   screenshots = "0.8"
   ```

2. Create `src/screenshot.rs`:
   ```rust
   pub struct Screenshot {
       pub width: u32,
       pub height: u32,
       pub data: Vec<u8>,  // RGBA pixels
   }

   impl Screenshot {
       /// Capture the primary monitor
       pub fn capture_screen() -> Result<Self, ScreenshotError>;

       /// Capture a specific window by title (optional enhancement)
       pub fn capture_window(title: &str) -> Result<Self, ScreenshotError>;

       /// Save to PNG file
       pub fn save_png(&self, path: &Path) -> Result<(), std::io::Error>;
   }
   ```

3. Integrate into `handle_save()` in `main.rs`:
   ```rust
   // After saving audio...
   if config.vision.screenshot_enabled {
       let screenshot = Screenshot::capture_screen()?;
       let screenshot_path = save_dir.join(format!("audio_{}.png", timestamp));
       screenshot.save_png(&screenshot_path)?;
   }
   ```

#### Config Additions

```toml
[vision]
# Enable screenshot capture
screenshot_enabled = true

# Capture mode: "screen" (full screen) or "window" (active window)
capture_mode = "screen"

# Target window title pattern (only used if capture_mode = "window")
# window_pattern = "Netflix"
```

---

### Phase 2: Windows OCR Integration

**Goal:** Extract text from screenshots using Windows native OCR.

#### Windows.Media.Ocr API

Windows 10+ includes a built-in OCR engine accessible via WinRT. Key types:

- `OcrEngine` — The main OCR processor
- `SoftwareBitmap` — Image format required by OCR engine
- `OcrResult` — Contains recognized text and word bounding boxes

#### Implementation Steps

1. Extend `windows-rs` features in `Cargo.toml`:
   ```toml
   [target.'cfg(windows)'.dependencies]
   windows = { version = "0.58", features = [
       "Win32_System_Console",
       "Graphics_Imaging",
       "Media_Ocr",
       "Globalization",
       "Storage_Streams",
   ] }
   ```

2. Create `src/ocr.rs`:
   ```rust
   use windows::Globalization::Language;
   use windows::Media::Ocr::OcrEngine;

   pub struct OcrResult {
       pub text: String,
       pub words: Vec<OcrWord>,
       pub language: String,
   }

   pub struct OcrWord {
       pub text: String,
       pub bounding_box: (f64, f64, f64, f64),  // x, y, width, height
       pub confidence: Option<f32>,
   }

   pub struct OcrProcessor {
       engine: OcrEngine,
   }

   impl OcrProcessor {
       /// Create OCR processor for a specific language
       pub fn new(language_tag: &str) -> Result<Self, OcrError>;

       /// Create OCR processor that auto-detects language
       pub fn auto_detect() -> Result<Self, OcrError>;

       /// Extract text from screenshot
       pub fn process(&self, screenshot: &Screenshot) -> Result<OcrResult, OcrError>;
   }
   ```

3. Language support considerations:
   - Chinese (Simplified): `zh-Hans`
   - Chinese (Traditional): `zh-Hant`
   - Japanese: `ja`
   - Korean: `ko`
   - User must have the language pack installed on Windows

#### Config Additions

```toml
[vision]
# Enable OCR text extraction
ocr_enabled = true

# OCR language (BCP-47 tag). Use "auto" for auto-detection.
# Common values: "zh-Hans", "zh-Hant", "ja", "ko", "en-US"
ocr_language = "zh-Hans"
```

---

### Phase 3: Unified Vision Module

**Goal:** Clean interface that combines screenshot + OCR with proper error handling.

#### Create `src/vision.rs`:

```rust
pub struct VisionCapture {
    screenshot_enabled: bool,
    ocr_enabled: bool,
    ocr_processor: Option<OcrProcessor>,
    capture_mode: CaptureMode,
}

pub enum CaptureMode {
    Screen,
    Window(String),
}

pub struct CaptureResult {
    pub screenshot: Option<Screenshot>,
    pub ocr_result: Option<OcrResult>,
}

impl VisionCapture {
    pub fn new(config: &VisionConfig) -> Result<Self, VisionError>;

    /// Capture screenshot and optionally run OCR
    pub fn capture(&self) -> Result<CaptureResult, VisionError>;

    /// Save results to files
    pub fn save(&self, result: &CaptureResult, base_path: &Path) -> Result<(), std::io::Error>;
}
```

#### Integration in `main.rs`:

```rust
fn handle_save(
    audio: &mut AudioCapture,
    vision: &VisionCapture,  // NEW
    duration: BufferDuration,
    save_dir: &PathBuf,
    show_notification: bool,
) -> Result<(), Box<dyn std::error::Error>> {
    let timestamp = generate_timestamp();

    // 1. Save audio (existing)
    let audio_path = save_dir.join(format!("audio_{}.wav", timestamp));
    // ... existing audio save code ...

    // 2. Capture and save vision (NEW)
    if let Ok(capture) = vision.capture() {
        let base_path = save_dir.join(format!("audio_{}", timestamp));
        vision.save(&capture, &base_path)?;
    }

    Ok(())
}
```

---

### Phase 4: JSON Metadata (Foundation for The Brain)

**Goal:** Output structured metadata that The Brain can ingest.

#### Metadata Schema

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

#### Implementation

Create `src/metadata.rs`:

```rust
#[derive(Serialize)]
pub struct CardMetadata {
    pub version: String,
    pub timestamp: String,
    pub audio: AudioMetadata,
    pub screenshot: Option<ScreenshotMetadata>,
    pub ocr: Option<OcrMetadata>,
}

impl CardMetadata {
    pub fn save(&self, path: &Path) -> Result<(), std::io::Error> {
        let json = serde_json::to_string_pretty(self)?;
        fs::write(path, json)?;
        Ok(())
    }
}
```

---

## Dependency Summary

```toml
# Add to Cargo.toml

[dependencies]
image = "0.25"           # PNG encoding
serde_json = "1"         # JSON metadata

[target.'cfg(windows)'.dependencies]
win-screenshot = "4"
windows = { version = "0.58", features = [
    "Win32_System_Console",
    "Graphics_Imaging",
    "Media_Ocr",
    "Globalization",
    "Storage_Streams",
] }
```

---

## Config Schema (Complete)

```toml
[general]
save_directory = "~/Music/Miner"
notifications = true

[hotkeys]
save_10s = "CTRL+ALT+1"
save_30s = "CTRL+ALT+2"
save_60s = "CTRL+ALT+3"

[audio]
buffer_10s = true
buffer_30s = true
buffer_60s = false

[vision]
# Master toggle for all vision features
enabled = true

# Screenshot capture
screenshot_enabled = true
capture_mode = "screen"  # "screen" or "window"
# window_pattern = "Netflix"  # Only if capture_mode = "window"

# OCR settings
ocr_enabled = true
ocr_language = "zh-Hans"  # BCP-47 tag, or "auto"

# JSON metadata output
metadata_enabled = true
```

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| OCR language pack not installed | Graceful fallback: skip OCR, log warning, still save screenshot |
| Screenshot capture fails (permissions, etc.) | Continue with audio-only save, notify user |
| OCR is slow (> 500ms) | Run OCR async, don't block audio save |
| Large screenshots use too much memory | Optionally downscale before OCR (configurable) |

---

## Testing Plan

1. **Unit tests** for screenshot capture on Windows
2. **Unit tests** for OCR with test images containing known text
3. **Integration test**: Full capture flow (audio + screenshot + OCR + metadata)
4. **Manual testing** with Netflix, YouTube, VLC

---

## Future Enhancements (Not in Scope)

- [ ] Active window detection (auto-capture the video player window)
- [ ] Subtitle region detection (crop to subtitle area for faster/better OCR)
- [ ] GPU-accelerated screenshot capture
- [ ] macOS/Linux OCR support (Tesseract fallback)

---

## Implementation Order

1. **Phase 1: Screenshot** — Get screenshots working alongside audio
2. **Phase 2: Config** — Add `[vision]` config section
3. **Phase 3: OCR** — Integrate Windows OCR
4. **Phase 4: Metadata** — JSON output for The Brain
5. **Phase 5: Polish** — Error handling, notifications, edge cases

Each phase is independently shippable and provides incremental value.
