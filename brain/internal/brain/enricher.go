// brain/internal/brain/enricher.go
package brain

import (
	"github.com/mitchelldurbin/polybius/brain/internal/nlp"
)

type EnrichedWord struct {
	Word       string
	Pinyin     string
	Definition string
}

type EnrichedText struct {
	RawText string
	Words   []EnrichedWord
}

type Enricher struct {
	segmenter *nlp.Segmenter
	dict      *nlp.CEDICT
}

func NewEnricher(cedictPath string) (*Enricher, error) {
	seg, err := nlp.NewSegmenter()
	if err != nil {
		return nil, err
	}

	dict, err := nlp.LoadCEDICT(cedictPath)
	if err != nil {
		return nil, err
	}

	return &Enricher{
		segmenter: seg,
		dict:      dict,
	}, nil
}

func (e *Enricher) Enrich(text string) *EnrichedText {
	words := e.segmenter.Segment(text)

	var enriched []EnrichedWord
	for _, word := range words {
		ew := EnrichedWord{Word: word}

		if entry, ok := e.dict.LookupWithFallback(word); ok {
			ew.Pinyin = entry.Pinyin
			ew.Definition = entry.Definition
		}

		enriched = append(enriched, ew)
	}

	return &EnrichedText{
		RawText: text,
		Words:   enriched,
	}
}

type EnrichedTextWithVocab struct {
	*EnrichedText
	KnownWords   []string
	UnknownWords []string
	I1Score      float64
}

func (e *Enricher) EnrichWithVocab(text string, isKnown func(string) bool) *EnrichedTextWithVocab {
	base := e.Enrich(text)

	var known, unknown []string
	for _, w := range base.Words {
		// Skip empty words
		if len([]rune(w.Word)) == 0 {
			continue
		}

		if isKnown(w.Word) {
			known = append(known, w.Word)
		} else {
			unknown = append(unknown, w.Word)
		}
	}

	total := len(known) + len(unknown)
	var score float64
	if total > 0 {
		score = float64(len(known)) / float64(total)
	}

	return &EnrichedTextWithVocab{
		EnrichedText: base,
		KnownWords:   known,
		UnknownWords: unknown,
		I1Score:      score,
	}
}

// IsI1 returns true if there's exactly one unknown word (ideal learning condition)
func (e *EnrichedTextWithVocab) IsI1() bool {
	return len(e.UnknownWords) == 1
}

// IsLearnable returns true if 80-99% of words are known
func (e *EnrichedTextWithVocab) IsLearnable() bool {
	return e.I1Score >= 0.8 && e.I1Score < 1.0
}
