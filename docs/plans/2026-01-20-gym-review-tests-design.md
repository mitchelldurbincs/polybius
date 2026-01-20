# Gym Review Module Unit Tests Design

## Overview

Add unit tests for `brain/internal/gym/review.go` by introducing interfaces for dependency injection, enabling isolated testing of session management logic.

## Goals

- Unit test `GetDueCards` and `SubmitRating` functions in isolation
- Test helper functions `cardStateToInt` and `intToCardState`
- No external mocking libraries - use simple mock structs
- Maintain backwards compatibility with existing code

## Interface Definitions

Add to `review.go`:

```go
// CardStore abstracts database operations for review sessions
type CardStore interface {
    GetDueCards(limit int) ([]*storage.Card, error)
    GetMoment(id int64) (*storage.Moment, error)
    GetCard(id int64) (*storage.Card, error)
    UpdateCardAfterReview(id int64, update storage.CardUpdate) error
}

// ReviewScheduler abstracts spaced repetition calculations
type ReviewScheduler interface {
    Review(state fsrs.CardState, rating int, now time.Time) (fsrs.CardState, time.Time)
}
```

## Session Struct Changes

From:
```go
type Session struct {
    db        *storage.DB
    scheduler *fsrs.Scheduler
}
```

To:
```go
type Session struct {
    store     CardStore
    scheduler ReviewScheduler
}
```

`NewSession` signature unchanged - still accepts `*storage.DB`, creates real scheduler.

## Test Cases

### Helper Functions (pure, no mocks)

| Test | Description |
|------|-------------|
| `TestCardStateToInt` | Verify all state string to int mappings |
| `TestIntToCardState` | Verify all int to state string mappings |
| Edge cases | Unknown values return defaults |

### GetDueCards

| Test | Description |
|------|-------------|
| `TestGetDueCards_Success` | Returns enriched ReviewCards with moment data |
| `TestGetDueCards_Empty` | Returns empty slice when no cards due |
| `TestGetDueCards_DBError` | Propagates error from store |
| `TestGetDueCards_MomentNotFound` | Skips cards with missing moments |

### SubmitRating

| Test | Description |
|------|-------------|
| `TestSubmitRating_Success` | Rating flows through scheduler, updates store |
| `TestSubmitRating_CardNotFound` | Returns error when card doesn't exist |
| `TestSubmitRating_UpdateError` | Propagates update errors |
| `TestSubmitRating_WithLastReview` | Handles cards reviewed before |
| `TestSubmitRating_NewCard` | Handles cards with nil LastReview |

## Mock Implementations

```go
type mockCardStore struct {
    // Return values (set per test)
    dueCards      []*storage.Card
    dueCardsErr   error
    moment        *storage.Moment
    momentErr     error
    card          *storage.Card
    cardErr       error
    updateErr     error

    // Call tracking (verify in tests)
    getDueCardsLimit int
    getMomentID      int64
    getCardID        int64
    updatedID        int64
    updatedWith      storage.CardUpdate
}

type mockScheduler struct {
    returnState fsrs.CardState
    returnTime  time.Time

    // Call tracking
    calledWith struct {
        state  fsrs.CardState
        rating int
        now    time.Time
    }
}
```

## Files Changed

| File | Change |
|------|--------|
| `review.go` | Add interfaces, update Session struct fields |
| `review_test.go` | New file with mocks and all tests |

## No Changes Required

- `storage/db.go` - already satisfies CardStore interface
- `fsrs/scheduler.go` - already satisfies ReviewScheduler interface
- `tui.go`, `triage.go` - don't use Session internals
- `cmd/polybius/main.go` - still calls `NewSession(db)` unchanged

## Test Commands

```bash
cd brain && go test ./internal/gym/... -v
```
