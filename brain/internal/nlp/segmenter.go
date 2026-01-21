// brain/internal/nlp/segmenter.go
package nlp

import (
	"github.com/go-ego/gse"
)

type Segmenter struct {
	seg gse.Segmenter
}

func NewSegmenter() (*Segmenter, error) {
	var seg gse.Segmenter
	// Load default dictionary (embedded)
	if err := seg.LoadDict(); err != nil {
		return nil, err
	}
	return &Segmenter{seg: seg}, nil
}

func (s *Segmenter) Segment(text string) []string {
	// Use accurate mode for better word boundaries
	return s.seg.Cut(text, true)
}

func (s *Segmenter) SegmentSearch(text string) []string {
	// Use search mode (finer granularity)
	return s.seg.CutSearch(text, true)
}
