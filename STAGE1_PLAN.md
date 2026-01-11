# Stage 1 Implementation Plan: The Recorder

**Goal:** Run the app, play audio (YouTube/system sound), hit Ctrl+C, get a valid `.wav` file saved to disk.

---

## Overview

Stage 1 is the foundation - we need to prove we can capture system audio on Windows using WASAPI loopback. This is the riskiest technical piece, so we tackle it first.

---

## Dependencies (Cargo.toml)

```toml
[package]
name = "miner"
version = "0.1.0"
edition = "2021"

[dependencies]
cpal = "0.15"           # Audio capture (WASAPI support)
hound = "3.5"           # WAV file writing
global-hotkey = "0.6"   # Global hotkey detection (Ctrl+Alt+C)
```

**Why these versions:**
- `cpal 0.15` - Latest stable, good WASAPI loopback support
- `hound 3.5` - Simple, battle-tested WAV writer
- `global-hotkey 0.6` - Cross-platform global hotkey, same crate we'll use in Stage 3

---

## Architecture (Stage 1 Only)

```
┌─────────────────────────────────────────────────────┐
│                    Main Thread                       │
├─────────────────────────────────────────────────────┤
│  1. Initialize WASAPI loopback device               │
│  2. Create sample buffer (Vec<f32>)                 │
│  3. Register Ctrl+Alt+C hotkey                      │
│  4. Start audio stream → push samples to buffer     │
│  5. Wait for hotkey event                           │
│  6. Stop stream, write buffer to audio.wav          │
└─────────────────────────────────────────────────────┘
```

For Stage 1, we keep it simple: **single-threaded, no ring buffer yet**. Just capture everything until Ctrl+Alt+C, then dump to WAV.

---

## Implementation Steps

### Step 1: Project Setup
- Initialize Cargo project
- Add dependencies
- Create basic main.rs skeleton

### Step 2: Device Discovery
- List all audio devices
- Find WASAPI loopback device (output device used as input)
- Print device info for debugging

**Key insight:** WASAPI loopback captures "what you hear" by opening an *output* device in loopback mode. In cpal, we look for the default output device and request loopback.

### Step 3: Audio Stream Setup
- Get default output device config (sample rate, channels)
- Build input stream using loopback
- Configure callback to collect samples

**Expected config:** Typically 44100 Hz or 48000 Hz, 2 channels (stereo), f32 samples

### Step 4: Sample Collection
- Use `Arc<Mutex<Vec<f32>>>` for thread-safe sample storage
- Audio callback pushes samples to the shared buffer
- Main thread runs event loop waiting for hotkey

### Step 5: WAV Export
- On Ctrl+Alt+C, stop the stream
- Create WAV spec from stream config
- Write all samples using hound
- Print success message with file path

---

## File Structure

```
polybius/
├── Cargo.toml
├── src/
│   └── main.rs         # All Stage 1 code in one file
├── README.md
└── STAGE1_PLAN.md      # This file
```

---

## Code Outline (main.rs)

```rust
// Pseudocode structure

fn main() {
    // 1. Setup global hotkey manager
    let manager = GlobalHotKeyManager::new()?;
    let hotkey = HotKey::new(Modifiers::CONTROL | Modifiers::ALT, Code::KeyC);
    manager.register(hotkey)?;

    // 2. Get WASAPI host and default output device
    let host = cpal::host_from_id(HostId::Wasapi)?;
    let device = host.default_output_device()?;

    // 3. Get supported config
    let config = device.default_output_config()?;

    // 4. Create sample buffer
    let samples = Arc::new(Mutex::new(Vec::new()));

    // 5. Build loopback input stream
    let stream = device.build_input_stream(
        &config,
        |data: &[f32]| { samples.push(data); },
        |err| { eprintln!("Error: {}", err); },
    )?;

    // 6. Start stream and run event loop
    stream.play()?;
    println!("Recording... Press Ctrl+Alt+C to stop");

    // Event loop - wait for hotkey
    loop {
        if let Ok(event) = GlobalHotKeyEvent::receiver().recv() {
            if event.id == hotkey.id() {
                break;
            }
        }
    }

    // 7. Stop and save
    drop(stream);
    write_wav("audio.wav", &samples, config)?;
    println!("Saved audio.wav");
}
```

---

## Known Challenges & Solutions

### Challenge 1: WASAPI Loopback API
**Problem:** cpal's loopback API is WASAPI-specific and slightly different from normal input.

**Solution:** Use `device.build_input_stream()` with the device's output config. On Windows with WASAPI, when you open an input stream on an output device, it automatically does loopback capture.

### Challenge 2: Finding the Right Device
**Problem:** Device enumeration can be confusing.

**Solution:** Use `host.default_output_device()` - this is what we want to capture from (the speakers/headphones).

### Challenge 3: Sample Format
**Problem:** Devices may report different sample formats (i16, f32, etc).

**Solution:** Handle both f32 and i16, convert to f32 internally, write WAV as 16-bit PCM (most compatible).

---

## Success Criteria

1. ✅ Run `cargo run`
2. ✅ See "Recording... Press Ctrl+Alt+C to stop"
3. ✅ Play a YouTube video for ~5 seconds
4. ✅ Press Ctrl+Alt+C
5. ✅ See "Saved audio.wav"
6. ✅ Open audio.wav in any media player - hear the YouTube audio

---

## Testing Plan

1. **Smoke Test:** Run app, make noise, stop, verify WAV exists
2. **Playback Test:** Open WAV in VLC/Windows Media Player
3. **Quality Test:** Audio should be clear, correct duration
4. **Edge Cases:**
   - No audio playing → should create silent WAV (or empty)
   - Very short recording (<1s) → should still work

---

## Next Steps (Stage 2 Preview)

Once Stage 1 works, Stage 2 adds:
- Ring buffer (only keep last 10 seconds)
- Continuous recording (runs indefinitely)
- Thread-safe buffer management

But that's for later. Let's get the basics working first!

---

## Questions Before Implementation?

1. **Output location:** Save to `./output/audio.wav` or just `./audio.wav`?
2. **Sample rate preference:** Accept device default or force 44100 Hz?
3. **Error handling:** Panic on errors or graceful degradation?

Let me know if this plan looks good, or if you'd like any changes before I start coding!
