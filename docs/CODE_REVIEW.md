# Polybius Code Review

**Date**: 2026-01-20
**Scope**: Full codebase review (Miner, Brain, Gym)

## Executive Summary

The architecture is sound - three components with clear responsibilities, good use of ring buffers and FSRS-6. But the implementation has reliability gaps: silent errors, race conditions, no input validation. The code works on the happy path but will fail mysteriously under stress.

---

## Critical Issues

### 1. Silent Error Swallowing

**Location**: `brain/cmd/polybius/main.go`

```go
homeDir, _ := os.UserHomeDir()  // Error ignored!
```

This pattern appears in multiple places. If `UserHomeDir()` fails, you get a cryptic panic later instead of a clear error message.

**Fix**:
```go
homeDir, err := os.UserHomeDir()
if err != nil {
    log.Fatalf("Cannot determine home directory: %v", err)
}
```

### 2. Race Condition via Sleep

**Location**: `brain/internal/brain/service.go:72`

```go
func (s *Service) handleNewCapture(jsonPath string) {
    time.Sleep(200 * time.Millisecond)  // Hope the file is ready!
```

This is synchronization-by-prayer. On a slow disk or network drive, 200ms may not be enough.

**Fix**: Use file stat polling to detect when file size stabilizes, or implement proper file locking between Miner and Brain.

### 3. Panic After Error Check (Rust)

**Location**: `miner/src/clipboard.rs:70`

```rust
if hmem.is_err() {
    CloseClipboard().ok();
    return Err(ClipboardError::MemoryError(...));
}
let hmem = hmem.unwrap();  // Logically unreachable but confusing
```

The flow won't reach the unwrap if `is_err()` triggered, but this pattern shows a misunderstanding that could lead to bugs elsewhere.

**Fix**: Use idiomatic Rust error handling:
```rust
let hmem = hmem.map_err(|_| {
    CloseClipboard().ok();
    ClipboardError::MemoryError("GlobalAlloc failed".to_string())
})?;
```

---

## Architectural Concerns

### 1. Hardcoded Paths in CLI Tool

**Location**: `brain/cmd/polybius/main.go:40-43`

```go
minerDir := filepath.Join(homeDir, "Music", "Miner")
dbPath := filepath.Join(homeDir, ".polybius", "brain.db")
cedictPath := filepath.Join(homeDir, ".polybius", "cedict_ts.u8")
```

Problems:
- Cannot run multiple instances with different databases
- Cannot use a test database during development
- Cannot deploy to non-standard locations

**Fix**: Add environment variable overrides:
```go
func getPath(envVar, defaultPath string) string {
    if v := os.Getenv(envVar); v != "" {
        return v
    }
    return defaultPath
}

dbPath := getPath("POLYBIUS_DB", filepath.Join(homeDir, ".polybius", "brain.db"))
```

### 2. Tight Coupling Between Miner and Brain

The components communicate via an implicit file contract:
```
audio_<timestamp>.wav
audio_<timestamp>.png
audio_<timestamp>.json
```

Issues:
- No schema version in the JSON
- No validation that Brain understands what Miner wrote
- No graceful handling if the format changes

**Fix**: Add a `"schema_version": 1` field to metadata JSON. Brain should validate and handle unknown versions gracefully.

### 3. No Dependency Injection

**Location**: `brain/internal/gym/review.go`

```go
func NewSession(...) *Session {
    scheduler := fsrs.NewScheduler()  // Can't swap this out
```

This makes testing nearly impossible without hitting real FSRS calculations.

**Fix**: Accept scheduler as a parameter:
```go
func NewSession(scheduler ReviewScheduler, ...) *Session {
```

---

## Code Organization Issues

### 1. Large Files with Multiple Responsibilities

**`brain/internal/gym/tui.go`** (341 lines) handles:
- Reveal state machine (audio -> screenshot -> text -> pinyin)
- Progress tracking
- Audio playback coordination
- Image rendering
- Rating input
- All UI rendering

The `Update()` method is 80+ lines of nested switch statements.

**Recommendation**: Split into separate concerns:
- `reveal_state.go` - State machine for card reveal progression
- `audio_controller.go` - Audio playback management
- `tui.go` - Pure rendering and input handling

### 2. Repetitive Database Query Patterns

**Location**: `brain/internal/storage/models.go`

Nearly identical code blocks for scanning cards:

```go
// GetDraftCards - lines 83-85
var fsrsStateJSON string
err := row.Scan(&card.ID, ..., &fsrsStateJSON)
json.Unmarshal([]byte(fsrsStateJSON), &card.FSRSState)

// GetDraftCardsByMoment - lines 279-281 (same pattern)
```

**Fix**: Extract a helper:
```go
func scanCard(row *sql.Row) (*Card, error) {
    var card Card
    var fsrsJSON string
    err := row.Scan(&card.ID, ..., &fsrsJSON)
    if err != nil {
        return nil, err
    }
    if err := json.Unmarshal([]byte(fsrsJSON), &card.FSRSState); err != nil {
        return nil, fmt.Errorf("invalid FSRS state: %w", err)
    }
    return &card, nil
}
```

### 3. Giant Match Statement in Rust

**Location**: `miner/src/hotkeys.rs`

