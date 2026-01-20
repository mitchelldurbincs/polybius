// brain/internal/vocab/import.go
package vocab

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mitchelldurbin/polybius/brain/internal/storage"
)

type ImportResult struct {
	Added   int
	Skipped int
}

func ImportTSV(db *storage.DB, filePath string) (*ImportResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	result := &ImportResult{}
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			fmt.Printf("! line %d: expected 4 columns, got %d (skipped)\n", lineNum, len(parts))
			continue
		}

		simplified := strings.TrimSpace(parts[0])
		pinyin := strings.TrimSpace(parts[2])
		definition := strings.TrimSpace(parts[3])

		// Skip if empty word
		if simplified == "" {
			continue
		}

		// Check if already known
		if db.IsWordKnown(simplified) {
			fmt.Printf("= %s (already known)\n", simplified)
			result.Skipped++
			continue
		}

		// Upsert as known
		v := &storage.Vocabulary{
			Word:       simplified,
			Pinyin:     pinyin,
			Definition: definition,
			Status:     "known",
		}

		if err := db.UpsertVocabulary(v); err != nil {
			return nil, fmt.Errorf("failed to insert word %s: %w", simplified, err)
		}

		fmt.Printf("+ %s (%s) - %s\n", simplified, pinyin, truncate(definition, 50))
		result.Added++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return result, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
