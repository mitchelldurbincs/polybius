# Gym Home Screen Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a home screen dashboard to the Gym TUI that shows FSRS scheduling state and learning progress.

**Architecture:** New `home.go` Bubbletea model becomes the entry point, with stats queries in storage and FSRS label helpers. Existing triage/review modes remain unchanged but are now navigated to from home.

**Tech Stack:** Go, Bubbletea, Lipgloss, SQLite

---

## Task 1: Add Stats Queries to Storage

**Files:**
- Create: `brain/internal/storage/stats.go`
- Test: `brain/internal/storage/stats_test.go`

**Step 1: Write the failing tests**

```go
// brain/internal/storage/stats_test.go
package storage

import (
	"testing"
	"time"
)

func TestGetCardStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert test moment
	momentID, _ := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		RawText:   "test",
		Status:    "processed",
	})

	// Insert cards with various states
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(3 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	// Card due now
	db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review", DueDate: &past})
	// Card due later today
	db.InsertCard(&Card{MomentID: momentID, TargetWord: "word2", State: "review", DueDate: &future})
	// Card due tomorrow
	db.InsertCard(&Card{MomentID: momentID, TargetWord: "word3", State: "review", DueDate: &tomorrow})
	// Draft card (should not count)
	db.InsertCard(&Card{MomentID: momentID, TargetWord: "word4", State: "draft"})

	stats, err := db.GetCardStats()
	if err != nil {
		t.Fatalf("GetCardStats failed: %v", err)
	}

	if stats.DueNow != 1 {
		t.Errorf("DueNow = %d, want 1", stats.DueNow)
	}
	if stats.DueToday != 2 { // includes DueNow
		t.Errorf("DueToday = %d, want 2", stats.DueToday)
	}
	if stats.TotalCards != 3 { // excludes drafts
		t.Errorf("TotalCards = %d, want 3", stats.TotalCards)
	}
}

func TestGetNextDueTime(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	momentID, _ := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		Status:    "processed",
	})

	// No cards - should return nil
	nextDue, err := db.GetNextDueTime()
	if err != nil {
		t.Fatalf("GetNextDueTime failed: %v", err)
	}
	if nextDue != nil {
		t.Errorf("Expected nil for empty db, got %v", nextDue)
	}

	// Add a card due in the future
	future := time.Now().Add(3 * time.Hour)
	db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review", DueDate: &future})

	nextDue, err = db.GetNextDueTime()
	if err != nil {
		t.Fatalf("GetNextDueTime failed: %v", err)
	}
	if nextDue == nil {
		t.Fatal("Expected non-nil next due time")
	}
	// Allow 1 second tolerance
	if nextDue.Sub(future).Abs() > time.Second {
		t.Errorf("NextDue = %v, want ~%v", nextDue, future)
	}
}

func TestGetReviewStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	momentID, _ := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		Status:    "processed",
	})

	cardID, _ := db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review"})

	// Add some reviews
	db.InsertReview(cardID, 3) // Good
	db.InsertReview(cardID, 4) // Easy
	db.InsertReview(cardID, 1) // Again
	db.InsertReview(cardID, 3) // Good

	stats, err := db.GetReviewStats(30)
	if err != nil {
		t.Fatalf("GetReviewStats failed: %v", err)
	}

	if stats.TotalReviews != 4 {
		t.Errorf("TotalReviews = %d, want 4", stats.TotalReviews)
	}
	// Retention = (Good + Easy) / Total = 3/4 = 75%
	if stats.RetentionRate < 0.74 || stats.RetentionRate > 0.76 {
		t.Errorf("RetentionRate = %f, want ~0.75", stats.RetentionRate)
	}
}

func TestGetStreak(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	momentID, _ := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		Status:    "processed",
	})

	cardID, _ := db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review"})

	// Reviews today
	db.InsertReview(cardID, 3)
	// Reviews yesterday (need to insert with specific time)
	db.insertReviewAt(cardID, 3, time.Now().Add(-24*time.Hour))
	// Reviews 2 days ago
	db.insertReviewAt(cardID, 3, time.Now().Add(-48*time.Hour))

	streak, err := db.GetStreak()
	if err != nil {
		t.Fatalf("GetStreak failed: %v", err)
	}

	if streak != 3 {
		t.Errorf("Streak = %d, want 3", streak)
	}
}

func TestGetReviewsPerDay(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	momentID, _ := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		Status:    "processed",
	})

	cardID, _ := db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review"})

	// Add reviews on different days
	db.InsertReview(cardID, 3)
	db.InsertReview(cardID, 3)
	db.insertReviewAt(cardID, 3, time.Now().Add(-24*time.Hour))

	counts, err := db.GetReviewsPerDay(7)
	if err != nil {
		t.Fatalf("GetReviewsPerDay failed: %v", err)
	}

	if len(counts) != 7 {
		t.Errorf("len(counts) = %d, want 7", len(counts))
	}
	// Today should have 2 reviews
	if counts[6] != 2 {
		t.Errorf("Today's count = %d, want 2", counts[6])
	}
	// Yesterday should have 1
	if counts[5] != 1 {
		t.Errorf("Yesterday's count = %d, want 1", counts[5])
	}
}

func TestCountLearnedWords(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add known words
	db.UpsertVocabulary(&Vocabulary{Word: "word1", Status: "known"})
	db.UpsertVocabulary(&Vocabulary{Word: "word2", Status: "known"})
	db.UpsertVocabulary(&Vocabulary{Word: "word3", Status: "unknown"})

	count, err := db.CountLearnedWords()
	if err != nil {
		t.Fatalf("CountLearnedWords failed: %v", err)
	}

	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd brain && go test ./internal/storage -run "Test(GetCardStats|GetNextDueTime|GetReviewStats|GetStreak|GetReviewsPerDay|CountLearnedWords)" -v`

