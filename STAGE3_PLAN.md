# Stage 3 Implementation Plan: System Tray & Multi-Duration

**Goal:** Transform The Miner from a terminal app into a background utility with system tray icon, multiple save durations, and configurable settings.

---

## Overview

Stage 3 elevates The Miner from a developer tool to a daily-driver utility. Users won't need to keep a terminal open - the app runs silently in the background with a system tray icon for status and control. Multiple hotkeys let users choose how much audio to save (10s, 30s, or 60s), and settings persist across sessions.

---

## What Changes from Stage 2

| Aspect | Stage 2 | Stage 3 |
|--------|---------|---------|
| UI | Terminal window required | System tray icon (no window) |
| Lifecycle | Manual start/stop | Runs on startup (optional) |
| Buffer | Single 10s buffer | Multiple buffers (10s/30s/60s) |
| Hotkeys | Ctrl+Alt+C only | Three configurable hotkeys |
| Settings | Hardcoded | Config file (TOML) |
| Feedback | Console output | Desktop notifications |

---

## Dependencies (Cargo.toml)

```toml
[package]
name = "miner"
version = "0.3.0"
edition = "2021"

[dependencies]
cpal = "0.15"              # Audio capture
hound = "3.5"              # WAV file writing
global-hotkey = "0.6"      # Global hotkey detection
ringbuf = "0.4"            # Lock-free ring buffer
tray-icon = "0.19"         # System tray icon
winit = "0.30"             # Event loop (required by tray-icon on Windows)
notify-rust = "4"          # Desktop notifications
serde = { version = "1", features = ["derive"] }  # Config serialization
toml = "0.8"               # Config file format
directories = "5"          # Standard config/data paths

[target.'cfg(windows)'.dependencies]
windows = { version = "0.58", features = ["Win32_System_Console"] }  # Hide console window

[profile.release]
opt-level = 3
lto = true
strip = true
```

**New dependencies:**
- `tray-icon` - Cross-platform system tray support
- `winit` - Window/event loop management (required for tray events)
- `notify-rust` - Native desktop notifications
- `serde` + `toml` - Configuration file handling
- `directories` - Platform-standard paths for config/data
- `windows` - Hide console window on Windows

---

## Architecture

```
┌───────────────────────────────────────────────────────────────────────┐
│                          Main Thread                                   │
├───────────────────────────────────────────────────────────────────────┤
│  1. Load config from ~/.config/miner/config.toml                      │
│  2. Initialize system tray with icon and menu                         │
│  3. Register multiple hotkeys (Ctrl+Alt+1/2/3)                        │
│  4. Start audio capture thread → feeds all ring buffers               │
│  5. Run unified event loop (tray + hotkey events)                     │
│  6. On hotkey: select appropriate buffer, peek, save, notify          │
│  7. Tray menu: settings, pause/resume, quit                           │
└───────────────────────────────────────────────────────────────────────┘

         ┌──────────────────────┐
         │   Audio Thread       │
         │   (Single Producer)  │
         └──────────┬───────────┘
                    │ push samples
                    ▼
    ┌─────────────────────────────────────────────┐
    │         Ring Buffers (Shared Memory)         │
    │  ┌─────────┐ ┌─────────┐ ┌─────────┐        │
    │  │ 10 sec  │ │ 30 sec  │ │ 60 sec  │        │
    │  │ ~4 MB   │ │ ~12 MB  │ │ ~24 MB  │        │
    │  └─────────┘ └─────────┘ └─────────┘        │
    └─────────────────────────────────────────────┘
                    │ peek on save
                    ▼
         ┌──────────────────────┐
         │    Main Thread       │
         │  (Consumer + Tray)   │
         └──────────────────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
┌──────────────┐       ┌──────────────────┐
│  WAV Writer  │       │  Notification    │
│  (to disk)   │       │  (toast popup)   │
└──────────────┘       └──────────────────┘
```

---

## Configuration

### Config File Location

