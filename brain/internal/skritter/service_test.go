package skritter

import (
	"context"
	"errors"
	"testing"
)

type fakeAPI struct {
	lists         []List
	list          List
	section       Section
	vocabs        []Vocab
	creates, puts int
	readErr       error
	ignoreWrite   bool
}

func (f *fakeAPI) Lists(context.Context) ([]List, error)          { return f.lists, f.readErr }
func (f *fakeAPI) GetList(context.Context, string) (*List, error) { return &f.list, nil }
func (f *fakeAPI) CreateList(context.Context) (*List, error) {
	f.creates++
	f.lists = []List{f.list}
	return &f.list, nil
}
func (f *fakeAPI) Search(context.Context, Word) ([]Vocab, error) { return f.vocabs, nil }
func (f *fakeAPI) GetSection(context.Context, string, string) (*Section, error) {
	return &f.section, nil
}
func (f *fakeAPI) PutRows(_ context.Context, _, _ string, rows []Row) error {
	f.puts++
	if !f.ignoreWrite {
		f.section.Rows = rows
	}
	return nil
}
func fixture() *fakeAPI {
	section := Section{ID: "section", Name: "Words", Rows: []Row{newRow("old", "old")}}
	return &fakeAPI{list: List{ID: "list", Name: "Polybius", Lang: "zh", Sections: []Section{section}}, section: section, vocabs: []Vocab{{ID: "hello", Writing: "你好", Reading: "ni3hao3", Style: "both"}}}
}
func TestAddCreatesPreservesAndDeduplicates(t *testing.T) {
	f := fixture()
	s := NewService(f)
	ctx := context.Background()
	r, err := s.AddWords(ctx, []Word{{Writing: "你好"}, {Writing: "你好"}})
	if err != nil || r.Added != 1 || r.Existing != 1 || f.creates != 1 || f.puts != 1 {
		t.Fatalf("result=%+v err=%v creates=%d puts=%d", r, err, f.creates, f.puts)
	}
	if f.section.Rows[0].id("vocabId") != "old" {
		t.Fatal("existing word lost")
	}
	r, err = s.AddWords(ctx, []Word{{Writing: "你好"}})
	if err != nil || r.Added != 0 || r.Existing != 1 || f.creates != 1 || f.puts != 1 {
		t.Fatalf("retry: %+v %v", r, err)
	}
}
func TestFailedListReadNeverCreates(t *testing.T) {
	f := fixture()
	f.readErr = errors.New("offline")
	_, err := NewService(f).EnsureList(context.Background())
	if err == nil || f.creates != 0 {
		t.Fatal("created after failed read")
	}
}
func TestDuplicateListsAreRejected(t *testing.T) {
	f := fixture()
	f.lists = []List{f.list, f.list}
	_, err := NewService(f).EnsureList(context.Background())
	if err == nil || f.creates != 0 {
		t.Fatal("duplicate names were accepted")
	}
}
func TestSilentWriteFailureIsNotSuccess(t *testing.T) {
	f := fixture()
	f.ignoreWrite = true
	r, err := NewService(f).AddWords(context.Background(), []Word{{Writing: "你好"}})
	if err == nil || r.Added != 0 {
		t.Fatalf("false success: %+v %v", r, err)
	}
}
func TestUnmatchedBatchDoesNotWrite(t *testing.T) {
	f := fixture()
	_, err := NewService(f).AddWords(context.Background(), []Word{{Writing: "你好"}, {Writing: "不存在"}})
	if err == nil || f.puts != 0 || f.creates != 0 {
		t.Fatal("partially wrote unresolved batch")
	}
}
func TestDuplicateInAnotherSection(t *testing.T) {
	f := fixture()
	f.list.Sections = append(f.list.Sections, Section{ID: "other", Rows: []Row{newRow("hello", "hello")}})
	f.lists = []List{f.list}
	r, err := NewService(f).AddWords(context.Background(), []Word{{Writing: "你好"}})
	if err != nil || r.Existing != 1 || f.puts != 0 {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestResolveVariantsAndAmbiguity(t *testing.T) {
	defs := map[string]string{"en": "study"}
	vocabs := []Vocab{{ID: "simp", Writing: "学习", Reading: "xue2xi2", Style: "simp", Definitions: defs}, {ID: "trad", Writing: "學習", Reading: "xue2xi2", Style: "trad", Definitions: defs}}
	for _, word := range []string{"学习", "學習"} {
		r, err := resolve(Word{word, "xue2 xi2"}, vocabs)
		if err != nil || r.id("vocabId") != "simp" || r.id("tradVocabId") != "trad" {
			t.Fatalf("%v %v", r, err)
		}
	}
	vocabs = append(vocabs, Vocab{ID: "other", Writing: "学习", Reading: "xue2xi2", Style: "simp", Definitions: defs})
	if _, err := resolve(Word{Writing: "学习"}, vocabs); err == nil {
		t.Fatal("ambiguous match accepted")
	}
}
