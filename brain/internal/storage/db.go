// brain/internal/storage/db.go
package storage

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func OpenDatabase(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func migrate(db *sql.DB) error {
	schema := `
	-- Captured moments with enrichment
	CREATE TABLE IF NOT EXISTS moments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		audio_file TEXT NOT NULL,
		screenshot_file TEXT,
		raw_text TEXT,
		segmented_json TEXT,
		i1_score REAL,
		status TEXT DEFAULT 'pending',
		created_at TEXT DEFAULT CURRENT_TIMESTAMP
	);

	-- Review cards (one per target word per moment)
	CREATE TABLE IF NOT EXISTS cards (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		moment_id INTEGER NOT NULL REFERENCES moments(id),
		target_word TEXT NOT NULL,
		target_pinyin TEXT,
		target_definition TEXT,
		stability REAL DEFAULT 0,
		difficulty REAL DEFAULT 0,
		due_date TEXT,
		last_review TEXT,
		reps INTEGER DEFAULT 0,
		lapses INTEGER DEFAULT 0,
		state TEXT DEFAULT 'new',
		created_at TEXT DEFAULT CURRENT_TIMESTAMP
	);

	-- Vocabulary knowledge
	CREATE TABLE IF NOT EXISTS vocabulary (
		word TEXT PRIMARY KEY,
		pinyin TEXT,
		definition TEXT,
		status TEXT DEFAULT 'unknown',
		times_seen INTEGER DEFAULT 0,
		times_correct INTEGER DEFAULT 0,
		updated_at TEXT DEFAULT CURRENT_TIMESTAMP
	);

	-- Review history
	CREATE TABLE IF NOT EXISTS reviews (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		card_id INTEGER NOT NULL REFERENCES cards(id),
		rating INTEGER NOT NULL,
		time_taken_ms INTEGER,
		reviewed_at TEXT DEFAULT CURRENT_TIMESTAMP
	);

	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_cards_due ON cards(due_date);
	CREATE INDEX IF NOT EXISTS idx_cards_moment ON cards(moment_id);
	CREATE INDEX IF NOT EXISTS idx_vocab_status ON vocabulary(status);
	`

	_, err := db.Exec(schema)
	return err
}
