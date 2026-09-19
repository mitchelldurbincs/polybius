package gym

import (
	"context"
	"errors"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
)

func TestSkritterAdditionRunsAsCommand(t *testing.T) {
	called := false
	m := NewCardsModel([]*CardListItem{{TargetWord: "学习", Pinyin: "xue2 xi2"}}).WithWordAdder(func(_ context.Context, word, reading string) (string, error) {
		called = true
		if word != "学习" || reading != "xue2 xi2" {
			t.Error("wrong word")
		}
		return "", errors.New("offline")
	})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(CardsModel)
	if called || cmd == nil || !m.adding {
		t.Fatal("network operation blocked Update")
	}
	_, duplicate := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if duplicate != nil {
		t.Fatal("duplicate in-flight command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(CardsModel)
	if !called || m.adding || !strings.Contains(m.View(), "offline") {
		t.Fatal("missing failure feedback")
	}
	_, retry := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if retry == nil {
		t.Fatal("cannot retry")
	}
}