Expected: FAIL - functions don't exist

**Step 3: Implement the stats queries**

```go
// brain/internal/storage/stats.go
package storage

import (
	"time"
)

// CardStats holds aggregate card statistics
type CardStats struct {
	DueNow     int
	DueToday   int
	TotalCards int
}

// ReviewStats holds review history statistics
type ReviewStats struct {
	TotalReviews  int
	RetentionRate float64 // 0.0 to 1.0
	ReviewsToday  int
}

// GetCardStats returns aggregate statistics about cards
func (db *DB) GetCardStats() (*CardStats, error) {
	stats := &CardStats{}

	// Count cards due now (excluding drafts)
	err := db.QueryRow(`
		SELECT COUNT(*) FROM cards
		WHERE state != 'draft'
		AND (due_date IS NULL OR due_date <= datetime('now'))
	`).Scan(&stats.DueNow)
	if err != nil {
		return nil, err
	}

	// Count cards due today (from now until end of day)
	err = db.QueryRow(`
		SELECT COUNT(*) FROM cards
		WHERE state != 'draft'
		AND (due_date IS NULL OR due_date <= datetime('now', 'start of day', '+1 day'))
	`).Scan(&stats.DueToday)
	if err != nil {
		return nil, err
	}

	// Count total active cards (excluding drafts)
	err = db.QueryRow(`
		SELECT COUNT(*) FROM cards WHERE state != 'draft'
	`).Scan(&stats.TotalCards)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// GetNextDueTime returns when the next card is due, or nil if no cards
func (db *DB) GetNextDueTime() (*time.Time, error) {
	var dueDateStr *string
	err := db.QueryRow(`
		SELECT due_date FROM cards
		WHERE state != 'draft' AND due_date IS NOT NULL
		ORDER BY due_date ASC
		LIMIT 1
	`).Scan(&dueDateStr)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}

	if dueDateStr == nil {
		return nil, nil
	}

	t, err := time.Parse(time.RFC3339, *dueDateStr)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// GetReviewStats returns review statistics for the last N days
func (db *DB) GetReviewStats(days int) (*ReviewStats, error) {
	stats := &ReviewStats{}

	cutoff := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)

	// Total reviews in period
	err := db.QueryRow(`
		SELECT COUNT(*) FROM reviews
		WHERE reviewed_at >= ?
	`, cutoff).Scan(&stats.TotalReviews)
	if err != nil {
		return nil, err
	}

	// Retention rate (Good=3 or Easy=4 / total)
	if stats.TotalReviews > 0 {
		var successCount int
		err = db.QueryRow(`
			SELECT COUNT(*) FROM reviews
			WHERE reviewed_at >= ? AND rating >= 3
		`, cutoff).Scan(&successCount)
		if err != nil {
			return nil, err
		}
		stats.RetentionRate = float64(successCount) / float64(stats.TotalReviews)
	}

	// Reviews today
	today := time.Now().Format("2006-01-02")
	err = db.QueryRow(`
		SELECT COUNT(*) FROM reviews
		WHERE date(reviewed_at) = ?
	`, today).Scan(&stats.ReviewsToday)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// GetStreak returns the number of consecutive days with reviews
func (db *DB) GetStreak() (int, error) {
	// Get distinct review dates in descending order
	rows, err := db.Query(`
		SELECT DISTINCT date(reviewed_at) as review_date
		FROM reviews
		ORDER BY review_date DESC
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	streak := 0
	expectedDate := time.Now().Truncate(24 * time.Hour)

	for rows.Next() {
		var dateStr string
		if err := rows.Scan(&dateStr); err != nil {
			return 0, err
		}

		reviewDate, _ := time.Parse("2006-01-02", dateStr)
		reviewDate = reviewDate.Truncate(24 * time.Hour)

		// Check if this is the expected date or yesterday (for first iteration)
		if streak == 0 {
			// First date can be today or yesterday
			if reviewDate.Equal(expectedDate) || reviewDate.Equal(expectedDate.Add(-24*time.Hour)) {
				streak = 1
				expectedDate = reviewDate.Add(-24 * time.Hour)
			} else {
				break // No streak
			}
		} else {
			if reviewDate.Equal(expectedDate) {
				streak++
				expectedDate = expectedDate.Add(-24 * time.Hour)
			} else {
				break // Streak broken
			}
		}
	}

	return streak, nil
}

