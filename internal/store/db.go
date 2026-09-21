package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"epub-reader/internal/epub"
	"epub-reader/internal/library"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS books (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL DEFAULT '',
		author TEXT NOT NULL DEFAULT '',
		file_path TEXT NOT NULL UNIQUE,
		cover_base64 TEXT NOT NULL DEFAULT '',
		added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS progress (
		book_id TEXT PRIMARY KEY,
		spine_index INTEGER NOT NULL DEFAULT 0,
		scroll_offset REAL NOT NULL DEFAULT 0.0,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) UpsertBook(b library.BookMeta) error {
	_, err := s.db.Exec(`
		INSERT INTO books (id, title, author, file_path, cover_base64, added_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			author = excluded.author,
			file_path = excluded.file_path,
			cover_base64 = excluded.cover_base64
	`, b.ID, b.Title, b.Author, b.FilePath, b.CoverBase64, b.AddedAt)
	return err
}

func (s *Store) GetBooks() ([]library.BookMeta, error) {
	rows, err := s.db.Query(`
		SELECT b.id, b.title, b.author, b.file_path, b.cover_base64, b.added_at,
		       CASE WHEN p.book_id IS NOT NULL THEN 1 ELSE 0 END as has_progress
		FROM books b
		LEFT JOIN progress p ON b.id = p.book_id
		ORDER BY b.added_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []library.BookMeta
	for rows.Next() {
		var b library.BookMeta
		var hasProgress int
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.FilePath, &b.CoverBase64, &b.AddedAt, &hasProgress); err != nil {
			return nil, err
		}
		b.HasProgress = hasProgress == 1
		books = append(books, b)
	}
	return books, rows.Err()
}

func (s *Store) RemoveBook(bookID string) error {
	_, err := s.db.Exec("DELETE FROM books WHERE id = ?", bookID)
	return err
}

func (s *Store) SaveProgress(pos epub.ReadingPosition) error {
	_, err := s.db.Exec(`
		INSERT INTO progress (book_id, spine_index, scroll_offset, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(book_id) DO UPDATE SET
			spine_index = excluded.spine_index,
			scroll_offset = excluded.scroll_offset,
			updated_at = excluded.updated_at
	`, pos.BookID, pos.SpineIndex, pos.ScrollOffset, time.Now())
	return err
}

func (s *Store) GetProgress(bookID string) (epub.ReadingPosition, error) {
	var pos epub.ReadingPosition
	pos.BookID = bookID

	err := s.db.QueryRow(`
		SELECT spine_index, scroll_offset FROM progress WHERE book_id = ?
	`, bookID).Scan(&pos.SpineIndex, &pos.ScrollOffset)

	if err == sql.ErrNoRows {
		return pos, nil
	}
	return pos, err
}

func (s *Store) SaveSetting(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

func (s *Store) GetSetting(key string) string {
	var val string
	s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	return val
}
