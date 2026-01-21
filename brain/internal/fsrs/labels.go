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
