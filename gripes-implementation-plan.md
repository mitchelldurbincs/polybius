# Gripes Implementation Plan

> Implementation details for the three critical improvements identified in gripes.md

---

## 1. Draft/Triage System (Anti-Burnout Firewall)

### Problem
Current flow auto-creates active cards for every unknown word. Capturing 50 moments = 100+ cards in your review queue overnight.

### Solution
Cards start as drafts. User triages before they enter the FSRS scheduler.

### Schema Changes

```sql
-- Modify cards table: add 'draft' and 'suspended' states
-- Current states: 'new', 'learning', 'review', 'relearning'
-- New states: 'draft', 'new', 'learning', 'review', 'relearning', 'suspended'

-- No migration needed - 'state' is already TEXT
-- Just use 'draft' as the initial state instead of 'new'
```

### Brain Service Changes

**File:** `brain/internal/brain/service.go`

```go
// In handleNewCapture(), change card creation:
card := &storage.Card{
    MomentID:         momentID,
    TargetWord:       unknownWord,
    TargetPinyin:     pinyin,
    TargetDefinition: definition,
    State:            "draft",  // Changed from "new"
    DueDate:          nil,      // No due date until approved
}
```

### Storage Changes

**File:** `brain/internal/storage/models.go`

```go
// Add new query methods:

func (db *DB) GetDraftCards(limit int) ([]*Card, error) {
    rows, err := db.Query(`
        SELECT c.id, c.moment_id, c.target_word, c.target_pinyin, c.target_definition,
               c.stability, c.difficulty, c.due_date, c.last_review, c.reps, c.lapses, c.state
        FROM cards c
        WHERE c.state = 'draft'
        ORDER BY c.created_at DESC
        LIMIT ?`, limit)
    // ... same scanning logic as GetDueCards
}

func (db *DB) ApproveCard(cardID int64) error {
    now := time.Now().Format(time.RFC3339)
    _, err := db.Exec(`
        UPDATE cards
        SET state = 'new', due_date = ?
        WHERE id = ? AND state = 'draft'`, now, cardID)
    return err
}

func (db *DB) DeleteCard(cardID int64) error {
    _, err := db.Exec(`DELETE FROM cards WHERE id = ?`, cardID)
    return err
}

func (db *DB) GetDraftCardsByMoment(momentID int64) ([]*Card, error) {
    // Get all draft cards for a specific moment (for grouped triage view)
}

func (db *DB) ApproveMoment(momentID int64) error {
    // Approve all draft cards for a moment at once
    now := time.Now().Format(time.RFC3339)
    _, err := db.Exec(`
        UPDATE cards
        SET state = 'new', due_date = ?
        WHERE moment_id = ? AND state = 'draft'`, now, momentID)
    return err
}

func (db *DB) DeleteMoment(momentID int64) error {
    // Delete moment and all associated cards
    _, err := db.Exec(`DELETE FROM cards WHERE moment_id = ?`, momentID)
    if err != nil {
        return err
    }
    _, err = db.Exec(`DELETE FROM moments WHERE id = ?`, momentID)
    return err
}
```

### Gym TUI Changes

**File:** `brain/internal/gym/triage.go` (new file)

```go
package gym

// TriageModel is the bubbletea model for the triage screen
type TriageModel struct {
    moments    []*MomentWithCards  // Grouped by moment
    cursor     int
    expanded   map[int64]bool      // Which moments are expanded to show cards
    counts     TriageCounts
}

type MomentWithCards struct {
    Moment *storage.Moment
    Cards  []*storage.Card
}

type TriageCounts struct {
    Draft    int
    Approved int
    Deleted  int
}

// View renders:
// ┌─────────────────────────────────────────────────────────────┐
// │  TRIAGE                           Draft: 47  │  Ready: 12   │
// ├─────────────────────────────────────────────────────────────┤
// │                                                             │
// │  ▸ [3 cards] 你听到了吗                     Jan 19, 8:42pm  │
// │  ▸ [2 cards] 我今天很忙                     Jan 19, 8:45pm  │
// │  ▾ [1 card]  他不知道                       Jan 19, 8:51pm  │
// │      └─ 知道 (zhīdào) - to know                             │
// │                                                             │
// │  [Enter] Expand  [A] Approve  [D] Delete  [R] Review  [Q] Quit│
// └─────────────────────────────────────────────────────────────┘

func (m TriageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "up", "k":
            m.cursor--
        case "down", "j":
            m.cursor++
        case "enter":
            // Toggle expand/collapse moment
        case "a":
            // Approve current moment (all its cards)
        case "A":
            // Approve ALL draft moments
        case "d":
            // Delete current moment
        case "r":
            // Switch to review mode (only if approved cards exist)
        case "q":
            return m, tea.Quit
        }
    }
    return m, nil
}
```