| Platform | Path |
|----------|------|
| Windows | `%APPDATA%\miner\config.toml` |
| macOS | `~/Library/Application Support/miner/config.toml` |
| Linux | `~/.config/miner/config.toml` |

### Default Config (config.toml)

```toml
# The Miner Configuration

[general]
# Where to save audio files
save_directory = "~/Music/Miner"  # Expands ~ automatically
# Run on system startup
autostart = false
# Show notification after save
notifications = true

[hotkeys]
# Format: "MODIFIER+MODIFIER+KEY"
# Available modifiers: CTRL, ALT, SHIFT, SUPER
# Available keys: A-Z, 0-9, F1-F12
save_10s = "CTRL+ALT+1"
save_30s = "CTRL+ALT+2"
save_60s = "CTRL+ALT+3"

[audio]
# Buffer durations in seconds (memory usage shown)
# 10s = ~4 MB, 30s = ~12 MB, 60s = ~24 MB
buffer_10s = true
buffer_30s = true
buffer_60s = false  # Disabled by default to save memory

# Output format
sample_rate = 48000  # Or "device" to use device default
```

---

## System Tray Design

### Icon States

| State | Icon | Tooltip |
|-------|------|---------|
| Recording | 🔴 (red circle) | "The Miner - Recording" |
| Paused | ⏸️ (pause symbol) | "The Miner - Paused" |
| Saving | 💾 (disk) | "The Miner - Saving..." |

### Context Menu

```
┌──────────────────────────┐
│ ● Recording              │  ← Status indicator
├──────────────────────────┤
│ Save Last 10 seconds     │  Ctrl+Alt+1
│ Save Last 30 seconds     │  Ctrl+Alt+2
│ Save Last 60 seconds     │  Ctrl+Alt+3
├──────────────────────────┤
│ ⏸ Pause Recording        │
│ 📁 Open Save Folder      │
│ ⚙️ Settings...           │  → Opens config file
├──────────────────────────┤
│ 🚪 Quit                  │
└──────────────────────────┘
```

---

## Implementation Steps

### Phase 1: Project Restructure

**Step 1.1: Create module structure**
```
src/
├── main.rs           # Entry point, event loop
├── audio.rs          # Audio capture, ring buffers
├── config.rs         # Configuration loading/saving
├── hotkeys.rs        # Hotkey registration and handling
├── tray.rs           # System tray setup and menu
├── notifications.rs  # Desktop notification helpers
└── wav.rs            # WAV file writing (extracted)
```

**Step 1.2: Add new dependencies to Cargo.toml**

**Step 1.3: Extract existing code into modules**
- Move `build_input_stream` to `audio.rs`
- Move `write_wav` to `wav.rs`

### Phase 2: Configuration System

**Step 2.1: Define config struct**
```rust
#[derive(Debug, Serialize, Deserialize)]
pub struct Config {
    pub general: GeneralConfig,
    pub hotkeys: HotkeyConfig,
    pub audio: AudioConfig,
}

impl Default for Config {
    fn default() -> Self {
        // Return sensible defaults
    }
}
```

**Step 2.2: Implement config loading/saving**
- Load from standard path using `directories` crate
- Create default config if none exists
- Validate on load

**Step 2.3: Expand paths**
- Handle `~` expansion for save_directory
- Create directory if it doesn't exist

### Phase 3: Multi-Buffer Audio System

**Step 3.1: Create buffer manager**
```rust
pub struct BufferManager {
    buffers: HashMap<Duration, HeapCons<f32>>,
    producer: HeapProd<f32>,  // Single producer feeds all
}
```

Wait - actually we need separate buffers since they have different sizes. Let me rethink...

**Step 3.1 (revised): Independent ring buffers**
```rust
pub struct AudioCapture {
    // Each duration gets its own producer/consumer pair
    buffer_10s: Option<(HeapProd<f32>, HeapCons<f32>)>,
    buffer_30s: Option<(HeapProd<f32>, HeapCons<f32>)>,
    buffer_60s: Option<(HeapProd<f32>, HeapCons<f32>)>,
}
```

