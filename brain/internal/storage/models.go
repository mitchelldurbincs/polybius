// brain/internal/storage/models.go
package storage

import (
	"database/sql"
	"encoding/json"
	"time"
)

type Moment struct {
	ID             int64
	Timestamp      string
	AudioFile      string
	ScreenshotFile string
	RawText        string
	SegmentedWords []string
	I1Score        float64
	Status         string
	CreatedAt      time.Time
}

type Card struct {
	ID               int64
	MomentID         int64
	TargetWord       string
	TargetPinyin     string
	TargetDefinition string
	Stability        float64
	Difficulty       float64
	DueDate          *time.Time
	LastReview       *time.Time
	Reps             int
	Lapses           int
	State            string
	CreatedAt        time.Time
}

type Vocabulary struct {
	Word         string
	Pinyin       string
	Definition   string
	Status       string
	TimesSeen    int
	TimesCorrect int
	UpdatedAt    time.Time
}

func (db *DB) InsertMoment(m *Moment) (int64, error) {
	segJSON, _ := json.Marshal(m.SegmentedWords)
	result, err := db.Exec(`
		INSERT INTO moments (timestamp, audio_file, screenshot_file, raw_text, segmented_json, i1_score, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.Timestamp, m.AudioFile, m.ScreenshotFile, m.RawText, string(segJSON), m.I1Score, m.Status,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) GetMoment(id int64) (*Moment, error) {
	m := &Moment{}
	var segJSON sql.NullString
	var screenshot sql.NullString
	var rawText sql.NullString
	var i1Score sql.NullFloat64
	var createdAt string

	err := db.QueryRow(`
		SELECT id, timestamp, audio_file, screenshot_file, raw_text, segmented_json, i1_score, status, created_at
		FROM moments WHERE id = ?`, id).Scan(
		&m.ID, &m.Timestamp, &m.AudioFile, &screenshot, &rawText, &segJSON, &i1Score, &m.Status, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	m.ScreenshotFile = screenshot.String
	m.RawText = rawText.String
	m.I1Score = i1Score.Float64
	m.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)

	if segJSON.Valid {
		_ = json.Unmarshal([]byte(segJSON.String), &m.SegmentedWords)
	}

	return m, nil
}

// scanCardRow is a helper interface for scanning card rows from either *sql.Row or *sql.Rows
type scannable interface {
	Scan(dest ...interface{}) error
}

// scanCard scans a card from a row, handling nullable time fields
func scanCard(row scannable) (*Card, error) {
	c := &Card{}
	var dueDate, lastReview sql.NullString

	err := row.Scan(&c.ID, &c.MomentID, &c.TargetWord, &c.TargetPinyin, &c.TargetDefinition,
		&c.Stability, &c.Difficulty, &dueDate, &lastReview, &c.Reps, &c.Lapses, &c.State)
	if err != nil {
		return nil, err
	}

	if dueDate.Valid {
		t, _ := time.Parse(time.RFC3339, dueDate.String)
		c.DueDate = &t
	}
	if lastReview.Valid {
		t, _ := time.Parse(time.RFC3339, lastReview.String)
		c.LastReview = &t
	}

	return c, nil
}

func (db *DB) InsertCard(c *Card) (int64, error) {
	var dueDate, lastReview interface{}
	if c.DueDate != nil {
		dueDate = c.DueDate.Format(time.RFC3339)
	}
	if c.LastReview != nil {
		lastReview = c.LastReview.Format(time.RFC3339)
	}

	result, err := db.Exec(`
		INSERT INTO cards (moment_id, target_word, target_pinyin, target_definition, stability, difficulty, due_date, last_review, reps, lapses, state)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.MomentID, c.TargetWord, c.TargetPinyin, c.TargetDefinition,
		c.Stability, c.Difficulty, dueDate, lastReview, c.Reps, c.Lapses, c.State,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) GetDueCards(limit int) ([]*Card, error) {
	rows, err := db.Query(`
		SELECT c.id, c.moment_id, c.target_word, c.target_pinyin, c.target_definition,
		       c.stability, c.difficulty, c.due_date, c.last_review, c.reps, c.lapses, c.state
		FROM cards c
		WHERE c.due_date IS NULL OR c.due_date <= datetime('now')
		ORDER BY c.due_date ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []*Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, nil
}

func (db *DB) GetVocabulary(word string) (*Vocabulary, error) {
	v := &Vocabulary{}
	err := db.QueryRow(`
		SELECT word, pinyin, definition, status, times_seen, times_correct, updated_at
		FROM vocabulary WHERE word = ?`, word).Scan(
		&v.Word, &v.Pinyin, &v.Definition, &v.Status, &v.TimesSeen, &v.TimesCorrect, &v.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (db *DB) UpsertVocabulary(v *Vocabulary) error {
	_, err := db.Exec(`
		INSERT INTO vocabulary (word, pinyin, definition, status, times_seen, times_correct, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(word) DO UPDATE SET
			pinyin = excluded.pinyin,
			definition = excluded.definition,
			status = excluded.status,
			times_seen = excluded.times_seen,
			times_correct = excluded.times_correct,
			updated_at = datetime('now')`,
		v.Word, v.Pinyin, v.Definition, v.Status, v.TimesSeen, v.TimesCorrect,
	)
	return err
}

func (db *DB) IsWordKnown(word string) bool {
	var status string
	err := db.QueryRow("SELECT status FROM vocabulary WHERE word = ?", word).Scan(&status)
	if err != nil {
		return false
	}
	return status == "known"
}

func (db *DB) GetCard(id int64) (*Card, error) {
	row := db.QueryRow(`
		SELECT id, moment_id, target_word, target_pinyin, target_definition,
		       stability, difficulty, due_date, last_review, reps, lapses, state
		FROM cards WHERE id = ?`, id)
	return scanCard(row)
}

type CardUpdate struct {
	Stability  float64
	Difficulty float64
	Reps       int
	Lapses     int
	State      string
	DueDate    time.Time
	LastReview time.Time
}

func (db *DB) UpdateCardAfterReview(id int64, update CardUpdate) error {
	_, err := db.Exec(`
		UPDATE cards SET
			stability = ?,
			difficulty = ?,
			reps = ?,
			lapses = ?,
			state = ?,
			due_date = ?,
			last_review = ?
		WHERE id = ?`,
		update.Stability, update.Difficulty, update.Reps, update.Lapses,
		update.State, update.DueDate.Format(time.RFC3339), update.LastReview.Format(time.RFC3339), id,
	)
	return err
}

// Draft/Triage methods

func (db *DB) CountDraftCards() (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM cards WHERE state = 'draft'`).Scan(&count)
	return count, err
}

func (db *DB) GetDraftMoments() ([]*Moment, error) {
	rows, err := db.Query(`
		SELECT DISTINCT m.id, m.timestamp, m.audio_file, m.screenshot_file, m.raw_text,
		       m.segmented_json, m.i1_score, m.status, m.created_at
		FROM moments m
		INNER JOIN cards c ON c.moment_id = m.id
		WHERE c.state = 'draft'
		ORDER BY m.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var moments []*Moment
	for rows.Next() {
		m := &Moment{}
		var segJSON sql.NullString
		var screenshot sql.NullString
		var rawText sql.NullString
		var i1Score sql.NullFloat64
		var createdAt string

		err := rows.Scan(&m.ID, &m.Timestamp, &m.AudioFile, &screenshot, &rawText,
			&segJSON, &i1Score, &m.Status, &createdAt)
		if err != nil {
			return nil, err
		}

		m.ScreenshotFile = screenshot.String
		m.RawText = rawText.String
		m.I1Score = i1Score.Float64
		m.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)

		if segJSON.Valid {
			_ = json.Unmarshal([]byte(segJSON.String), &m.SegmentedWords)
		}

		moments = append(moments, m)
	}
	return moments, nil
}

func (db *DB) GetDraftCardsByMoment(momentID int64) ([]*Card, error) {
	rows, err := db.Query(`
		SELECT id, moment_id, target_word, target_pinyin, target_definition,
		       stability, difficulty, due_date, last_review, reps, lapses, state
		FROM cards
		WHERE moment_id = ? AND state = 'draft'`, momentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []*Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, nil
}

func (db *DB) ApproveCard(cardID int64) error {
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(`
		UPDATE cards
		SET state = 'new', due_date = ?
		WHERE id = ? AND state = 'draft'`, now, cardID)
	return err
}

func (db *DB) ApproveMoment(momentID int64) error {
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(`
		UPDATE cards
		SET state = 'new', due_date = ?
		WHERE moment_id = ? AND state = 'draft'`, now, momentID)
	return err
}

func (db *DB) DeleteCard(cardID int64) error {
	_, err := db.Exec(`DELETE FROM cards WHERE id = ?`, cardID)
	return err
}

func (db *DB) DeleteMoment(momentID int64) error {
	// Delete cards first
	_, err := db.Exec(`DELETE FROM cards WHERE moment_id = ?`, momentID)
	if err != nil {
		return err
	}
	// Then delete moment
	_, err = db.Exec(`DELETE FROM moments WHERE id = ?`, momentID)
	return err
}

// GetAllCards returns all non-draft cards ordered by due date
func (db *DB) GetAllCards() ([]*Card, error) {
	rows, err := db.Query(`
		SELECT id, moment_id, target_word, target_pinyin, target_definition,
		       stability, difficulty, due_date, last_review, reps, lapses, state
		FROM cards
		WHERE state != 'draft'
		ORDER BY due_date ASC NULLS FIRST`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []*Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, nil
}