**File:** `brain/cmd/polybius/main.go`

```go
func runGym() {
    // ...

    // Check for drafts first
    draftCount, _ := db.CountDraftCards()

    if draftCount > 0 {
        // Start with triage
        triageModel := gym.NewTriageModel(db)
        p := tea.NewProgram(triageModel)
        finalModel, _ := p.Run()

        // After triage, check if user wants to review
        if finalModel.(gym.TriageModel).StartReview {
            // Continue to review...
        }
    } else {
        // No drafts, go straight to review
    }
}
```

### User Flow

```
$ polybius gym

# If drafts exist, triage screen appears first:
TRIAGE                               Draft: 47  │  Ready: 12

▸ [3 cards] 你听到了吗                          Jan 19, 8:42pm
▸ [2 cards] 我今天很忙                          Jan 19, 8:45pm

[A] Approve  [D] Delete  [R] Start Review  [Q] Quit

# User approves good ones, deletes bad ones
# Press R to start reviewing approved cards
```

---

## 2. Audio-First Card State (Visual Crutch Killer)

### Problem
If Hanzi is visible when audio plays, the brain reads instead of listens. This defeats the goal of training listening comprehension.

### Solution
Three-state reveal: Audio-only → Hanzi → Full (Pinyin/Definition)

### Gym TUI Changes

**File:** `brain/internal/gym/tui.go`

```go
type RevealState int

const (
    StateAudioOnly RevealState = iota  // Screenshot + audio, no text
    StateHanziRevealed                  // + Hanzi sentence visible
    StateFullRevealed                   // + Pinyin + Definition
)

type Model struct {
    cards       []*ReviewCard
    currentIdx  int
    revealState RevealState  // Changed from bool 'revealed'
    quitting    bool
    // ...
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case " ": // Space advances reveal state
            switch m.revealState {
            case StateAudioOnly:
                m.revealState = StateHanziRevealed
            case StateHanziRevealed:
                m.revealState = StateFullRevealed
            case StateFullRevealed:
                // Already fully revealed, space does nothing
            }
            return m, nil

        case "1", "2", "3", "4":
            // Can only rate after at least seeing Hanzi
            if m.revealState >= StateHanziRevealed {
                // Process rating...
            }
            return m, nil
        }
    }
    return m, nil
}

func (m Model) View() string {
    card := m.cards[m.currentIdx]

    var content string

    switch m.revealState {
    case StateAudioOnly:
        // Only show that audio is playing, no text
        content += "Listen to the audio...\n\n"
        content += fmt.Sprintf("Audio: [▶ Playing...]\n")
        content += "\n" + hiddenStyle.Render("[Press Space to reveal what was said]")

    case StateHanziRevealed:
        // Show the sentence with target highlighted
        content += fmt.Sprintf("Sentence: %s\n\n", highlightWord(card.Sentence, card.TargetWord))
        content += fmt.Sprintf("Target: %s\n", targetStyle.Render(card.TargetWord))
        content += fmt.Sprintf("Pinyin: %s\n", hiddenStyle.Render("[hidden]"))
        content += fmt.Sprintf("Meaning: %s\n", hiddenStyle.Render("[hidden]"))

    case StateFullRevealed:
        // Show everything
        content += fmt.Sprintf("Sentence: %s\n\n", highlightWord(card.Sentence, card.TargetWord))
        content += fmt.Sprintf("Target: %s\n", targetStyle.Render(card.TargetWord))
        content += fmt.Sprintf("Pinyin: %s\n", card.Pinyin)
        content += fmt.Sprintf("Meaning: %s\n", card.Definition)
    }

    // Help text changes based on state
    var help string
    switch m.revealState {
    case StateAudioOnly:
        help = "[Space] Reveal Hanzi  [R] Replay  [Q] Quit"
    case StateHanziRevealed:
        help = "[Space] Show Meaning  [R] Replay  [1-4] Rate  [Q] Quit"
    case StateFullRevealed:
        help = "[R] Replay  [1-4] Rate  [Q] Quit"
    }

    return fmt.Sprintf("%s\n\n%s\n\n%s\n", header, cardStyle.Render(content), helpStyle.Render(help))
}
```

