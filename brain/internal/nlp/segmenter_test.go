// brain/internal/nlp/segmenter_test.go
package nlp

import (
	"testing"
)

func TestSegmentChinese(t *testing.T) {
	seg, err := NewSegmenter()
	if err != nil {
		t.Fatalf("Failed to create segmenter: %v", err)
	}

	tests := []struct {
		input    string
		expected []string
	}{
		{"你好世界", []string{"你好", "世界"}},
		{"我今天去超市", []string{"我", "今天", "去", "超市"}},
	}

	for _, tt := range tests {
		got := seg.Segment(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("Segment(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("Segment(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}
