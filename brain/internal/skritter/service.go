package skritter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"unicode"
)

type Word struct{ Writing, Reading string }
type Result struct {
	ListID          string
	Added, Existing int
}

func (r Result) String() string {
	return fmt.Sprintf("Skritter: %d added, %d already in Polybius", r.Added, r.Existing)
}

// API isolates list policy from HTTP and the terminal UI.
type API interface {
	Lists(context.Context) ([]List, error)
	GetList(context.Context, string) (*List, error)
	CreateList(context.Context) (*List, error)
	Search(context.Context, Word) ([]Vocab, error)
	GetSection(context.Context, string, string) (*Section, error)
	PutRows(context.Context, string, string, []Row) error
}
type Service struct {
	api API
	mu  sync.Mutex
}

func NewService(api API) *Service { return &Service{api: api} }

// EnsureList reuses exactly one active Chinese Polybius list. It never guesses
// between duplicate names, and never creates a replacement after a failed read.
func (s *Service) ensureList(ctx context.Context) (*List, error) {
	lists, err := s.api.Lists(ctx)
	if err != nil {
		return nil, err
	}
	var found *List
	for _, l := range lists {
		if l.Name == "Polybius" && l.Lang == "zh" && !l.Deleted && !l.Disabled {
			if found != nil {
				return nil, fmt.Errorf("multiple Polybius lists exist; rename the extra list in Skritter")
			}
			copy := l
			found = &copy
		}
	}
	if found == nil {
		return s.api.CreateList(ctx)
	}
	return s.api.GetList(ctx, found.ID)
}
func (s *Service) EnsureList(ctx context.Context) (*List, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureList(ctx)
}

// AddWords resolves the entire batch before writing. Existing rows (including
// unknown API fields) are retained. A timeout is not reported as success.
func (s *Service) AddWords(ctx context.Context, words []Word) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := Result{}
	if len(words) == 0 {
		return result, fmt.Errorf("provide at least one word")
	}
	rows := []Row{}
	for _, word := range words {
		word.Writing = strings.TrimSpace(word.Writing)
		word.Reading = strings.TrimSpace(word.Reading)
		if word.Writing == "" {
			return result, fmt.Errorf("word cannot be empty")
		}
		vocabs, err := s.api.Search(ctx, word)
		if err != nil {
			return result, err
		}
		row, err := resolve(word, vocabs)
		if err != nil {
			return result, err
		}
		rows = append(rows, row)
	}
	list, err := s.ensureList(ctx)
	if err != nil {
		return result, err
	}
	result.ListID = list.ID
	var target string
	existing := map[string]bool{}
	for _, sec := range list.Sections {
		if sec.Deleted {
			continue
		}
		if target == "" || sec.Name == "Words" {
			target = sec.ID
		}
		for _, row := range sec.Rows {
			existing[row.id("vocabId")] = true
			existing[row.id("tradVocabId")] = true
		}
	}
	if target == "" {
		return result, fmt.Errorf("Polybius has no section; create a Words section in Skritter")
	}
	// Fetch fresh rows immediately before replacing a section.
	section, err := s.api.GetSection(ctx, list.ID, target)
	if err != nil {
		return result, err
	}
	for _, row := range section.Rows {
		existing[row.id("vocabId")] = true
		existing[row.id("tradVocabId")] = true
	}
	combined := append([]Row{}, section.Rows...)
	for _, row := range rows {
		id := row.id("vocabId")
		if existing[id] {
			result.Existing++
			continue
		}
		existing[id] = true
		combined = append(combined, row)
	}
	added := len(combined) - len(section.Rows)
	if added == 0 {
		return result, nil
	}
	if err := s.api.PutRows(ctx, list.ID, target, combined); err != nil {
		return result, err
	}
	saved, err := s.api.GetSection(ctx, list.ID, target)
	if err != nil {
		return result, fmt.Errorf("update sent but verification failed: %w", err)
	}
	membership := map[string]bool{}
	for _, row := range saved.Rows {
		membership[row.id("vocabId")+"\x00"+row.id("tradVocabId")] = true
	}
	for _, row := range combined {
		if !membership[row.id("vocabId")+"\x00"+row.id("tradVocabId")] {
			return result, fmt.Errorf("Skritter did not retain all words; retry to check membership")
		}
	}
	result.Added = added
	return result, nil
}

func readingKey(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "u:", "ü")
	s = strings.ReplaceAll(s, "v", "ü")
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '\'' {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}
func resolve(word Word, vocabs []Vocab) (Row, error) {
	matches := []Vocab{}
	for _, v := range vocabs {
		if v.ID != "" && v.Writing == word.Writing && (word.Reading == "" || readingKey(v.Reading) == readingKey(word.Reading)) {
			matches = append(matches, v)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no exact Skritter match for %s (%s)", word.Writing, word.Reading)
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("ambiguous Skritter match for %s; specify pinyin", word.Writing)
	}
	v := matches[0]
	if v.Style == "both" {
		return newRow(v.ID, v.ID), nil
	}
	otherStyle := "trad"
	if v.Style == "trad" {
		otherStyle = "simp"
	} else if v.Style != "simp" {
		return nil, fmt.Errorf("unsupported writing style for %s", word.Writing)
	}
	variants := []Vocab{}
	for _, other := range vocabs {
		if other.Style == otherStyle && readingKey(other.Reading) == readingKey(v.Reading) && other.Definitions["en"] == v.Definitions["en"] && other.ID != "" {
			variants = append(variants, other)
		}
	}
	if len(variants) != 1 {
		return nil, fmt.Errorf("cannot determine simplified/traditional pair for %s", word.Writing)
	}
	if v.Style == "trad" {
		return newRow(variants[0].ID, v.ID), nil
	}
	return newRow(v.ID, variants[0].ID), nil
}
