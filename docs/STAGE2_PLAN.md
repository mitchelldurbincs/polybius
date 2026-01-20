# Stage 2 Implementation Plan: Ring Buffer

**Goal:** Record system audio continuously, keeping only the last 10 seconds in a ring buffer. Press Ctrl+Alt+C to save what's in the buffer.

---

## Overview

Stage 2 transforms our one-shot recorder into a continuous capture system. Instead of recording everything until stop, we maintain a rolling 10-second window of audio. This is the foundation for the "retrospective capture" feature - save audio that *already happened*.

---

## What Changes from Stage 1

| Aspect | Stage 1 | Stage 2 |
|--------|---------|---------|
| Buffer | `Vec<f32>` (grows forever) | Ring buffer (fixed 10s) |
| Recording | Start → Stop | Always running |
| Memory | Unbounded | Fixed (~4MB for 10s stereo) |
| Threading | Mutex on Vec | Lock-free ring buffer |

---

## Dependencies (Cargo.toml)

```toml
[package]
name = "miner"
version = "0.2.0"
edition = "2021"

[dependencies]
cpal = "0.15"           # Audio capture
hound = "3.5"           # WAV file writing
global-hotkey = "0.6"   # Global hotkey detection
ringbuf = "0.4"         # Lock-free ring buffer
```

**New dependency:**
- `ringbuf 0.4` - High-performance lock-free SPSC (single-producer, single-consumer) ring buffer

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                       Main Thread                            │
├─────────────────────────────────────────────────────────────┤
│  1. Initialize ring buffer (10 seconds capacity)            │
│  2. Start audio stream → Producer writes to ring buffer     │
│  3. Run event loop waiting for Ctrl+Alt+C                   │
│  4. On hotkey: Consumer reads all available samples         │
│  5. Write samples to WAV file                               │
└─────────────────────────────────────────────────────────────┘

         ┌──────────────┐
         │ Audio Stream │ (Producer)
         │   Callback   │
         └──────┬───────┘
                │ push samples
                ▼
    ┌───────────────────────────────┐
    │      Ring Buffer (10s)        │
    │  [older samples → newer]      │
    │  Auto-overwrites oldest       │
    └───────────────────────────────┘
                │ read all on save
                ▼
         ┌──────────────┐
         │  Main Thread │ (Consumer)
         │  WAV Writer  │
         └──────────────┘
```

---

## Ring Buffer Sizing

**Calculation:**
- Sample rate: 48000 Hz (typical)
- Channels: 2 (stereo)
- Sample size: 4 bytes (f32)
- Duration: 10 seconds

**Samples needed:** 48000 × 2 × 10 = 960,000 samples
**Memory:** 960,000 × 4 bytes = 3.84 MB

We'll calculate dynamically based on actual device config.

---

## Implementation Steps

### Step 1: Add ringbuf Dependency
- Update Cargo.toml with `ringbuf = "0.4"`

### Step 2: Create Ring Buffer Module
- Calculate buffer size from sample rate and channels
- Create producer/consumer pair
- Producer goes to audio callback
- Consumer stays in main thread

### Step 3: Modify Audio Callback
- Push samples to ring buffer (overwrites old automatically)
- Handle case when buffer is full (it just wraps)

### Step 4: Update Save Logic
- On Ctrl+Alt+C, drain all samples from ring buffer
- Samples come out in chronological order
- Write to WAV as before

### Step 5: Continuous Operation
- After saving, optionally continue recording
- Clear buffer or keep accumulating

---

## Code Changes

### New ring buffer setup:
```rust
use ringbuf::HeapRb;

// Calculate buffer size for 10 seconds
let buffer_duration_secs = 10;
let buffer_size = sample_rate as usize * channels as usize * buffer_duration_secs;

// Create ring buffer with producer/consumer
let ring = HeapRb::<f32>::new(buffer_size);
let (mut producer, mut consumer) = ring.split();
```

### Modified audio callback:
```rust
// In audio callback - push samples, old ones auto-discarded
move |data: &[T], _: &cpal::InputCallbackInfo| {
    for &sample in data {
        let _ = producer.try_push(f32::from_sample(sample));
        // If full, we need to pop one first to make room
    }
}
```

### Save logic:
```rust
// Drain all samples from ring buffer
let mut samples = Vec::with_capacity(consumer.len());
while let Some(sample) = consumer.try_pop() {
    samples.push(sample);
}
```

---

## Key Differences from Vec<Mutex<>>

1. **Lock-free:** No mutex contention between audio thread and main thread
2. **Fixed memory:** Won't grow unbounded
3. **Automatic overwrite:** Old samples discarded automatically
4. **Better latency:** Audio callback never blocks

---

## Success Criteria

1. Run `cargo run`
2. See "Recording continuously (last 10 seconds)..."
3. Play audio for 30 seconds
4. Press Ctrl+Alt+C
5. Get a WAV file that's ~10 seconds long (not 30)
6. Audio in WAV is from the LAST 10 seconds played

---

## Edge Cases

1. **Save immediately:** If hotkey pressed <10s after start, WAV is shorter
2. **No audio:** Silent sections are still captured
3. **Multiple saves:** Each save captures the last 10s at that moment

---

## Memory Footprint

| Duration | Stereo 48kHz | Stereo 44.1kHz |
|----------|--------------|----------------|
| 10 sec   | 3.84 MB      | 3.53 MB        |
| 30 sec   | 11.5 MB      | 10.6 MB        |
| 60 sec   | 23.0 MB      | 21.2 MB        |

10 seconds is a good default - enough for useful capture, minimal memory.

---

## Next Steps (Stage 3 Preview)

Stage 3 will add:
- Background service (runs on system startup)
- System tray icon
- Multiple hotkeys (save last 10s, 30s, 60s)
- Configurable save location

But first, let's get the ring buffer working!
