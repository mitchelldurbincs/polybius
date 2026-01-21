// brain/internal/nlp/cedict.go
package nlp

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

type DictEntry struct {
	Traditional string
	Simplified  string
	Pinyin      string
	Definition  string
}

type CEDICT struct {
	entries map[string]*DictEntry // keyed by simplified
}

// LoadCEDICT loads the CC-CEDICT dictionary file
func LoadCEDICT(path string) (*CEDICT, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dict := &CEDICT{
		entries: make(map[string]*DictEntry),
	}

	// Pattern: 傳統 简体 [pin1 yin1] /definition 1/definition 2/
	linePattern := regexp.MustCompile(`^(\S+)\s+(\S+)\s+\[([^\]]+)\]\s+/(.+)/$`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		matches := linePattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		entry := &DictEntry{
			Traditional: matches[1],
			Simplified:  matches[2],
			Pinyin:      convertPinyinTones(matches[3]),
			Definition:  strings.ReplaceAll(matches[4], "/", "; "),
		}

		// Store by simplified (primary key for Chinese learners)
		dict.entries[entry.Simplified] = entry
	}

	return dict, scanner.Err()
}

// Lookup finds a word in the dictionary
func (d *CEDICT) Lookup(word string) (*DictEntry, bool) {
	entry, ok := d.entries[word]
	return entry, ok
}

// LookupWithFallback tries the word, then each character individually
func (d *CEDICT) LookupWithFallback(word string) (*DictEntry, bool) {
	if entry, ok := d.Lookup(word); ok {
		return entry, true
	}

	// Fallback: try to build from individual characters
	runes := []rune(word)
	if len(runes) == 1 {
		return nil, false
	}

	var pinyins, defs []string
	for _, r := range runes {
		char := string(r)
		if entry, ok := d.Lookup(char); ok {
			pinyins = append(pinyins, entry.Pinyin)
			defs = append(defs, entry.Definition)
		} else {
			return nil, false
		}
	}

	return &DictEntry{
		Simplified: word,
		Pinyin:     strings.Join(pinyins, " "),
		Definition: strings.Join(defs, "; "),
	}, true
}

var toneMarks = map[rune][]rune{
	'a': {'ā', 'á', 'ǎ', 'à', 'a'},
	'e': {'ē', 'é', 'ě', 'è', 'e'},
	'i': {'ī', 'í', 'ǐ', 'ì', 'i'},
	'o': {'ō', 'ó', 'ǒ', 'ò', 'o'},
	'u': {'ū', 'ú', 'ǔ', 'ù', 'u'},
	'ü': {'ǖ', 'ǘ', 'ǚ', 'ǜ', 'ü'},
}

// convertPinyinTones converts numbered pinyin (ni3 hao3) to tone marks (nǐ hǎo)
func convertPinyinTones(numbered string) string {
	words := strings.Fields(strings.ToLower(numbered))
	var result []string

	for _, word := range words {
		result = append(result, convertSyllable(word))
	}

	return strings.Join(result, " ")
}

func convertSyllable(syllable string) string {
	// Handle ü written as v
	syllable = strings.ReplaceAll(syllable, "v", "ü")

	// Find the tone number (1-5) at the end
	if len(syllable) == 0 {
		return syllable
	}

	lastChar := syllable[len(syllable)-1]
	if lastChar < '1' || lastChar > '5' {
		return syllable
	}

	tone := int(lastChar - '1') // 0-4
	base := syllable[:len(syllable)-1]

	// Find the vowel to mark (rule: a/e always marked, otherwise last vowel)
	runes := []rune(base)
	markIndex := -1

	for i, r := range runes {
		if r == 'a' || r == 'e' {
			markIndex = i
			break
		}
		if r == 'i' || r == 'o' || r == 'u' || r == 'ü' {
			markIndex = i
		}
	}

	if markIndex == -1 {
		return base
	}

	// Apply tone mark
	vowel := runes[markIndex]
	if marks, ok := toneMarks[vowel]; ok && tone < len(marks) {
		runes[markIndex] = marks[tone]
	}

	return string(runes)
}

// Size returns the number of entries in the dictionary
func (d *CEDICT) Size() int {
	return len(d.entries)
}
