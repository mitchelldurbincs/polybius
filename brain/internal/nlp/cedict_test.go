// brain/internal/nlp/cedict_test.go
package nlp

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

func TestCEDICTLookup(t *testing.T) {
	dict, err := LoadCEDICT(testDataPath("cedict_ts.u8"))
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
	dict, err := LoadCEDICT(testDataPath("cedict_ts.u8"))
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

func TestPinyinToneConversion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ni3 hao3", "nǐ hǎo"},
		{"zhong1 guo2", "zhōng guó"},
		{"ma1 ma2 ma3 ma4 ma5", "mā má mǎ mà ma"},
		{"nv3", "nǚ"},
		{"lv4", "lǜ"},
	}

	for _, tt := range tests {
		got := convertPinyinTones(tt.input)
		if got != tt.expected {
			t.Errorf("convertPinyinTones(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
