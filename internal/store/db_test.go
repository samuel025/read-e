package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"epub-reader/internal/epub"
	"epub-reader/internal/library"
	"epub-reader/internal/store"
)

func TestStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "epub_store_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer st.Close()

	// 1. Settings test
	t.Run("Settings", func(t *testing.T) {
		val := st.GetSetting("theme")
		if val != "" {
			t.Errorf("expected empty string for missing setting, got %q", val)
		}

		if err := st.SaveSetting("theme", "sepia"); err != nil {
			t.Fatalf("failed to save setting: %v", err)
		}

		val = st.GetSetting("theme")
		if val != "sepia" {
			t.Errorf("expected 'sepia', got %q", val)
		}

		if err := st.SaveSetting("theme", "nord"); err != nil {
			t.Fatalf("failed to update setting: %v", err)
		}
		val = st.GetSetting("theme")
		if val != "nord" {
			t.Errorf("expected 'nord', got %q", val)
		}
	})

	// 2. Books & Library test
	t.Run("Books and Progress", func(t *testing.T) {
		book := library.BookMeta{
			ID:          "book-123",
			Title:       "Moby Dick",
			Author:      "Herman Melville",
			FilePath:    "/path/to/moby.epub",
			CoverBase64: "data:image/png;base64,mock",
			AddedAt:     time.Now().UTC().Truncate(time.Second),
		}

		if err := st.UpsertBook(book); err != nil {
			t.Fatalf("failed to upsert book: %v", err)
		}

		books, err := st.GetBooks()
		if err != nil {
			t.Fatalf("failed to get books: %v", err)
		}
		if len(books) != 1 {
			t.Fatalf("expected 1 book, got %d", len(books))
		}
		if books[0].ID != book.ID || books[0].Title != book.Title || books[0].HasProgress {
			t.Errorf("unexpected book meta: %+v", books[0])
		}

		progress := epub.ReadingPosition{
			BookID:       "book-123",
			SpineIndex:   3,
			ScrollOffset: 450.5,
		}
		if err := st.SaveProgress(progress); err != nil {
			t.Fatalf("failed to save progress: %v", err)
		}

		retrievedPos, err := st.GetProgress("book-123")
		if err != nil {
			t.Fatalf("failed to get progress: %v", err)
		}
		if retrievedPos.SpineIndex != 3 || retrievedPos.ScrollOffset != 450.5 {
			t.Errorf("unexpected progress: %+v", retrievedPos)
		}

		books, err = st.GetBooks()
		if err != nil {
			t.Fatalf("failed to get books: %v", err)
		}
		if !books[0].HasProgress {
			t.Errorf("expected book to have progress")
		}

		// Highlights test
		h1 := epub.Highlight{
			ID:         "hl-1",
			BookID:     "book-123",
			SpineIndex: 3,
			Text:       "Call me Ishmael.",
			Prefix:     "Chapter 1. ",
			Suffix:     " Some years ago",
			Color:      "yellow",
			Note:       "Famous opening line",
			CreatedAt:  time.Now(),
		}
		if err := st.SaveHighlight(h1); err != nil {
			t.Fatalf("failed to save highlight: %v", err)
		}

		h2 := epub.Highlight{
			ID:         "hl-2",
			BookID:     "book-123",
			SpineIndex: 3,
			Text:       "whenever my hypos get such an upper hand",
			Color:      "purple",
		}
		if err := st.SaveHighlight(h2); err != nil {
			t.Fatalf("failed to save highlight 2: %v", err)
		}

		highlights, err := st.GetHighlights("book-123")
		if err != nil {
			t.Fatalf("failed to get highlights: %v", err)
		}
		if len(highlights) != 2 {
			t.Fatalf("expected 2 highlights, got %d", len(highlights))
		}
		if highlights[0].Color != "yellow" || highlights[1].Color != "purple" {
			t.Errorf("unexpected highlight colors: %+v", highlights)
		}

		// Update highlight color
		h1.Color = "green"
		if err := st.SaveHighlight(h1); err != nil {
			t.Fatalf("failed to update highlight: %v", err)
		}
		highlights, _ = st.GetHighlights("book-123")
		if highlights[0].Color != "green" {
			t.Errorf("expected updated color 'green', got %q", highlights[0].Color)
		}

		// Update highlight note
		if err := st.UpdateHighlightNote("hl-2", "Interesting reflection"); err != nil {
			t.Fatalf("failed to update highlight note: %v", err)
		}
		highlights, _ = st.GetHighlights("book-123")
		var foundNote string
		for _, h := range highlights {
			if h.ID == "hl-2" {
				foundNote = h.Note
			}
		}
		if foundNote != "Interesting reflection" {
			t.Errorf("expected note 'Interesting reflection', got %q", foundNote)
		}

		// Bookmarks test
		bm := epub.Bookmark{
			ID:           "bm-1",
			BookID:       "book-123",
			SpineIndex:   2,
			Title:        "Important Section",
			ScrollOffset: 320.0,
			CreatedAt:    time.Now(),
		}
		if err := st.SaveBookmark(bm); err != nil {
			t.Fatalf("failed to save bookmark: %v", err)
		}

		bookmarks, err := st.GetBookmarks("book-123")
		if err != nil {
			t.Fatalf("failed to get bookmarks: %v", err)
		}
		if len(bookmarks) != 1 || bookmarks[0].Title != "Important Section" {
			t.Fatalf("unexpected bookmarks: %+v", bookmarks)
		}

		// Delete a highlight
		if err := st.DeleteHighlight("hl-1"); err != nil {
			t.Fatalf("failed to delete highlight: %v", err)
		}
		highlights, _ = st.GetHighlights("book-123")
		if len(highlights) != 1 || highlights[0].ID != "hl-2" {
			t.Fatalf("expected 1 highlight after deletion, got %d", len(highlights))
		}

		if err := st.RemoveBook("book-123"); err != nil {
			t.Fatalf("failed to remove book: %v", err)
		}

		books, err = st.GetBooks()
		if err != nil {
			t.Fatalf("failed to get books: %v", err)
		}
		if len(books) != 0 {
			t.Fatalf("expected 0 books after removal, got %d", len(books))
		}

		// Cascaded delete check
		highlights, _ = st.GetHighlights("book-123")
		if len(highlights) != 0 {
			t.Fatalf("expected 0 highlights after book removal, got %d", len(highlights))
		}
		bookmarks, _ = st.GetBookmarks("book-123")
		if len(bookmarks) != 0 {
			t.Fatalf("expected 0 bookmarks after book removal, got %d", len(bookmarks))
		}
	})
}
