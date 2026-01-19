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