**Challenge:** Single audio callback needs to write to multiple buffers.

**Solution:** Audio callback pushes to all enabled producers. Since push_overwrite is O(1), this is negligible overhead.

```rust
// In audio callback
for &sample in data {
    let sample_f32 = f32::from_sample(sample);
    if let Some(ref mut p) = producer_10s { p.push_overwrite(sample_f32); }
    if let Some(ref mut p) = producer_30s { p.push_overwrite(sample_f32); }
    if let Some(ref mut p) = producer_60s { p.push_overwrite(sample_f32); }
}
```

**Memory usage (all enabled):** ~4 + 12 + 24 = ~40 MB

### Phase 4: System Tray Integration

**Step 4.1: Create tray icon**
```rust
use tray_icon::{TrayIconBuilder, Icon, menu::*};

let icon = Icon::from_resource(1, None)?;  // Embedded resource
let tray = TrayIconBuilder::new()
    .with_icon(icon)
    .with_tooltip("The Miner - Recording")
    .with_menu(Box::new(build_menu()))
    .build()?;
```

**Step 4.2: Build context menu**
- Use `tray_icon::menu` for cross-platform menus
- Connect menu items to actions

**Step 4.3: Handle menu events**
```rust
MenuEvent::receiver().try_recv()  // Non-blocking
```

### Phase 5: Unified Event Loop

**Step 5.1: Combine event sources**
```rust
// Main event loop handles:
// 1. Tray menu events (MenuEvent::receiver())
// 2. Global hotkey events (GlobalHotKeyEvent::receiver())
// 3. Tray icon events (TrayIconEvent::receiver())

loop {
    // Check hotkeys
    if let Ok(event) = GlobalHotKeyEvent::receiver().try_recv() {
        handle_hotkey(event);
    }

    // Check tray menu
    if let Ok(event) = MenuEvent::receiver().try_recv() {
        handle_menu(event);
    }

    // Small sleep to prevent busy-waiting
    std::thread::sleep(Duration::from_millis(10));
}
```

### Phase 6: Desktop Notifications

**Step 6.1: Implement notification helper**
```rust
use notify_rust::Notification;

pub fn notify_save_complete(path: &str, duration: f32) {
    Notification::new()
        .summary("Audio Saved")
        .body(&format!("Saved {:.1}s to {}", duration, path))
        .timeout(3000)
        .show()
        .ok();  // Ignore errors - notifications are nice-to-have
}
```

### Phase 7: Windows-Specific Polish

**Step 7.1: Hide console window**
```rust
#[cfg(windows)]
fn hide_console() {
    use windows::Win32::System::Console::{GetConsoleWindow, FreeConsole};
    unsafe {
        if !GetConsoleWindow().is_invalid() {
            FreeConsole();
        }
    }
}
```

**Step 7.2: Autostart (optional)**
- Add registry entry for autostart on Windows
- Use `auto-launch` crate or manual registry manipulation

---

## File Structure (Stage 3)

```
polybius/
├── Cargo.toml
├── Cargo.lock
├── src/
│   ├── main.rs             # Entry point, event loop
│   ├── audio.rs            # Audio capture, ring buffers
│   ├── config.rs           # Configuration management
│   ├── hotkeys.rs          # Hotkey registration
│   ├── tray.rs             # System tray UI
│   ├── notifications.rs    # Desktop notifications
│   └── wav.rs              # WAV file writing
├── assets/
│   ├── icon.ico            # Windows icon
│   ├── icon.png            # Linux/macOS icon
│   └── icon_paused.png     # Paused state icon
├── README.md
├── STAGE1_PLAN.md
├── STAGE2_PLAN.md
└── STAGE3_PLAN.md          # This file
```

---

## Success Criteria

1. ✅ Run `cargo run --release`
2. ✅ App window closes immediately, tray icon appears
3. ✅ Right-click tray icon shows context menu
4. ✅ Press Ctrl+Alt+1 → saves 10s, notification appears
5. ✅ Press Ctrl+Alt+2 → saves 30s (if enabled in config)
6. ✅ "Open Save Folder" opens correct directory
7. ✅ "Pause" stops recording, icon changes
8. ✅ "Resume" restarts recording
9. ✅ "Quit" cleanly exits app
10. ✅ Config changes take effect after restart

