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
