package store

import (
	"database/sql"
	"fmt"
	"math"
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
	CREATE TABLE IF NOT EXISTS dictionary_cache (
		word TEXT PRIMARY KEY,
		data_json TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Safely add format column if migrating existing database
	_, _ = s.db.Exec("ALTER TABLE books ADD COLUMN format TEXT NOT NULL DEFAULT 'epub';")

	// Safely add total_count column if migrating existing database
	_, _ = s.db.Exec("ALTER TABLE books ADD COLUMN total_count INTEGER NOT NULL DEFAULT 0;")

	// Safely add finished column to progress if migrating existing database
	_, _ = s.db.Exec("ALTER TABLE progress ADD COLUMN finished BOOLEAN NOT NULL DEFAULT 0;")

	// Create reading_sessions table
	_, _ = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS reading_sessions (
			date TEXT PRIMARY KEY,
			duration_seconds INTEGER NOT NULL DEFAULT 0,
			pages_turned INTEGER NOT NULL DEFAULT 0
		);
	`)

	// Backfill any existing books with total_count <= 0
	rows, err := s.db.Query("SELECT id, file_path, format FROM books WHERE total_count <= 0")
	if err == nil {
		type fixItem struct {
			id    string
			count int
		}
		var fixes []fixItem
		for rows.Next() {
			var id, path, format string
			if err := rows.Scan(&id, &path, &format); err == nil {
				if _, statErr := os.Stat(path); statErr == nil {
					var count int
					if format == "pdf" {
						count = library.CountPDFPages(path)
					} else {
						if r, err := epub.Open(path); err == nil {
							count = r.Info().SpineCount
							r.Close()
						}
					}
					if count > 0 {
						fixes = append(fixes, fixItem{id: id, count: count})
					}
				}
			}
		}
		rows.Close()

		for _, fix := range fixes {
			_, _ = s.db.Exec("UPDATE books SET total_count = ? WHERE id = ?", fix.count, fix.id)
		}
	}

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
		INSERT INTO books (id, title, author, file_path, cover_base64, format, total_count, added_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			author = excluded.author,
			file_path = excluded.file_path,
			cover_base64 = excluded.cover_base64,
			format = excluded.format,
			total_count = CASE WHEN excluded.total_count > 0 THEN excluded.total_count ELSE books.total_count END
	`, b.ID, b.Title, b.Author, b.FilePath, b.CoverBase64, format, b.TotalCount, b.AddedAt)
	return err
}

func (s *Store) UpdateBookCover(id string, coverBase64 string) error {
	_, err := s.db.Exec("UPDATE books SET cover_base64 = ? WHERE id = ?", coverBase64, id)
	return err
}

func (s *Store) UpdateBookTotalCount(id string, totalCount int) error {
	_, err := s.db.Exec("UPDATE books SET total_count = ? WHERE id = ?", totalCount, id)
	return err
}

