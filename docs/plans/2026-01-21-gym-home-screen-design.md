# Gym Home Screen Design

## Overview

Add a home screen as the central hub when launching the Gym, providing visibility into FSRS scheduling state and learning progress.

## Problem

Currently, when there are no cards due, the Gym shows "no cards to review" with no context about when cards will be due or how learning is progressing.

## Solution

A new home screen that displays:
- Review summary (due now, due today, next review time)
- Progress stats (words learned, retention, streak, review history)
- Keyboard navigation to triage, review, and card list views

---

## Home Screen Layout

### Top: Review Summary

```
╭─ Reviews ─────────────────────────────────────────╮
│  Due now: 0    Due today: 1    Total cards: 1    │
│  Next review: 3h 24m (2:30 PM)                   │
╰──────────────────────────────────────────────────╯
```

### Middle: Progress Stats

```
╭─ Progress ────────────────────────────────────────╮
│  Words learned: 12    Retention: 87%             │
│  Reviews today: 0     Streak: 3 days             │
│                                                  │
│  Last 7 days: ▁▃▅▂▄▁▁                           │
╰──────────────────────────────────────────────────╯
```

### Bottom: Navigation

```
  [r] Review    [t] Triage (2 drafts)    [c] Cards    [q] Quit
```

---

## Cards View

Accessible via `c` from home screen. Shows all cards with FSRS state.

```
╭─ All Cards ──────────────────────────────────────────────────╮
│                                                              │
│  1. 超市                                                     │
│     "我今天去了超市" · Due: 3h 24m                           │
│     Stability: Strong · Difficulty: Easy · Reviews: 4        │
│                                                              │
│  2. 电影院                                                   │
│     "我们去电影院看电影" · Due: Tomorrow                      │
│     Stability: Building · Difficulty: Medium · Reviews: 2    │
│                                                              │
│  3. 咖啡                                                     │
│     "我喜欢喝咖啡" · Due: 5 days                             │
│     Stability: Solid · Difficulty: Easy · Reviews: 7         │
│                                                              │
╰──────────────────────────────────────────────────────────────╯
  [↑/↓] Navigate    [Enter] Details    [Esc] Back
```

### FSRS Label Mapping

| FSRS Value | User-Friendly Label |
|------------|---------------------|
| Stability (days to 90% forgetting) | Fragile → Building → Solid → Strong |
| Difficulty (0-1 scale) | Easy → Medium → Hard |
| Due time | Relative: "3h 24m", "Tomorrow", "5 days" |

---

## Flow Integration

### Before

```
gym launch → triage mode (or "no cards" message)
```

### After

```
gym launch → home screen → [r] review / [t] triage / [c] cards
```

### Navigation

- From any mode, `Esc` returns to home screen
- From home screen, `q` quits the Gym
- After completing review/triage sessions, return to home screen
- Stats refresh each time you return to home screen

### Unchanged

- Triage mode works exactly as before
- Review mode works exactly as before
- All existing keybindings within those modes unchanged

---

## Technical Implementation

### New Components

1. **Home Model** (`internal/gym/home.go`)
   - New Bubbletea model for the home screen
   - Fetches stats from storage on init and when returning from other modes
   - Handles `r`, `t`, `c`, `q` key events

2. **Stats Queries** (`internal/storage/stats.go`)
   - `GetDueCount(now, endOfDay)` - cards due now vs today
   - `GetNextDueTime()` - when is the next card due
   - `GetRetentionRate()` - calculated from review history
   - `GetStreak()` - consecutive days with reviews
   - `GetReviewsPerDay(days)` - for the sparkline graph

3. **FSRS Label Mapping** (`internal/fsrs/labels.go`)
   - `StabilityLabel(stability float64) string` - returns "Fragile"/"Building"/"Solid"/"Strong"
   - `DifficulityLabel(difficulty float64) string` - returns "Easy"/"Medium"/"Hard"
   - `RelativeDue(dueTime time.Time) string` - returns "3h 24m"/"Tomorrow"/"5 days"

4. **Cards List Model** (`internal/gym/cards.go`)
   - New Bubbletea model for browsing all cards
   - Paginated list with FSRS details per card

### Mode Switching

Main Gym model gains a `currentMode` field that switches between home/triage/review/cards views.

---

## Edge Cases

### Empty States

- **No cards at all**: "No cards yet - capture some moments with the Miner!"
- **No review history**: Retention shows "—", streak shows "0 days", sparkline empty
- **No drafts**: Triage option shows `[t] Triage` without draft count

### Stat Calculations

- **Retention rate**: Percentage of reviews rated Good (3) or Easy (4) over last 30 days
- **Streak**: Consecutive calendar days with at least one review
- **Sparkline**: Last 7 days of review counts, scaled (▁▂▃▄▅▆▇█)

### Refresh Behavior

- Stats refresh when returning to home screen from any mode
- No auto-refresh while on home screen

---

## Out of Scope

- Notifications when cards become due
- Filtering cards by difficulty/stability
- Export stats to file
- Card detail view (just list for now)
