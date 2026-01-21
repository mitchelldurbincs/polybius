// brain/internal/brain/enricher_test.go
package brain

import (
	"path/filepath"
	"runtime"
	"testing"
)

// testDataPath returns the path to the test data directory
func testDataPath(filename string) string {
	_, currentFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(currentFile)
	return filepath.Join(dir, "..", "..", "data", filename)
}

func TestEnrichText(t *testing.T) {
	e, err := NewEnricher(testDataPath("cedict_ts.u8"))
	if err != nil {
		t.Fatalf("Failed to create enricher: %v", err)
	}

	result := e.Enrich("你好世界")

	if len(result.Words) == 0 {
		t.Error("Expected words to be segmented")
	}

	// Check that we got pinyin and definitions
	found := false
	for _, w := range result.Words {
		if w.Word == "你好" {
			found = true
			if w.Pinyin == "" {
				t.Error("Expected pinyin for 你好")
			}
			if w.Definition == "" {
				t.Error("Expected definition for 你好")
			}
		}
	}
	if !found {
		t.Error("Expected to find 你好 in segmented words")
	}
}

func TestI1Scoring(t *testing.T) {
	e, err := NewEnricher(testDataPath("cedict_ts.u8"))
	if err != nil {
		t.Fatalf("Failed to create enricher: %v", err)
	}

	// Mock vocabulary: user knows 你好 but not 世界
	knownWords := map[string]bool{"你好": true}
	isKnown := func(word string) bool { return knownWords[word] }

	result := e.EnrichWithVocab("你好世界", isKnown)

	if result.I1Score < 0.4 || result.I1Score > 0.6 {
		t.Errorf("Expected i+1 score around 0.5 (1 of 2 known), got %f", result.I1Score)
	}

	if len(result.UnknownWords) != 1 {
		t.Errorf("Expected 1 unknown word, got %d", len(result.UnknownWords))
	}

	if result.UnknownWords[0] != "世界" {
		t.Errorf("Expected unknown word to be 世界, got %s", result.UnknownWords[0])
	}
}

func TestNormalizeChinese(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Basic case: spaces between Chinese characters
		{"你 好 世 界", "你好世界"},
		// Mixed: Chinese and punctuation (space before punct preserved - punct isn't Han)
		{"你 好 ！", "你好 ！"},
		// Already normalized
		{"你好世界", "你好世界"},
		// English with spaces (should preserve)
		{"hello world", "hello world"},
		// Mixed Chinese and English (preserve space at boundary)
		{"你好 hello", "你好 hello"},
		// Real Windows OCR example (space before 。 preserved since it's not Han)
		{"我 的 爸 爸 很 酷 。", "我的爸爸很酷 。"},
	}

	for _, tt := range tests {
		got := normalizeChinese(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeChinese(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEnrichWithSpacedText(t *testing.T) {
	e, err := NewEnricher(testDataPath("cedict_ts.u8"))
	if err != nil {
		t.Fatalf("Failed to create enricher: %v", err)
	}

	// Windows OCR style input with spaces between characters
	result := e.Enrich("爸 爸")

	// Should be segmented as one word, not two
	found := false
	for _, w := range result.Words {
		if w.Word == "爸爸" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 爸爸 as single word, got words: %v", result.Words)
	}
}

func TestEnrichBabaFullSentence(t *testing.T) {
	e, err := NewEnricher(testDataPath("cedict_ts.u8"))
	if err != nil {
		t.Fatalf("Failed to create enricher: %v", err)
	}

	// The exact sentence from the user's Windows OCR
	result := e.Enrich("我 的 爸 爸 很 酷 。")

	// Check that 爸爸 is a single word
	var words []string
	for _, w := range result.Words {
		words = append(words, w.Word)
	}

	// Should contain 爸爸 as one word, not two 爸
	foundBaba := false
	foundSingleBa := false
	for _, word := range words {
		if word == "爸爸" {
			foundBaba = true
		}
		if word == "爸" {
			foundSingleBa = true
		}
	}

	if !foundBaba {
		t.Errorf("Expected 爸爸 as single word in %v", words)
	}
	if foundSingleBa {
		t.Errorf("Should NOT have single 爸 character, got words: %v", words)
	}
}