The `parse_key_code()` function is 75+ lines of match arms:
```rust
match s {
    "a" => Code::KeyA,
    "b" => Code::KeyB,
    // ... 70 more lines
}
```

**Fix**: Use a HashMap lookup:
```rust
lazy_static! {
    static ref KEY_MAP: HashMap<&'static str, Code> = {
        let mut m = HashMap::new();
        m.insert("a", Code::KeyA);
        m.insert("b", Code::KeyB);
        // ...
        m
    };
}

fn parse_key_code(s: &str) -> Option<Code> {
    KEY_MAP.get(s).copied()
}
```

---

## Error Handling Inconsistencies

### Three Different Strategies in Go

```go
// Strategy 1: Ignore and continue (triage.go:70-71)
cards, err := m.db.GetDraftCardsByMoment(moment.ID)
if err != nil {
    continue  // User never knows their cards didn't load
}

// Strategy 2: Log fatal (main.go)
if err != nil {
    log.Fatalf("Failed to open database: %v", err)
}

// Strategy 3: Return error (vocab/import.go)
if err != nil {
    return fmt.Errorf("failed to parse TSV: %w", err)
}
```

**Recommendation**: Establish a consistent philosophy:
- **Startup errors**: Fatal (can't recover)
- **User-facing operations**: Return errors, display to user
- **Background processing**: Log and continue, but notify user of failures

### Unwrap Happy Path in Rust

```rust
// hotkeys.rs:231
let c = s.chars().next().unwrap();  // Empty string = panic

// main.rs:426-429
SystemTime::now()
    .duration_since(UNIX_EPOCH)
    .unwrap()  // System clock error = panic
```

These are in user-facing code paths. A malformed config or weird system state crashes the whole app.

**Fix**: Return `Result` types and handle errors gracefully.

### No Custom Error Types in Go

Everything is just `error` interface with string messages. Cannot programmatically distinguish between database errors, file system errors, or validation errors.

**Recommendation**: Create an error hierarchy:
```go
type PolybiusError struct {
    Kind    ErrorKind
    Message string
    Cause   error
}

type ErrorKind int
const (
    ErrDatabase ErrorKind = iota
    ErrFileSystem
    ErrValidation
    ErrNetwork
)
```

---

## Test Coverage Gaps

| Package | Test Status | Risk Level |
|---------|-------------|------------|
| `gym/tui.go` | No tests | High |
| `brain/watcher.go` | Minimal | Medium |
| `fsrs/scheduler.go` | No tests | High |
| `miner/` (all Rust) | No visible tests | High |

### Critical Gap

No integration test for the core value chain:
```
Capture -> File Watch -> Enrich -> Card Creation -> Review
```

Individual pieces are tested but not the system working together.

### Fragile Test Setup

```go
// Uses hardcoded relative paths
dictPath := "../../data/cedict_ts.u8"
```

Tests break if run from different directories.

**Fix**: Use `runtime.Caller` or embed test fixtures.

---

## Quick Wins (High Impact, Low Effort)

| Fix | Effort | Impact |
|-----|--------|--------|
| Handle `UserHomeDir()` error | 10 min | Prevents mysterious failures |
| Extract `scanCard()` helper | 30 min | Eliminates 5+ duplicate blocks |
| Replace hotkey match with HashMap | 30 min | Cleaner, faster, maintainable |
| Add env var overrides for paths | 20 min | Instant configurability |
| Add schema version to metadata JSON | 15 min | Future-proofs protocol |

---

## Medium-Term Improvements (Priority Order)

| Priority | Issue | Why It Matters |
|----------|-------|----------------|
| 1 | Replace sleep-based sync with file stat polling | Prevents data corruption |
| 2 | Add schema version to Miner JSON output | Future-proofs the protocol |
| 3 | Extract TUI into smaller components | Enables testing, reduces bugs |
| 4 | Add integration test for capture->card flow | Catches regressions |
| 5 | Introduce dependency injection for Scheduler | Enables unit testing |
| 6 | Add structured logging (Go: `slog`, Rust: `tracing`) | Debuggability |

---

## Code Smells Summary

| Smell | Location | Severity |
|-------|----------|----------|
| Magic Strings/Numbers | Multiple | Medium |
| Silent Error Handling | main.go, triage.go | High |
| Hardcoded Paths | main.go | High |
| Race Condition (Sleep-based) | service.go:72 | High |
| Panic on Unwrap | clipboard.rs:70, hotkeys.rs:231 | Medium |
| Large Functions | tui.go, region_overlay.rs | Medium |
| Missing Validation | models.go | Medium |
| Tight Coupling | service.go, main.rs | Medium |
| Platform-Specific With No Tests | ocr.rs, vision.rs | Low-Medium |

---

## Conclusion

The codebase demonstrates solid architectural thinking - the three-component separation is clean, the ring buffer implementation is correct, and FSRS-6 integration is well-done. The issues identified are primarily around production hardening:

1. **Error handling discipline** - Be consistent, never swallow errors silently
2. **Configuration management** - Make paths configurable
3. **Race condition elimination** - Replace sleep with proper synchronization
4. **Test coverage** - Focus on integration tests for the critical path

Addressing the "Quick Wins" section alone would significantly improve reliability. The medium-term improvements would bring the codebase to production-ready quality.