func (s *Store) GetBooks() ([]library.BookMeta, error) {
	rows, err := s.db.Query(`
		SELECT b.id, b.title, b.author, b.file_path, b.cover_base64, b.format,
		       COALESCE(b.total_count, 0), b.added_at,
		       COALESCE(p.spine_index, 0) as spine_index,
		       COALESCE(p.finished, 0) as finished,
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
		var hasProgress, finished, spineIndex, totalCount int
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.FilePath, &b.CoverBase64, &b.Format, &totalCount, &b.AddedAt, &spineIndex, &finished, &hasProgress); err != nil {
			return nil, err
		}
		b.TotalCount = totalCount
		b.HasProgress = hasProgress == 1
		b.Finished = finished == 1
		b.SpineIndex = spineIndex

		// On-the-fly resolution if total_count is still 0
		if b.TotalCount <= 0 {
			if b.Format == "pdf" {
				b.TotalCount = library.CountPDFPages(b.FilePath)
			} else {
				if r, err := epub.Open(b.FilePath); err == nil {
					b.TotalCount = r.Info().SpineCount
					r.Close()
				}
			}
			if b.TotalCount > 0 {
				_, _ = s.db.Exec("UPDATE books SET total_count = ? WHERE id = ?", b.TotalCount, b.ID)
			}
		}

		if b.Finished {
			b.Progress = 100.0
		} else if b.TotalCount > 0 && b.HasProgress {
			if b.TotalCount > 1 && b.SpineIndex >= b.TotalCount-1 {
				b.Progress = 100.0
				b.Finished = true
				_, _ = s.db.Exec("UPDATE progress SET finished = 1 WHERE book_id = ?", b.ID)
			} else {
				pct := float64(spineIndex+1) / float64(b.TotalCount) * 100.0
				if pct > 100.0 {
					pct = 100.0
				}
				b.Progress = math.Round(pct*10) / 10
				if b.Progress >= 100.0 {
					b.Finished = true
					_, _ = s.db.Exec("UPDATE progress SET finished = 1 WHERE book_id = ?", b.ID)
				}
			}
		} else {
			b.Progress = 0.0
		}
		books = append(books, b)
	}
	return books, rows.Err()
}

func (s *Store) GetBook(bookID string) (library.BookMeta, error) {
	var b library.BookMeta
	var totalCount int
	err := s.db.QueryRow(`
		SELECT id, title, author, file_path, cover_base64, format, COALESCE(total_count, 0), added_at
		FROM books
		WHERE id = ?
	`, bookID).Scan(&b.ID, &b.Title, &b.Author, &b.FilePath, &b.CoverBase64, &b.Format, &totalCount, &b.AddedAt)
	b.TotalCount = totalCount
	return b, err
}

func (s *Store) RemoveBook(bookID string) error {
	_, err := s.db.Exec("DELETE FROM books WHERE id = ?", bookID)
	return err
}

func (s *Store) SaveProgress(pos epub.ReadingPosition) error {
	var totalCount int
	_ = s.db.QueryRow("SELECT COALESCE(total_count, 0) FROM books WHERE id = ?", pos.BookID).Scan(&totalCount)

	isFinished := 0
	if totalCount > 1 && pos.SpineIndex >= totalCount-1 {
		isFinished = 1
	}

	_, err := s.db.Exec(`
		INSERT INTO progress (book_id, spine_index, scroll_offset, finished, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(book_id) DO UPDATE SET
			spine_index = excluded.spine_index,
			scroll_offset = excluded.scroll_offset,
			finished = CASE WHEN ? = 1 THEN 1 ELSE progress.finished END,
			updated_at = excluded.updated_at
	`, pos.BookID, pos.SpineIndex, pos.ScrollOffset, isFinished, time.Now(), isFinished)
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

// Insights Models and Methods
type DailySession struct {
	Date            string `json:"date"`
	DurationSeconds int    `json:"duration_seconds"`
	PagesTurned     int    `json:"pages_turned"`
}

type ReadingInsights struct {
	TotalHours      float64        `json:"total_hours"`
	FinishedBooks   int            `json:"finished_books"`
	CurrentStreak   int            `json:"current_streak"`
	LongestStreak   int            `json:"longest_streak"`
	DailyData       []DailySession `json:"daily_data"`
}

func (s *Store) LogReadingSession(date string, durationSecs int, pagesTurned int) error {
	_, err := s.db.Exec(`
		INSERT INTO reading_sessions (date, duration_seconds, pages_turned)
		VALUES (?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET
			duration_seconds = duration_seconds + excluded.duration_seconds,
			pages_turned = pages_turned + excluded.pages_turned
	`, date, durationSecs, pagesTurned)
	return err
}

func (s *Store) MarkBookFinished(bookID string, finished bool) error {
	if finished {
		_, err := s.db.Exec(`
			INSERT INTO progress (book_id, spine_index, scroll_offset, finished, updated_at)
			VALUES (?, 0, 0, 1, CURRENT_TIMESTAMP)
			ON CONFLICT(book_id) DO UPDATE SET
				finished = 1,
				updated_at = CURRENT_TIMESTAMP
		`, bookID)
		return err
	}
	_, err := s.db.Exec(`
		UPDATE progress SET finished = 0, spine_index = 0, scroll_offset = 0, updated_at = CURRENT_TIMESTAMP WHERE book_id = ?
	`, bookID)
	return err
}

func (s *Store) GetReadingInsights() (ReadingInsights, error) {
	var insights ReadingInsights

	rows, err := s.db.Query(`SELECT date, duration_seconds, pages_turned FROM reading_sessions ORDER BY date ASC`)
	if err != nil {
		return insights, err
	}
	defer rows.Close()

	var totalSeconds int
	for rows.Next() {
		var d DailySession
		if err := rows.Scan(&d.Date, &d.DurationSeconds, &d.PagesTurned); err == nil {
			insights.DailyData = append(insights.DailyData, d)
			totalSeconds += d.DurationSeconds
		}
	}
	insights.TotalHours = float64(totalSeconds) / 3600.0

	currentStreak := 0
	longestStreak := 0
	streakCounter := 0
	var lastDate time.Time

	for i, session := range insights.DailyData {
		date, err := time.Parse("2006-01-02", session.Date)
		if err != nil {
			continue
		}
		if i == 0 || date.Sub(lastDate).Hours() <= 24.0 {
			streakCounter++
		} else {
			streakCounter = 1
		}
		if streakCounter > longestStreak {
			longestStreak = streakCounter
		}
		lastDate = date
	}
	
	if len(insights.DailyData) > 0 {
		lastSessionDate, _ := time.Parse("2006-01-02", insights.DailyData[len(insights.DailyData)-1].Date)
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		diffDays := today.Sub(lastSessionDate).Hours() / 24.0
		if diffDays <= 1.0 {
			currentStreak = streakCounter
		} else {
			currentStreak = 0
		}
	}
	
	insights.CurrentStreak = currentStreak
	insights.LongestStreak = longestStreak

	row := s.db.QueryRow(`SELECT COUNT(*) FROM progress WHERE finished = 1`)
	_ = row.Scan(&insights.FinishedBooks)

	return insights, nil
}

// GetCachedDefinition returns the JSON-encoded definition for a word, or error if not cached
func (s *Store) GetCachedDefinition(word string) (string, error) {
	var dataJSON string
	err := s.db.QueryRow(`SELECT data_json FROM dictionary_cache WHERE word = ?`, word).Scan(&dataJSON)
	if err != nil {
		return "", err
	}
	return dataJSON, nil
}

// SaveCachedDefinition caches a word's definition JSON into sqlite
func (s *Store) SaveCachedDefinition(word string, dataJSON string) error {
	_, err := s.db.Exec(`
		INSERT INTO dictionary_cache (word, data_json, created_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(word) DO UPDATE SET data_json = excluded.data_json, created_at = CURRENT_TIMESTAMP
	`, word, dataJSON)
	return err
}

