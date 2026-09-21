package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
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
		format TEXT NOT NULL DEFAULT 'epub',
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
	CREATE TABLE IF NOT EXISTS highlights (
		id TEXT PRIMARY KEY,
		book_id TEXT NOT NULL,
		spine_index INTEGER NOT NULL DEFAULT 0,
		text TEXT NOT NULL,
		prefix TEXT NOT NULL DEFAULT '',
		suffix TEXT NOT NULL DEFAULT '',
		color TEXT NOT NULL DEFAULT 'yellow',
		note TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_highlights_book ON highlights(book_id, spine_index);
	CREATE TABLE IF NOT EXISTS bookmarks (
		id TEXT PRIMARY KEY,
		book_id TEXT NOT NULL,
		spine_index INTEGER NOT NULL DEFAULT 0,
		title TEXT NOT NULL DEFAULT '',
		scroll_offset REAL NOT NULL DEFAULT 0.0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_bookmarks_book ON bookmarks(book_id, spine_index);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Safely add format column if migrating existing database
	_, _ = s.db.Exec("ALTER TABLE books ADD COLUMN format TEXT NOT NULL DEFAULT 'epub';")

	return nil
}

func (s *Store) UpsertBook(b library.BookMeta) error {
	format := b.Format
	if format == "" {
		if strings.HasSuffix(strings.ToLower(b.FilePath), ".pdf") {
			format = "pdf"
		} else {
			format = "epub"
		}
	}
	_, err := s.db.Exec(`
		INSERT INTO books (id, title, author, file_path, cover_base64, format, added_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			author = excluded.author,
			file_path = excluded.file_path,
			cover_base64 = excluded.cover_base64,
			format = excluded.format
	`, b.ID, b.Title, b.Author, b.FilePath, b.CoverBase64, format, b.AddedAt)
	return err
}

func (s *Store) UpdateBookCover(id string, coverBase64 string) error {
	_, err := s.db.Exec("UPDATE books SET cover_base64 = ? WHERE id = ?", coverBase64, id)
	return err
}

func (s *Store) GetBooks() ([]library.BookMeta, error) {
	rows, err := s.db.Query(`
		SELECT b.id, b.title, b.author, b.file_path, b.cover_base64, b.format, b.added_at,
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
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.FilePath, &b.CoverBase64, &b.Format, &b.AddedAt, &hasProgress); err != nil {
			return nil, err
		}
		b.HasProgress = hasProgress == 1
		books = append(books, b)
	}
	return books, rows.Err()
}

func (s *Store) GetBook(bookID string) (library.BookMeta, error) {
	var b library.BookMeta
	err := s.db.QueryRow(`
		SELECT id, title, author, file_path, cover_base64, format, added_at
		FROM books
		WHERE id = ?
	`, bookID).Scan(&b.ID, &b.Title, &b.Author, &b.FilePath, &b.CoverBase64, &b.Format, &b.AddedAt)
	return b, err
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

func (s *Store) SaveHighlight(h epub.Highlight) error {
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now()
	}
	_, err := s.db.Exec(`
		INSERT INTO highlights (id, book_id, spine_index, text, prefix, suffix, color, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			color = excluded.color,
			note = excluded.note
	`, h.ID, h.BookID, h.SpineIndex, h.Text, h.Prefix, h.Suffix, h.Color, h.Note, h.CreatedAt)
	return err
}

func (s *Store) GetHighlights(bookID string) ([]epub.Highlight, error) {
	rows, err := s.db.Query(`
		SELECT id, book_id, spine_index, text, prefix, suffix, color, note, created_at
		FROM highlights
		WHERE book_id = ?
		ORDER BY spine_index ASC, created_at ASC
	`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var highlights []epub.Highlight
	for rows.Next() {
		var h epub.Highlight
		if err := rows.Scan(&h.ID, &h.BookID, &h.SpineIndex, &h.Text, &h.Prefix, &h.Suffix, &h.Color, &h.Note, &h.CreatedAt); err != nil {
			return nil, err
		}
		highlights = append(highlights, h)
	}
	if highlights == nil {
		highlights = []epub.Highlight{}
	}
	return highlights, rows.Err()
}

func (s *Store) DeleteHighlight(id string) error {
	_, err := s.db.Exec("DELETE FROM highlights WHERE id = ?", id)
	return err
}

func (s *Store) UpdateHighlightNote(id string, note string) error {
	_, err := s.db.Exec("UPDATE highlights SET note = ? WHERE id = ?", note, id)
	return err
}

func (s *Store) SaveBookmark(b epub.Bookmark) error {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	_, err := s.db.Exec(`
		INSERT INTO bookmarks (id, book_id, spine_index, title, scroll_offset, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			scroll_offset = excluded.scroll_offset
	`, b.ID, b.BookID, b.SpineIndex, b.Title, b.ScrollOffset, b.CreatedAt)
	return err
}

func (s *Store) GetBookmarks(bookID string) ([]epub.Bookmark, error) {
	rows, err := s.db.Query(`
		SELECT id, book_id, spine_index, title, scroll_offset, created_at
		FROM bookmarks
		WHERE book_id = ?
		ORDER BY spine_index ASC, created_at ASC
	`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []epub.Bookmark
	for rows.Next() {
		var b epub.Bookmark
		if err := rows.Scan(&b.ID, &b.BookID, &b.SpineIndex, &b.Title, &b.ScrollOffset, &b.CreatedAt); err != nil {
			return nil, err
		}
		bookmarks = append(bookmarks, b)
	}
	if bookmarks == nil {
		bookmarks = []epub.Bookmark{}
	}
	return bookmarks, rows.Err()
}

func (s *Store) DeleteBookmark(id string) error {
	_, err := s.db.Exec("DELETE FROM bookmarks WHERE id = ?", id)
	return err
}