// GetReviewsPerDay returns review counts for the last N days
// Returns a slice where index 0 is N days ago and last index is today
func (db *DB) GetReviewsPerDay(days int) ([]int, error) {
	counts := make([]int, days)

	// Query reviews grouped by date
	rows, err := db.Query(`
		SELECT date(reviewed_at) as review_date, COUNT(*) as cnt
		FROM reviews
		WHERE reviewed_at >= datetime('now', ?)
		GROUP BY date(reviewed_at)
	`, fmt.Sprintf("-%d days", days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	today := time.Now().Truncate(24 * time.Hour)
	dateCounts := make(map[string]int)

	for rows.Next() {
		var dateStr string
		var count int
		if err := rows.Scan(&dateStr, &count); err != nil {
			return nil, err
		}
		dateCounts[dateStr] = count
	}

	// Fill in the counts array
	for i := 0; i < days; i++ {
		date := today.AddDate(0, 0, -(days - 1 - i))
		dateStr := date.Format("2006-01-02")
		counts[i] = dateCounts[dateStr]
	}

	return counts, nil
}

// CountLearnedWords returns the number of words marked as known
func (db *DB) CountLearnedWords() (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM vocabulary WHERE status = 'known'`).Scan(&count)
	return count, err
}

// InsertReview adds a review record
func (db *DB) InsertReview(cardID int64, rating int) error {
	_, err := db.Exec(`
		INSERT INTO reviews (card_id, rating) VALUES (?, ?)
	`, cardID, rating)
	return err
}

// insertReviewAt adds a review record with a specific time (for testing)
func (db *DB) insertReviewAt(cardID int64, rating int, at time.Time) error {
	_, err := db.Exec(`
		INSERT INTO reviews (card_id, rating, reviewed_at) VALUES (?, ?, ?)
	`, cardID, rating, at.Format(time.RFC3339))
	return err
}
```

Add missing import at top of file:
```go
import (
	"fmt"
	"time"
)
```

**Step 4: Run tests to verify they pass**

Run: `cd brain && go test ./internal/storage -run "Test(GetCardStats|GetNextDueTime|GetReviewStats|GetStreak|GetReviewsPerDay|CountLearnedWords)" -v`

Expected: PASS

**Step 5: Commit**

```bash
git add brain/internal/storage/stats.go brain/internal/storage/stats_test.go
git commit -m "feat(storage): add stats queries for gym home screen"
```

---

## Task 2: Add FSRS Label Helpers

**Files:**
- Create: `brain/internal/fsrs/labels.go`
- Test: `brain/internal/fsrs/labels_test.go`

**Step 1: Write the failing tests**

```go
// brain/internal/fsrs/labels_test.go
package fsrs

import (
	"testing"
	"time"
)

func TestStabilityLabel(t *testing.T) {
	tests := []struct {
		stability float64
		want      string
	}{
		{0.5, "Fragile"},
		{2.0, "Building"},
		{10.0, "Solid"},
		{30.0, "Strong"},
		{100.0, "Strong"},
	}

	for _, tt := range tests {
		got := StabilityLabel(tt.stability)
		if got != tt.want {
			t.Errorf("StabilityLabel(%f) = %q, want %q", tt.stability, got, tt.want)
		}
	}
}

func TestDifficultyLabel(t *testing.T) {
	tests := []struct {
		difficulty float64
		want       string
	}{
		{0.1, "Easy"},
		{0.4, "Medium"},
		{0.7, "Hard"},
		{0.95, "Hard"},
	}

	for _, tt := range tests {
		got := DifficultyLabel(tt.difficulty)
		if got != tt.want {
			t.Errorf("DifficultyLabel(%f) = %q, want %q", tt.difficulty, got, tt.want)
		}
	}
}

func TestRelativeDue(t *testing.T) {
	now := time.Now()

	tests := []struct {
		due  time.Time
		want string
	}{
		{now.Add(-1 * time.Hour), "Now"},
		{now.Add(30 * time.Minute), "30m"},
		{now.Add(2 * time.Hour), "2h 0m"},
		{now.Add(25 * time.Hour), "Tomorrow"},
		{now.Add(3 * 24 * time.Hour), "3 days"},
		{now.Add(14 * 24 * time.Hour), "2 weeks"},
	}

	for _, tt := range tests {
		got := RelativeDue(tt.due)
		if got != tt.want {
			t.Errorf("RelativeDue(%v) = %q, want %q", tt.due.Sub(now), got, tt.want)
		}
	}
}

func TestStateLabel(t *testing.T) {
	tests := []struct {
		state int
		want  string
	}{
		{0, "New"},
		{1, "Learning"},
		{2, "Review"},
		{3, "Relearning"},
	}

	for _, tt := range tests {
		got := StateLabel(tt.state)
		if got != tt.want {
			t.Errorf("StateLabel(%d) = %q, want %q", tt.state, got, tt.want)
		}
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd brain && go test ./internal/fsrs -run "Test(StabilityLabel|DifficultyLabel|RelativeDue|StateLabel)" -v`

Expected: FAIL - functions don't exist

**Step 3: Implement the label helpers**

```go
// brain/internal/fsrs/labels.go
package fsrs

import (
	"fmt"
	"time"
)

// StabilityLabel converts FSRS stability (days to 90% retention) to a friendly label
func StabilityLabel(stability float64) string {
	switch {
	case stability < 1:
		return "Fragile"
	case stability < 7:
		return "Building"
	case stability < 21:
		return "Solid"
	default:
		return "Strong"
	}
}

// DifficultyLabel converts FSRS difficulty (0-1 scale) to a friendly label
func DifficultyLabel(difficulty float64) string {
	switch {
	case difficulty < 0.3:
		return "Easy"
	case difficulty < 0.6:
		return "Medium"
	default:
		return "Hard"
	}
}

// RelativeDue converts a due time to a human-readable relative string
func RelativeDue(due time.Time) string {
	now := time.Now()
	diff := due.Sub(now)

	if diff <= 0 {
		return "Now"
	}

	hours := diff.Hours()
	minutes := int(diff.Minutes()) % 60

	switch {
	case hours < 1:
		return fmt.Sprintf("%dm", int(diff.Minutes()))
	case hours < 24:
		return fmt.Sprintf("%dh %dm", int(hours), minutes)
	case hours < 48:
		return "Tomorrow"
	case hours < 24*14:
		return fmt.Sprintf("%d days", int(hours/24))
	default:
		return fmt.Sprintf("%d weeks", int(hours/24/7))
	}
}

// StateLabel converts FSRS state int to a friendly label
func StateLabel(state int) string {
	switch state {
	case 0:
		return "New"
	case 1:
		return "Learning"
	case 2:
		return "Review"
	case 3:
		return "Relearning"
	default:
		return "Unknown"
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd brain && go test ./internal/fsrs -run "Test(StabilityLabel|DifficultyLabel|RelativeDue|StateLabel)" -v`

Expected: PASS

**Step 5: Commit**

```bash
git add brain/internal/fsrs/labels.go brain/internal/fsrs/labels_test.go
git commit -m "feat(fsrs): add user-friendly label helpers"
```

---

## Task 3: Create Home Screen Model

**Files:**
- Create: `brain/internal/gym/home.go`
- Test: `brain/internal/gym/home_test.go`

**Step 1: Write the failing tests**

```go
// brain/internal/gym/home_test.go
package gym

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHomeModelView(t *testing.T) {
	stats := &HomeStats{
		DueNow:        0,
		DueToday:      1,
		TotalCards:    5,
		NextDue:       timePtr(time.Now().Add(3 * time.Hour)),
		WordsLearned:  12,
		RetentionRate: 0.87,
		ReviewsToday:  3,
		Streak:        5,
		Last7Days:     []int{1, 3, 5, 2, 4, 1, 3},
		DraftCount:    2,
	}

	model := NewHomeModel(stats)
	view := model.View()

	// Check that key elements are present
	if !strings.Contains(view, "Due now: 0") {
		t.Error("View should show due now count")
	}
	if !strings.Contains(view, "Due today: 1") {
		t.Error("View should show due today count")
	}
	if !strings.Contains(view, "Total cards: 5") {
		t.Error("View should show total cards")
	}
	if !strings.Contains(view, "Words learned: 12") {
		t.Error("View should show words learned")
	}
	if !strings.Contains(view, "87%") {
		t.Error("View should show retention rate")
	}
	if !strings.Contains(view, "Streak: 5") {
		t.Error("View should show streak")
	}
	if !strings.Contains(view, "[r]") {
		t.Error("View should show review shortcut")
	}
	if !strings.Contains(view, "[t]") {
		t.Error("View should show triage shortcut")
	}
	if !strings.Contains(view, "(2 drafts)") {
		t.Error("View should show draft count")
	}
}

func TestHomeModelKeyHandling(t *testing.T) {
	stats := &HomeStats{DraftCount: 2}
	model := NewHomeModel(stats)

	tests := []struct {
		key      string
		wantMode HomeAction
	}{
		{"r", ActionReview},
		{"t", ActionTriage},
		{"c", ActionCards},
		{"q", ActionQuit},
	}

	for _, tt := range tests {
		m := model
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
		newModel, _ := m.Update(msg)
		hm := newModel.(HomeModel)

		if hm.action != tt.wantMode {
			t.Errorf("Key %q: action = %v, want %v", tt.key, hm.action, tt.wantMode)
		}
	}
}

func TestHomeModelEmptyState(t *testing.T) {
	stats := &HomeStats{
		TotalCards: 0,
		DraftCount: 0,
	}

	model := NewHomeModel(stats)
	view := model.View()

	if !strings.Contains(view, "No cards yet") {
		t.Error("Empty state should show 'No cards yet' message")
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
```

**Step 2: Run tests to verify they fail**

Run: `cd brain && go test ./internal/gym -run "TestHomeModel" -v`

Expected: FAIL - HomeModel doesn't exist

**Step 3: Implement the home model**

```go
// brain/internal/gym/home.go
package gym

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mitchelldurbin/polybius/brain/internal/fsrs"
)

var (
	homeTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Width(52)

	boxTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	statLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	statValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	homeHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
)

// HomeAction represents what the user wants to do after leaving home screen
type HomeAction int

const (
	ActionNone HomeAction = iota
	ActionReview
	ActionTriage
	ActionCards
	ActionQuit
)

// HomeStats holds all the data needed to render the home screen
type HomeStats struct {
	DueNow        int
	DueToday      int
	TotalCards    int
	NextDue       *time.Time
	WordsLearned  int
	RetentionRate float64
	ReviewsToday  int
	Streak        int
	Last7Days     []int
	DraftCount    int
}

type HomeModel struct {
	stats    *HomeStats
	action   HomeAction
	quitting bool
}

func NewHomeModel(stats *HomeStats) HomeModel {
	return HomeModel{
		stats:  stats,
		action: ActionNone,
	}
}

func (m HomeModel) Init() tea.Cmd {
	return nil
}

func (m HomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.action = ActionReview
			return m, tea.Quit
		case "t":
			m.action = ActionTriage
			return m, tea.Quit
		case "c":
			m.action = ActionCards
			return m, tea.Quit
		case "q", "ctrl+c":
			m.action = ActionQuit
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m HomeModel) View() string {
	if m.quitting {
		return ""
	}

	var sb strings.Builder

	// Title
	sb.WriteString(homeTitleStyle.Render("POLYBIUS GYM"))
	sb.WriteString("\n\n")

	// Empty state
	if m.stats.TotalCards == 0 && m.stats.DraftCount == 0 {
		sb.WriteString(emptyStyle.Render("No cards yet - capture some moments with the Miner!"))
		sb.WriteString("\n\n")
		sb.WriteString(homeHelpStyle.Render("  [q] Quit"))
		return sb.String()
	}

	// Reviews box
	sb.WriteString(m.renderReviewsBox())
	sb.WriteString("\n")

	// Progress box
	sb.WriteString(m.renderProgressBox())
	sb.WriteString("\n\n")

	// Navigation
	sb.WriteString(m.renderNavigation())

	return sb.String()
}

func (m HomeModel) renderReviewsBox() string {
	var content strings.Builder

	content.WriteString(boxTitleStyle.Render("─ Reviews "))
	content.WriteString(boxTitleStyle.Render(strings.Repeat("─", 40)))
	content.WriteString("\n")

	// Stats line
	content.WriteString(fmt.Sprintf("  %s %s    %s %s    %s %s\n",
		statLabelStyle.Render("Due now:"),
		statValueStyle.Render(fmt.Sprintf("%d", m.stats.DueNow)),
		statLabelStyle.Render("Due today:"),
		statValueStyle.Render(fmt.Sprintf("%d", m.stats.DueToday)),
		statLabelStyle.Render("Total cards:"),
		statValueStyle.Render(fmt.Sprintf("%d", m.stats.TotalCards)),
	))

	// Next review line
	nextDueStr := "—"
	if m.stats.NextDue != nil {
		if m.stats.NextDue.Before(time.Now()) {
			nextDueStr = "Now"
		} else {
			relDue := fsrs.RelativeDue(*m.stats.NextDue)
			nextDueStr = fmt.Sprintf("%s (%s)", relDue, m.stats.NextDue.Format("3:04 PM"))
		}
	}
	content.WriteString(fmt.Sprintf("  %s %s",
		statLabelStyle.Render("Next review:"),
		statValueStyle.Render(nextDueStr),
	))

	return boxStyle.Render(content.String())
}

func (m HomeModel) renderProgressBox() string {
	var content strings.Builder

	content.WriteString(boxTitleStyle.Render("─ Progress "))
	content.WriteString(boxTitleStyle.Render(strings.Repeat("─", 39)))
	content.WriteString("\n")

	// Stats line 1
	retentionStr := "—"
	if m.stats.RetentionRate > 0 {
		retentionStr = fmt.Sprintf("%.0f%%", m.stats.RetentionRate*100)
	}
	content.WriteString(fmt.Sprintf("  %s %s    %s %s\n",
		statLabelStyle.Render("Words learned:"),
		statValueStyle.Render(fmt.Sprintf("%d", m.stats.WordsLearned)),
		statLabelStyle.Render("Retention:"),
		statValueStyle.Render(retentionStr),
	))

	// Stats line 2
	content.WriteString(fmt.Sprintf("  %s %s     %s %s\n",
		statLabelStyle.Render("Reviews today:"),
		statValueStyle.Render(fmt.Sprintf("%d", m.stats.ReviewsToday)),
		statLabelStyle.Render("Streak:"),
		statValueStyle.Render(fmt.Sprintf("%d days", m.stats.Streak)),
	))

	// Sparkline
	content.WriteString(fmt.Sprintf("\n  %s %s",
		statLabelStyle.Render("Last 7 days:"),
		m.renderSparkline(m.stats.Last7Days),
	))

	return boxStyle.Render(content.String())
}

func (m HomeModel) renderSparkline(counts []int) string {
	if len(counts) == 0 {
		return "—"
	}

	// Find max for scaling
	max := 0
	for _, c := range counts {
		if c > max {
			max = c
		}
	}

	if max == 0 {
		return strings.Repeat("▁", len(counts))
	}

	// Sparkline characters from lowest to highest
	chars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	var spark strings.Builder
	for _, c := range counts {
		idx := (c * (len(chars) - 1)) / max
		spark.WriteRune(chars[idx])
	}

	return spark.String()
}

func (m HomeModel) renderNavigation() string {
	var parts []string

	parts = append(parts, "[r] Review")

	if m.stats.DraftCount > 0 {
		parts = append(parts, fmt.Sprintf("[t] Triage (%d drafts)", m.stats.DraftCount))
	} else {
		parts = append(parts, "[t] Triage")
	}

	parts = append(parts, "[c] Cards")
	parts = append(parts, "[q] Quit")

	return homeHelpStyle.Render("  " + strings.Join(parts, "    "))
}

// Action returns the action the user selected
func (m HomeModel) Action() HomeAction {
	return m.action
}
```

**Step 4: Run tests to verify they pass**

Run: `cd brain && go test ./internal/gym -run "TestHomeModel" -v`

Expected: PASS

**Step 5: Commit**

```bash
git add brain/internal/gym/home.go brain/internal/gym/home_test.go
git commit -m "feat(gym): add home screen model"
```

---

## Task 4: Create Cards List Model

**Files:**
- Create: `brain/internal/gym/cards.go`
- Test: `brain/internal/gym/cards_test.go`

**Step 1: Write the failing tests**

```go
// brain/internal/gym/cards_test.go
package gym

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCardsModelView(t *testing.T) {
	now := time.Now()
	cards := []*CardListItem{
		{
			ID:         1,
			TargetWord: "超市",
			Sentence:   "我今天去了超市",
			DueDate:    timePtr(now.Add(3 * time.Hour)),
			Stability:  15.0,
			Difficulty: 0.2,
			Reps:       4,
			State:      "review",
		},
		{
			ID:         2,
			TargetWord: "电影院",
			Sentence:   "我们去电影院看电影",
			DueDate:    timePtr(now.Add(24 * time.Hour)),
			Stability:  5.0,
			Difficulty: 0.5,
			Reps:       2,
			State:      "learning",
		},
	}

	model := NewCardsModel(cards)
	view := model.View()

	// Check that cards are displayed
	if !strings.Contains(view, "超市") {
		t.Error("View should show first card's target word")
	}
	if !strings.Contains(view, "电影院") {
		t.Error("View should show second card's target word")
	}
	if !strings.Contains(view, "Stability:") {
		t.Error("View should show stability")
	}
	if !strings.Contains(view, "Difficulty:") {
		t.Error("View should show difficulty")
	}
}

func TestCardsModelNavigation(t *testing.T) {
	cards := []*CardListItem{
		{ID: 1, TargetWord: "word1"},
		{ID: 2, TargetWord: "word2"},
		{ID: 3, TargetWord: "word3"},
	}

	model := NewCardsModel(cards)

	// Move down
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	newModel, _ := model.Update(msg)
	cm := newModel.(CardsModel)
	if cm.cursor != 1 {
		t.Errorf("After j, cursor = %d, want 1", cm.cursor)
	}

	// Move up
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	newModel, _ = cm.Update(msg)
	cm = newModel.(CardsModel)
	if cm.cursor != 0 {
		t.Errorf("After k, cursor = %d, want 0", cm.cursor)
	}
}

func TestCardsModelEscape(t *testing.T) {
	cards := []*CardListItem{{ID: 1, TargetWord: "word1"}}
	model := NewCardsModel(cards)

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, cmd := model.Update(msg)
	cm := newModel.(CardsModel)

	if !cm.goBack {
		t.Error("Escape should set goBack to true")
	}
	if cmd == nil {
		t.Error("Escape should return tea.Quit command")
	}
}

func TestCardsModelEmpty(t *testing.T) {
	model := NewCardsModel(nil)
	view := model.View()

	if !strings.Contains(view, "No cards") {
		t.Error("Empty state should show 'No cards' message")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd brain && go test ./internal/gym -run "TestCardsModel" -v`

Expected: FAIL - CardsModel doesn't exist

**Step 3: Implement the cards list model**

```go
// brain/internal/gym/cards.go
package gym

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mitchelldurbin/polybius/brain/internal/fsrs"
)

var (
	cardsTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	cardListStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Width(64)

	selectedCardStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")).
				Bold(true)

	unselectedCardStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	cardDetailStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(5)

	cardMetaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			PaddingLeft(5)

	cardsHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	cardsEmptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
)

// CardListItem represents a card in the list view
type CardListItem struct {
	ID         int64
	TargetWord string
	Sentence   string
	DueDate    *time.Time
	Stability  float64
	Difficulty float64
	Reps       int
	State      string
}

type CardsModel struct {
	cards  []*CardListItem
	cursor int
	goBack bool
}

func NewCardsModel(cards []*CardListItem) CardsModel {
	return CardsModel{
		cards: cards,
	}
}

func (m CardsModel) Init() tea.Cmd {
	return nil
}

func (m CardsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.cards)-1 {
				m.cursor++
			}
		case "esc":
			m.goBack = true
			return m, tea.Quit
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m CardsModel) View() string {
	var sb strings.Builder

	// Title
	sb.WriteString(cardsTitleStyle.Render("ALL CARDS"))
	sb.WriteString("\n\n")

	if len(m.cards) == 0 {
		sb.WriteString(cardsEmptyStyle.Render("No cards yet."))
		sb.WriteString("\n\n")
		sb.WriteString(cardsHelpStyle.Render("  [Esc] Back"))
		return sb.String()
	}

	// Card list
	var listContent strings.Builder
	for i, card := range m.cards {
		style := unselectedCardStyle
		prefix := "  "
		if i == m.cursor {
			style = selectedCardStyle
			prefix = "> "
		}

		// Card header
		listContent.WriteString(style.Render(fmt.Sprintf("%s%d. %s", prefix, i+1, card.TargetWord)))
		listContent.WriteString("\n")

		// Sentence preview (truncated)
		sentence := card.Sentence
		if len(sentence) > 35 {
			sentence = sentence[:35] + "..."
		}
		listContent.WriteString(cardDetailStyle.Render(fmt.Sprintf("\"%s\"", sentence)))
		listContent.WriteString(" · ")

		// Due time
		dueStr := "—"
		if card.DueDate != nil {
			if card.DueDate.Before(time.Now()) {
				dueStr = "Due: Now"
			} else {
				dueStr = "Due: " + fsrs.RelativeDue(*card.DueDate)
			}
		}
		listContent.WriteString(cardDetailStyle.Render(dueStr))
		listContent.WriteString("\n")

		// FSRS details
		stabilityLabel := fsrs.StabilityLabel(card.Stability)
		difficultyLabel := fsrs.DifficultyLabel(card.Difficulty)
		listContent.WriteString(cardMetaStyle.Render(fmt.Sprintf(
			"Stability: %s · Difficulty: %s · Reviews: %d",
			stabilityLabel, difficultyLabel, card.Reps,
		)))
		listContent.WriteString("\n\n")
	}

	sb.WriteString(cardListStyle.Render(listContent.String()))
	sb.WriteString("\n")

	// Help
	sb.WriteString(cardsHelpStyle.Render("  [↑/↓] Navigate    [Esc] Back    [q] Quit"))

	return sb.String()
}

// GoBack returns true if the user pressed escape to go back
func (m CardsModel) GoBack() bool {
	return m.goBack
}
```

**Step 4: Run tests to verify they pass**

Run: `cd brain && go test ./internal/gym -run "TestCardsModel" -v`

Expected: PASS

**Step 5: Commit**

```bash
git add brain/internal/gym/cards.go brain/internal/gym/cards_test.go
git commit -m "feat(gym): add cards list model"
```

---

## Task 5: Add GetAllCards Query to Storage

**Files:**
- Modify: `brain/internal/storage/models.go`
- Modify: `brain/internal/storage/models_test.go`

**Step 1: Write the failing test**

Add to `brain/internal/storage/models_test.go`:

```go
func TestGetAllCards(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	momentID, _ := db.InsertMoment(&Moment{
		Timestamp: time.Now().Format(time.RFC3339),
		AudioFile: "test.wav",
		RawText:   "test sentence",
		Status:    "processed",
	})

	now := time.Now()
	future := now.Add(3 * time.Hour)

	// Insert cards with various states
	db.InsertCard(&Card{MomentID: momentID, TargetWord: "word1", State: "review", DueDate: &future, Stability: 10.0, Difficulty: 0.3})
	db.InsertCard(&Card{MomentID: momentID, TargetWord: "word2", State: "learning", Stability: 2.0, Difficulty: 0.5})
	db.InsertCard(&Card{MomentID: momentID, TargetWord: "word3", State: "draft"}) // Should be excluded

	cards, err := db.GetAllCards()
	if err != nil {
		t.Fatalf("GetAllCards failed: %v", err)
	}

	if len(cards) != 2 {
		t.Errorf("len(cards) = %d, want 2 (excluding drafts)", len(cards))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd brain && go test ./internal/storage -run "TestGetAllCards" -v`

Expected: FAIL - GetAllCards doesn't exist

**Step 3: Implement GetAllCards**

Add to `brain/internal/storage/models.go`:

```go
// GetAllCards returns all non-draft cards ordered by due date
func (db *DB) GetAllCards() ([]*Card, error) {
	rows, err := db.Query(`
		SELECT id, moment_id, target_word, target_pinyin, target_definition,
		       stability, difficulty, due_date, last_review, reps, lapses, state
		FROM cards
		WHERE state != 'draft'
		ORDER BY due_date ASC NULLS FIRST`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []*Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd brain && go test ./internal/storage -run "TestGetAllCards" -v`

Expected: PASS

**Step 5: Commit**

```bash
git add brain/internal/storage/models.go brain/internal/storage/models_test.go
git commit -m "feat(storage): add GetAllCards query"
```

---

## Task 6: Integrate Home Screen into Main

**Files:**
- Modify: `brain/cmd/polybius/main.go`
- Modify: `brain/internal/gym/review.go`

**Step 1: Add LoadHomeStats to Session**

Add to `brain/internal/gym/review.go`:

```go
// HomeStatsStore abstracts database operations for home screen stats
type HomeStatsStore interface {
	GetCardStats() (*storage.CardStats, error)
	GetNextDueTime() (*time.Time, error)
	GetReviewStats(days int) (*storage.ReviewStats, error)
	GetStreak() (int, error)
	GetReviewsPerDay(days int) ([]int, error)
	CountLearnedWords() (int, error)
	CountDraftCards() (int, error)
}

// LoadHomeStats loads all statistics needed for the home screen
func (s *Session) LoadHomeStats() (*HomeStats, error) {
	cardStats, err := s.store.(HomeStatsStore).GetCardStats()
	if err != nil {
		return nil, err
	}

	nextDue, err := s.store.(HomeStatsStore).GetNextDueTime()
	if err != nil {
		return nil, err
	}

	reviewStats, err := s.store.(HomeStatsStore).GetReviewStats(30)
	if err != nil {
		return nil, err
	}

	streak, err := s.store.(HomeStatsStore).GetStreak()
	if err != nil {
		return nil, err
	}

	last7Days, err := s.store.(HomeStatsStore).GetReviewsPerDay(7)
	if err != nil {
		return nil, err
	}

	wordsLearned, err := s.store.(HomeStatsStore).CountLearnedWords()
	if err != nil {
		return nil, err
	}

	draftCount, err := s.store.(HomeStatsStore).CountDraftCards()
	if err != nil {
		return nil, err
	}

	return &HomeStats{
		DueNow:        cardStats.DueNow,
		DueToday:      cardStats.DueToday,
		TotalCards:    cardStats.TotalCards,
		NextDue:       nextDue,
		WordsLearned:  wordsLearned,
		RetentionRate: reviewStats.RetentionRate,
		ReviewsToday:  reviewStats.ReviewsToday,
		Streak:        streak,
		Last7Days:     last7Days,
		DraftCount:    draftCount,
	}, nil
}

// GetAllCardsForList returns cards formatted for the cards list view
func (s *Session) GetAllCardsForList() ([]*CardListItem, error) {
	store, ok := s.store.(interface {
		GetAllCards() ([]*storage.Card, error)
		GetMoment(id int64) (*storage.Moment, error)
	})
	if !ok {
		return nil, fmt.Errorf("store does not support GetAllCards")
	}

	cards, err := store.GetAllCards()
	if err != nil {
		return nil, err
	}

	var items []*CardListItem
	for _, c := range cards {
		moment, err := store.GetMoment(c.MomentID)
		sentence := ""
		if err == nil {
			sentence = moment.RawText
		}

		items = append(items, &CardListItem{
			ID:         c.ID,
			TargetWord: c.TargetWord,
			Sentence:   sentence,
			DueDate:    c.DueDate,
			Stability:  c.Stability,
			Difficulty: c.Difficulty,
			Reps:       c.Reps,
			State:      c.State,
		})
	}

	return items, nil
}
```

Add the missing import at the top of `review.go`:
```go
import (
	"fmt"
	"time"
	// ... existing imports
)
```

**Step 2: Update main.go to use home screen**

Replace the `runGym` function in `brain/cmd/polybius/main.go`:

```go
func runGym(cfg *config.Config) {
	db, err := storage.OpenDatabase(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	session := gym.NewSession(db)

	for {
		// Load stats and show home screen
		stats, err := session.LoadHomeStats()
		if err != nil {
			log.Fatalf("Failed to load stats: %v", err)
		}

		homeModel := gym.NewHomeModel(stats)
		p := tea.NewProgram(homeModel)
		finalModel, err := p.Run()
		if err != nil {
			log.Fatalf("Error running home screen: %v", err)
		}

		hm := finalModel.(gym.HomeModel)
		action := hm.Action()

		switch action {
		case gym.ActionQuit:
			return

		case gym.ActionTriage:
			runTriage(db)

		case gym.ActionReview:
			runReview(db, session)

		case gym.ActionCards:
			runCardsList(session)
		}
	}
}

func runTriage(db *storage.DB) {
	triageModel := gym.NewTriageModel(db)
	p := tea.NewProgram(triageModel)
	p.Run()
}

func runReview(db *storage.DB, session *gym.Session) {
	cards, err := session.GetDueCards(20)
	if err != nil {
		log.Printf("Failed to get cards: %v", err)
		return
	}

	if len(cards) == 0 {
		fmt.Println("No cards due for review.")
		time.Sleep(1 * time.Second)
		return
	}

	onRate := func(cardID int64, rating int) error {
		return session.SubmitRating(cardID, rating)
	}

	model := gym.NewModel(cards, onRate)
	p := tea.NewProgram(model)
	p.Run()
}

func runCardsList(session *gym.Session) {
	cards, err := session.GetAllCardsForList()
	if err != nil {
		log.Printf("Failed to get cards: %v", err)
		return
	}

	model := gym.NewCardsModel(cards)
	p := tea.NewProgram(model)
	p.Run()
}
```

Add imports at top of `main.go`:
```go
import (
	"fmt"
	"time"
	// ... existing imports
)
```

**Step 3: Run the application to verify it works**

Run: `cd brain && make run-gym`

Expected: Home screen displays with stats and navigation options

**Step 4: Commit**

```bash
git add brain/cmd/polybius/main.go brain/internal/gym/review.go
git commit -m "feat(gym): integrate home screen as entry point"
```

---

## Task 7: Run Full Test Suite and Fix Issues

**Step 1: Run all tests**

Run: `cd brain && make test`

**Step 2: Fix any failing tests**

Address any compilation errors or test failures.

**Step 3: Manual testing checklist**

- [ ] Launch gym with empty database - shows "No cards yet" message
- [ ] Launch gym with cards - shows stats
- [ ] Press `r` - goes to review mode (or shows "no cards due")
- [ ] Press `Esc` from review - returns to home
- [ ] Press `t` - goes to triage mode
- [ ] Press `Esc` from triage - returns to home
- [ ] Press `c` - shows cards list with FSRS details
- [ ] Press `Esc` from cards - returns to home
- [ ] Press `q` - quits application

**Step 4: Commit any fixes**

```bash
git add -A
git commit -m "fix: address test failures and edge cases"
```

---

## Summary

| Task | Files | Purpose |
|------|-------|---------|
| 1 | `storage/stats.go` | Stats queries (due counts, retention, streak) |
| 2 | `fsrs/labels.go` | User-friendly FSRS labels |
| 3 | `gym/home.go` | Home screen model |
| 4 | `gym/cards.go` | Cards list model |
| 5 | `storage/models.go` | GetAllCards query |
| 6 | `main.go`, `review.go` | Wire everything together |
| 7 | — | Testing and fixes |

Total: ~7 commits, ~500 lines of new code
