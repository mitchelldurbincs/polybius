package fsrs

import (
	"fmt"
	"time"
)

func ExampleRelativeDueEdgeCases() {
	now := time.Now()

	// Test boundary at 24 hours (should be "Tomorrow")
	testCases := []time.Time{
		now.Add(24 * time.Hour),      // Exactly 24 hours
		now.Add(48 * time.Hour),      // Exactly 48 hours (should be "2 days" not "Tomorrow")
		now.Add(24 * 14 * time.Hour), // Exactly 14 days
	}

	for _, tc := range testCases {
		result := RelativeDue(tc)
		fmt.Printf("Due in %v: %s\n", tc.Sub(now), result)
	}
}
