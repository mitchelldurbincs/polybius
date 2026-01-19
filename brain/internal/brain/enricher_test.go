// brain/internal/brain/enricher_test.go
package brain

import (
	"testing"
)

func TestEnrichText(t *testing.T) {
	e, err := NewEnricher("../../data/cedict_ts.u8")
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
	e, err := NewEnricher("../../data/cedict_ts.u8")
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