### Self-Grading Prompt

Add a prompt at Hanzi reveal to encourage self-assessment:

```go
case StateHanziRevealed:
    content += fmt.Sprintf("Sentence: %s\n\n", highlightWord(card.Sentence, card.TargetWord))
    content += fmt.Sprintf("Target: %s\n", targetStyle.Render(card.TargetWord))
    content += "\n" + promptStyle.Render("Did you hear it correctly? Rate now or [Space] for meaning")
```

### User Flow

```
# State 0: Audio plays, screen shows screenshot only
┌─────────────────────────────────────────────────────────────┐
│  POLYBIUS GYM                          Due: 12  │  New: 3   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Listen to the audio...                                     │
│                                                             │
│  Audio: [▶ Playing... 0:03/0:10]                           │
│                                                             │
│  [Press Space to reveal what was said]                      │
│                                                             │
│  [Space] Reveal  [R] Replay  [Q] Quit                       │
└─────────────────────────────────────────────────────────────┘

# User presses Space → State 1: Hanzi revealed
┌─────────────────────────────────────────────────────────────┐
│  Sentence: 你 听到 了 吗                                     │
│                ^^^^                                         │
│  Target: 听到                                                │
│  Pinyin: [hidden]                                           │
│  Meaning: [hidden]                                          │
│                                                             │
│  Did you hear it correctly?                                 │
│                                                             │
│  [Space] Show Meaning  [R] Replay  [1-4] Rate  [Q] Quit     │
└─────────────────────────────────────────────────────────────┘

# User can rate now (if confident) or press Space again for full reveal
```

---

## 3. In-Place Segmentation Fixes (Hot-Fix Capability)

### Problem
Segmenters make mistakes. `看不起` → `看|不|起` creates a "leech card" you fail repeatedly due to bad data.

### Solution
Allow merging/splitting tokens during review with a quick hotkey.

### Schema Changes

```sql
-- Add custom segmentation override to cards
ALTER TABLE cards ADD COLUMN custom_segments TEXT;
-- JSON array, e.g., ["看不起"] to override the moment's segmentation for this card
-- NULL means use moment's default segmentation
```

### Storage Changes

**File:** `brain/internal/storage/models.go`

```go
type Card struct {
    // ... existing fields
    CustomSegments []string  // Override segmentation for this card
}

func (db *DB) UpdateCardSegments(cardID int64, segments []string) error {
    segJSON, _ := json.Marshal(segments)
    _, err := db.Exec(`
        UPDATE cards SET custom_segments = ? WHERE id = ?`,
        string(segJSON), cardID)
    return err
}
```

### Gym TUI Changes

**File:** `brain/internal/gym/edit.go` (new file)

