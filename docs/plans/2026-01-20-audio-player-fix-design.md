# Audio Player Fix Design

## Problem

The gym's `AudioPlayer` has several concurrency issues:

1. **Race condition** - `initialized` flag accessed without synchronization
2. **No cancellation** - Moving to next card or replaying doesn't stop current audio
3. **Goroutine leaks** - `PlayAsync` spawns untracked goroutines
4. **Silent failures** - Errors discarded, user gets no feedback

## Requirements

- Stop current audio immediately when new audio starts
- Show "[Audio unavailable]" in UI when playback fails
- Clean shutdown (close speaker) when TUI exits
- Thread-safe access to all state

## Design

### AudioPlayer Struct

```go
type AudioPlayer struct {
    mu          sync.Mutex
    initialized bool
    sampleRate  beep.SampleRate

    // Playback control
    playing     bool
    stopChan    chan struct{}

    // Error state for UI
    lastError   error
}
```

### Public API

| Method | Description |
|--------|-------------|
| `NewAudioPlayer()` | Creates player, no resources allocated yet |
| `Play(path string)` | Stops current playback, starts new (non-blocking) |
| `Stop()` | Stops current playback if any (idempotent) |
| `Close()` | Stops playback and releases speaker |
| `HasError() bool` | Returns true if last play attempt failed |

### Playback Flow

1. `Play()` acquires lock, closes existing `stopChan` if playing
2. Creates fresh `stopChan`, sets `playing = true`, clears `lastError`
3. Spawns goroutine with local copy of `stopChan`
4. Goroutine opens file, decodes WAV, initializes speaker (once)
5. Calls `speaker.Play()` with completion callback
6. `select` waits on either completion or stop signal
7. On stop signal, calls `speaker.Clear()` to halt playback
8. `defer` ensures `playing = false` and file cleanup

### Error Handling

- File open/decode errors stored in `lastError`
- UI checks `HasError()` in `View()` to show "[Audio unavailable]"
- Errors don't propagate - playback simply doesn't happen

### TUI Integration

```go
// Quitting - clean shutdown
case "q", "ctrl+c":
    m.audioPlayer.Close()
    m.quitting = true
    return m, tea.Quit

// View - show error state
case StateAudioOnly:
    if m.audioPlayer.HasError() {
        content.WriteString("[Audio unavailable]\n\n")
    } else {
        content.WriteString("Audio: [Playing...]\n\n")
    }
```

## Files to Modify

1. `brain/internal/gym/audio.go` - Complete rewrite
2. `brain/internal/gym/tui.go` - Add `Close()` call on quit, update View for error state

## Testing Notes

- Manual testing: rapid key presses (R, 1-4) should not cause audio overlap
- Manual testing: delete an audio file, should see "[Audio unavailable]"
- Manual testing: quit mid-playback, should exit cleanly without hanging