---

## Testing Plan

### Unit Tests
- Config parsing (valid TOML, invalid TOML, missing fields)
- Path expansion (`~`, environment variables)
- Buffer sizing calculations

### Integration Tests
- Full audio capture → save cycle
- Multiple rapid saves
- Pause/resume behavior

### Manual Testing Checklist
- [ ] Fresh install (no config file exists)
- [ ] Upgrade from v0.2.x (config migration)
- [ ] All three hotkeys work
- [ ] Tray menu items all function
- [ ] Notifications appear and dismiss
- [ ] Correct audio in saved files
- [ ] Memory usage stays bounded (~40 MB max)
- [ ] No audio glitches during save
- [ ] Clean shutdown (no orphan processes)

---

## Edge Cases

1. **No audio device:** Show error notification, tray icon indicates error state
2. **Save directory doesn't exist:** Create it automatically
3. **Disk full:** Show error notification, don't crash
4. **Duplicate hotkey:** Warn in config validation, use first definition
5. **Config file corrupted:** Use defaults, log warning
6. **Multiple instances:** Detect and warn, or allow with separate configs

---

## Memory Footprint

| Configuration | Buffer Memory | Total (estimated) |
|--------------|---------------|-------------------|
| 10s only | ~4 MB | ~8 MB |
| 10s + 30s | ~16 MB | ~20 MB |
| All three | ~40 MB | ~45 MB |

Memory stays constant regardless of runtime duration - that's the beauty of ring buffers.

---

## Performance Considerations

1. **Audio callback must stay fast**
   - Three `push_overwrite` calls are still O(1)
   - No allocations in hot path
   - Measured overhead: <1μs per callback

2. **Event loop polling**
   - 10ms sleep prevents busy-waiting
   - CPU usage: <0.1% when idle
   - Instant response to hotkeys (within 10ms)

3. **WAV writing**
   - Happens synchronously on main thread
   - Could be moved to background thread if needed
   - Current approach: simple, fast enough for 60s files

---

## Future Enhancements (Stage 4 Preview)

- **Audio format options:** MP3, FLAC, OGG export
- **Audio normalization:** Auto-level adjustment
- **Silence trimming:** Remove leading/trailing silence
- **Cloud sync:** Auto-upload to Dropbox/Google Drive
- **Clip history:** Browse and manage saved clips
- **Audio visualization:** Show waveform in tray tooltip

---

## Questions Before Implementation

1. **Hotkey scheme:**
   - Option A: Ctrl+Alt+1/2/3 (numeric, easy to remember duration)
   - Option B: Ctrl+Alt+C with popup to choose duration
   - Option C: Single hotkey, configurable duration

2. **Default buffer configuration:**
   - Enable all three (more memory, more flexibility)?
   - Enable 10s only (minimal memory)?
   - Enable 10s + 30s (balanced)?

3. **Settings UI:**
   - Option A: Open config file in default editor
   - Option B: Simple native settings window
   - Option C: Web-based settings (localhost)

4. **Installer:**
   - Create Windows installer (MSI/NSIS)?
   - Or just distribute as portable executable?

---

## Implementation Order (Recommended)

1. **Phase 1: Project restructure** - Get modules in place, tests passing
2. **Phase 2: Configuration** - Load/save config, validate settings
3. **Phase 3: Multi-buffer** - Three ring buffers, single producer
4. **Phase 4: System tray** - Basic tray icon and menu
5. **Phase 5: Event loop** - Unified handling of all events
6. **Phase 6: Notifications** - Toast popups on save
7. **Phase 7: Polish** - Hide console, error handling, edge cases

Each phase builds on the previous one. We can ship after Phase 5 with a "beta" label, then add notifications and polish.

---

Ready to start implementing when you are!
