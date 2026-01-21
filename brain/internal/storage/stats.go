// brain/internal/storage/stats.go
package storage

import (
	"fmt"
	"time"
)

// CardStats holds summary statistics about cards.
type CardStats struct {
	DueNow     int
	DueToday   int
	TotalCards int
}

// ReviewStats holds summary statistics about reviews.
type ReviewStats struct {
	TotalReviews  int
	RetentionRate float64
	ReviewsToday  int
}

// GetCardStats returns statistics about card due counts.
// TotalCards excludes draft cards.
// DueNow counts cards with due_date <= now.
// DueToday counts cards with due_date before end of today (includes DueNow).
func (db *DB) GetCardStats() (*CardStats, error) {
	stats := &CardStats{}
	now := time.Now()
	nowStr := now.Format(time.RFC3339)

	// Count total active cards (not drafts)
	err := db.QueryRow(`
		SELECT COUNT(*) FROM cards WHERE state != 'draft'
	`).Scan(&stats.TotalCards)
	if err != nil {
		return nil, fmt.Errorf("counting total cards: %w", err)
	}

	// Count cards due now (due_date <= now)
	err = db.QueryRow(`
		SELECT COUNT(*) FROM cards
		WHERE state != 'draft' AND due_date IS NOT NULL AND due_date <= ?
	`, nowStr).Scan(&stats.DueNow)
	if err != nil {
		return nil, fmt.Errorf("counting due now: %w", err)
	}

	// Count cards due today (due_date before end of today)
	// End of today = start of tomorrow at midnight in local time
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	tomorrowStr := tomorrow.Format(time.RFC3339)
	err = db.QueryRow(`
		SELECT COUNT(*) FROM cards
		WHERE state != 'draft' AND due_date IS NOT NULL AND due_date < ?
	`, tomorrowStr).Scan(&stats.DueToday)
	if err != nil {
		return nil, fmt.Errorf("counting due today: %w", err)
	}

	return stats, nil
}

// GetNextDueTime returns the next due time for any card, or nil if no cards are due.
// Only considers non-draft cards with future due dates.
func (db *DB) GetNextDueTime() (*time.Time, error) {
	nowStr := time.Now().Format(time.RFC3339)
	var dueDateStr *string
	err := db.QueryRow(`
		SELECT due_date FROM cards
		WHERE state != 'draft' AND due_date IS NOT NULL AND due_date > ?
		ORDER BY due_date ASC
		LIMIT 1
	`, nowStr).Scan(&dueDateStr)

	if err != nil {
		// No rows found means no future due cards
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("getting next due time: %w", err)
	}

	if dueDateStr == nil {
		return nil, nil
	}

	t, err := time.Parse(time.RFC3339, *dueDateStr)
	if err != nil {
		return nil, fmt.Errorf("parsing due date: %w", err)
	}
	return &t, nil
}

// GetReviewStats returns review statistics for the past N days.
// RetentionRate is the proportion of reviews with rating >= 3 (Good or Easy).
func (db *DB) GetReviewStats(days int) (*ReviewStats, error) {
	stats := &ReviewStats{}

	cutoff := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)

	// Count total reviews in period
	err := db.QueryRow(`
		SELECT COUNT(*) FROM reviews WHERE reviewed_at >= ?
	`, cutoff).Scan(&stats.TotalReviews)
	if err != nil {
		return nil, fmt.Errorf("counting total reviews: %w", err)
	}

	// Count successful reviews (rating >= 3 means Good or Easy)
	var successCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM reviews WHERE reviewed_at >= ? AND rating >= 3
	`, cutoff).Scan(&successCount)
	if err != nil {
		return nil, fmt.Errorf("counting successful reviews: %w", err)
	}

	if stats.TotalReviews > 0 {
		stats.RetentionRate = float64(successCount) / float64(stats.TotalReviews)
	}

	// Count reviews today
	todayStart := time.Now().Truncate(24 * time.Hour).Format(time.RFC3339)
	err = db.QueryRow(`
		SELECT COUNT(*) FROM reviews WHERE reviewed_at >= ?
	`, todayStart).Scan(&stats.ReviewsToday)
	if err != nil {
		return nil, fmt.Errorf("counting today's reviews: %w", err)
	}

	return stats, nil
}

// GetStreak returns the number of consecutive days with at least one review,
// counting backwards from today.
func (db *DB) GetStreak() (int, error) {
	// Get all unique dates with reviews, ordered descending
	// Extract date part from RFC3339 format (first 10 characters: YYYY-MM-DD)
	rows, err := db.Query(`
		SELECT DISTINCT substr(reviewed_at, 1, 10) as review_date
		FROM reviews
		ORDER BY review_date DESC
	`)
	if err != nil {
		return 0, fmt.Errorf("querying review dates: %w", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return 0, fmt.Errorf("scanning date: %w", err)
		}
		dates = append(dates, d)
	}

	if len(dates) == 0 {
		return 0, nil
	}

	// Count consecutive days starting from today
	today := time.Now()
	streak := 0

	for i, dateStr := range dates {
		expectedDate := today.AddDate(0, 0, -i).Format("2006-01-02")
		if dateStr == expectedDate {
			streak++
		} else {
			break
		}
	}

	return streak, nil
}

// GetReviewsPerDay returns review counts for the past N days.
// The slice is ordered chronologically: index 0 is N-1 days ago, last index is today.
func (db *DB) GetReviewsPerDay(days int) ([]int, error) {
	counts := make([]int, days)

	// Calculate cutoff date
	cutoff := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)

	// Query reviews grouped by date for the past N days
	// Extract date part from RFC3339 format (first 10 characters: YYYY-MM-DD)
	rows, err := db.Query(`
		SELECT substr(reviewed_at, 1, 10) as review_date, COUNT(*) as count
		FROM reviews
		WHERE reviewed_at >= ?
		GROUP BY review_date
		ORDER BY review_date ASC
	`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("querying reviews per day: %w", err)
	}
	defer rows.Close()

	// Build a map of date -> count
	reviewCounts := make(map[string]int)
	for rows.Next() {
		var dateStr string
		var count int
		if err := rows.Scan(&dateStr, &count); err != nil {
			return nil, fmt.Errorf("scanning review count: %w", err)
		}
		reviewCounts[dateStr] = count
	}

	// Fill in the counts array
	today := time.Now()
	for i := 0; i < days; i++ {
		// i=0 is (days-1) days ago, i=(days-1) is today
		date := today.AddDate(0, 0, -(days-1-i)).Format("2006-01-02")
		counts[i] = reviewCounts[date]
	}

	return counts, nil
}

// CountLearnedWords returns the count of words with status "known".
func (db *DB) CountLearnedWords() (int, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM vocabulary WHERE status = 'known'
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting learned words: %w", err)
	}
	return count, nil
}

// InsertReview inserts a review record for a card.
func (db *DB) InsertReview(cardID int64, rating int) error {
	_, err := db.Exec(`
		INSERT INTO reviews (card_id, rating)
		VALUES (?, ?)
	`, cardID, rating)
	if err != nil {
		return fmt.Errorf("inserting review: %w", err)
	}
	return nil
}

// insertReviewAt inserts a review record at a specific time (for testing).
func (db *DB) insertReviewAt(cardID int64, rating int, reviewedAt time.Time) error {
	_, err := db.Exec(`
		INSERT INTO reviews (card_id, rating, reviewed_at)
		VALUES (?, ?, ?)
	`, cardID, rating, reviewedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("inserting review at time: %w", err)
	}
	return nil
}