```go
package gym

type EditModel struct {
    card       *ReviewCard
    tokens     []string      // Current token list
    cursor     int           // Which token is selected
    selectMode bool          // Are we selecting a range to merge?
    selectStart int
    parent     *Model        // To return to after editing
}

func NewEditModel(card *ReviewCard, parent *Model) EditModel {
    // Parse current segments from card or moment
    tokens := card.Segments
    if len(tokens) == 0 {
        tokens = strings.Fields(card.Sentence) // fallback
    }
    return EditModel{
        card:   card,
        tokens: tokens,
        cursor: 0,
        parent: parent,
    }
}

func (m EditModel) View() string {
    // Show tokens with cursor
    // ┌─────────────────────────────────────────────────────────────┐
    // │  EDIT SEGMENTATION                                          │
    // ├─────────────────────────────────────────────────────────────┤
    // │                                                             │
    // │  [ 看 ] [ 不 ] [ 起 ]                                       │
    // │    ^^^^^^^^^^^^^^^                                          │
    // │    (selected for merge)                                     │
    // │                                                             │
    // │  [←→] Move  [Space] Select  [M] Merge  [S] Split  [Enter] Save│
    // └─────────────────────────────────────────────────────────────┘

    var tokenDisplay string
    for i, tok := range m.tokens {
        style := tokenStyle
        if m.selectMode && i >= m.selectStart && i <= m.cursor {
            style = selectedStyle
        } else if i == m.cursor {
            style = cursorStyle
        }
        tokenDisplay += style.Render(fmt.Sprintf("[ %s ]", tok)) + " "
    }

    return tokenDisplay
}

func (m EditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "left", "h":
            if m.cursor > 0 {
                m.cursor--
            }
        case "right", "l":
            if m.cursor < len(m.tokens)-1 {
                m.cursor++
            }
        case " ": // Start/extend selection
            if !m.selectMode {
                m.selectMode = true
                m.selectStart = m.cursor
            }
        case "m": // Merge selected tokens
            if m.selectMode {
                m.mergeTokens(m.selectStart, m.cursor)
                m.selectMode = false
            }
        case "s": // Split token at cursor (by character)
            m.splitToken(m.cursor)
        case "enter": // Save and return
            // Save to database
            m.parent.db.UpdateCardSegments(m.card.ID, m.tokens)
            return m.parent, nil
        case "escape", "q": // Cancel
            return m.parent, nil
        }
    }
    return m, nil
}

func (m *EditModel) mergeTokens(start, end int) {
    if start > end {
        start, end = end, start
    }
    merged := strings.Join(m.tokens[start:end+1], "")
    m.tokens = append(m.tokens[:start], append([]string{merged}, m.tokens[end+1:]...)...)
    m.cursor = start
}

func (m *EditModel) splitToken(idx int) {
    token := m.tokens[idx]
    runes := []rune(token)
    if len(runes) <= 1 {
        return // Can't split single character
    }
    var split []string
    for _, r := range runes {
        split = append(split, string(r))
    }
    m.tokens = append(m.tokens[:idx], append(split, m.tokens[idx+1:]...)...)
}
```

**File:** `brain/internal/gym/tui.go` - Add edit hotkey

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "e": // Enter edit mode
            return NewEditModel(m.cards[m.currentIdx], &m), nil
        // ... other cases
        }
    }
    return m, nil
}
```

### User Flow

```
# During review, user notices bad segmentation
Sentence: 看 不 起    ← Should be 看不起 (one word)

# Press 'e' to edit
┌─────────────────────────────────────────────────────────────┐
│  EDIT SEGMENTATION                                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  [ 看 ] [ 不 ] [ 起 ]                                       │
│  ^                                                          │
│                                                             │
│  [←→] Move  [Space] Start Select  [M] Merge  [S] Split     │
│  [Enter] Save  [Esc] Cancel                                 │
└─────────────────────────────────────────────────────────────┘

# User presses Space on 看, moves right to 起, presses M
┌─────────────────────────────────────────────────────────────┐
│  [ 看不起 ]                                                  │
│  ^                                                          │
│                                                             │
│  [Enter] Save  [Esc] Cancel                                 │
└─────────────────────────────────────────────────────────────┘

# Press Enter → saved, back to review
```

---

## Implementation Order

Recommended sequence:

1. **Audio-First (#2)** - Small change to existing TUI, high impact for learning goal
2. **Draft/Triage (#1)** - Requires schema awareness change + new TUI screen, prevents burnout
3. **Segmentation Fixes (#3)** - Nice-to-have, can be added after core flow works

### Phase Integration

These changes fit into the existing implementation plan:

| Existing Phase | Gripes Integration |
|----------------|-------------------|
| Phase 8 (Brain Service) | Create cards with `state='draft'` |
| Phase 9 (Gym TUI) | Add triage screen, audio-first states, edit mode |

---

## Testing Checklist

### Draft/Triage
- [ ] Cards created with state='draft'
- [ ] Draft cards don't appear in GetDueCards()
- [ ] Triage screen shows grouped drafts
- [ ] Approve moves cards to 'new' with due_date
- [ ] Delete removes card from DB
- [ ] Bulk approve works

### Audio-First
- [ ] Initial state shows no text
- [ ] First Space reveals Hanzi
- [ ] Second Space reveals Pinyin/Definition
- [ ] Can rate after Hanzi reveal
- [ ] Audio plays on card load

### Segmentation Fixes
- [ ] 'e' enters edit mode
- [ ] Can select range of tokens
- [ ] Merge combines tokens
- [ ] Split breaks into characters
- [ ] Changes persist to DB
- [ ] Review uses custom_segments if present
