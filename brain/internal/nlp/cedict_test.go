// brain/internal/nlp/cedict_test.go
package nlp

import (
	"testing"
)

func TestCEDICTLookup(t *testing.T) {
	dict, err := LoadCEDICT("../../data/cedict_ts.u8")
	if err != nil {
		t.Fatalf("Failed to load CEDICT: %v", err)
	}

	// Test basic lookup
	entry, ok := dict.Lookup("你好")
	if !ok {
		t.Fatal("Failed to find 你好")
	}
	if entry.Pinyin == "" {
		t.Error("Pinyin should not be empty")
	}
	if entry.Definition == "" {
		t.Error("Definition should not be empty")
	}

	t.Logf("你好: %s - %s", entry.Pinyin, entry.Definition)
}

func TestCEDICTCharacterFallback(t *testing.T) {
	dict, err := LoadCEDICT("../../data/cedict_ts.u8")
	if err != nil {
		t.Fatalf("Failed to load CEDICT: %v", err)
	}

	// Test single character lookup
	entry, ok := dict.Lookup("你")
	if !ok {
		t.Fatal("Failed to find 你")
	}
	if entry.Pinyin == "" {
		t.Error("Pinyin should not be empty for single character")
	}
}
