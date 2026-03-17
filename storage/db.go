package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

type FileRecord struct {
	FilePath     string
	Status       string
	QualityScore float64
	WordCount    int
	CleanedAt    *time.Time
	ExportFormat string
	AIUsed       string
	SkipReason   string
}

func Init() (*DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not find home directory: %w", err)
	}

	dir := filepath.Join(home, ".phantomclean")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("could not create .phantomclean directory: %w", err)
	}

	dbPath := filepath.Join(dir, "state.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}

	// WAL mode for better concurrent write performance
	conn.Exec("PRAGMA journal_mode=WAL")
	conn.Exec("PRAGMA synchronous=NORMAL")
	conn.Exec("PRAGMA busy_timeout=5000")

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			file_path     TEXT UNIQUE,
			status        TEXT,
			quality_score REAL DEFAULT 0,
			word_count    INTEGER DEFAULT 0,
			cleaned_at    TIMESTAMP,
			export_format TEXT,
			ai_used       TEXT,
			skip_reason   TEXT
		);
	`)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	return nil
}

// IsProcessed checks if file is already done or skipped
func (db *DB) IsProcessed(filePath string) bool {
	var count int
	db.conn.QueryRow(
		"SELECT COUNT(*) FROM files WHERE file_path = ? AND status IN ('done', 'skipped', 'omitted')",
		filePath,
	).Scan(&count)
	return count > 0
}

// IsDone checks if file is fully organized
func (db *DB) IsDone(filePath string) bool {
	var count int
	db.conn.QueryRow(
		"SELECT COUNT(*) FROM files WHERE file_path = ? AND status = 'done'",
		filePath,
	).Scan(&count)
	return count > 0
}

// MarkPending marks file as being processed
func (db *DB) MarkPending(filePath string) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO files (file_path, status)
		VALUES (?, 'pending')`,
		filePath,
	)
	return err
}

// MarkDone marks file as successfully organized
func (db *DB) MarkDone(filePath string, qualityScore float64, wordCount int, exportFormat, aiUsed string) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO files
		(file_path, status, quality_score, word_count, cleaned_at, export_format, ai_used)
		VALUES (?, 'done', ?, ?, ?, ?, ?)`,
		filePath, qualityScore, wordCount, time.Now(), exportFormat, aiUsed,
	)
	return err
}

// MarkSkipped marks file as skipped with reason
func (db *DB) MarkSkipped(filePath, reason string) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO files (file_path, status, skip_reason)
		VALUES (?, 'skipped', ?)`,
		filePath, reason,
	)
	return err
}

// MarkFailed marks file as failed
func (db *DB) MarkFailed(filePath, reason string) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO files (file_path, status, skip_reason)
		VALUES (?, 'failed', ?)`,
		filePath, reason,
	)
	return err
}

// MarkOmitted marks file as omitted by config
func (db *DB) MarkOmitted(filePath string) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO files (file_path, status)
		VALUES (?, 'omitted')`,
		filePath,
	)
	return err
}

// GetRulesOnly returns files cleaned by rules fallback only (no AI)
// Used by clean-ai command to retry AI on these files
func (db *DB) GetRulesOnly() ([]string, error) {
	rows, err := db.conn.Query(
		"SELECT file_path FROM files WHERE status = 'done' AND ai_used = 'rules'",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		rows.Scan(&p)
		paths = append(paths, p)
	}
	return paths, nil
}

// GetPending returns files marked pending (interrupted run)
// Used by resume command
func (db *DB) GetPending() ([]string, error) {
	rows, err := db.conn.Query(
		"SELECT file_path FROM files WHERE status IN ('pending', 'failed')",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		rows.Scan(&p)
		paths = append(paths, p)
	}
	return paths, nil
}

// GetAllRecords returns all file records for stats display
func (db *DB) GetAllRecords() ([]FileRecord, error) {
	rows, err := db.conn.Query(`
		SELECT file_path, status, quality_score, word_count,
		       COALESCE(export_format, ''), COALESCE(ai_used, ''),
		       COALESCE(skip_reason, ''), cleaned_at
		FROM files
		ORDER BY cleaned_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []FileRecord
	for rows.Next() {
		var r FileRecord
		var cleanedAt sql.NullTime
		err := rows.Scan(
			&r.FilePath, &r.Status, &r.QualityScore, &r.WordCount,
			&r.ExportFormat, &r.AIUsed, &r.SkipReason, &cleanedAt,
		)
		if err != nil {
			continue
		}
		if cleanedAt.Valid {
			r.CleanedAt = &cleanedAt.Time
		}
		records = append(records, r)
	}
	return records, nil
}

// GetStats returns summary counts
func (db *DB) GetStats() (done, skipped, failed, pending, rulesOnly int, err error) {
	db.conn.QueryRow("SELECT COUNT(*) FROM files WHERE status = 'done'").Scan(&done)
	db.conn.QueryRow("SELECT COUNT(*) FROM files WHERE status = 'skipped'").Scan(&skipped)
	db.conn.QueryRow("SELECT COUNT(*) FROM files WHERE status = 'failed'").Scan(&failed)
	db.conn.QueryRow("SELECT COUNT(*) FROM files WHERE status = 'pending'").Scan(&pending)
	db.conn.QueryRow("SELECT COUNT(*) FROM files WHERE status = 'done' AND ai_used = 'rules'").Scan(&rulesOnly)
	return
}

// Reset wipes all state
func (db *DB) Reset() error {
	_, err := db.conn.Exec("DELETE FROM files")
	return err
}

// ResetPending clears only pending and failed entries so that start
// re-processes them through the batch system. Done and skipped entries
// are preserved to avoid re-cleaning already completed files.
func (db *DB) ResetPending() error {
	_, err := db.conn.Exec("DELETE FROM files WHERE status IN ('pending', 'failed')")
	return err
}

func (db *DB) Close() {
	db.conn.Close()
}
