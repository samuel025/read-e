package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"epub-reader/internal/epub"
	"epub-reader/internal/library"
	"epub-reader/internal/store"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx   context.Context
	store *store.Store

	mu        sync.Mutex
	openBooks map[string]*epub.Reader
	bookPaths map[string]string
}

func NewApp() *App {
	return &App{
		openBooks: make(map[string]*epub.Reader),
		bookPaths: make(map[string]string),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	dbPath := filepath.Join(configDir, "read-e", "library.db")

	s, err := store.New(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open store: %v\n", err)
		return
	}
	a.store = s

	books, _ := s.GetBooks()
	for _, b := range books {
		a.bookPaths[b.ID] = b.FilePath
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	for _, r := range a.openBooks {
		r.Close()
	}
	a.mu.Unlock()

	if a.store != nil {
		a.store.Close()
	}
}

func (a *App) ScanLibrary(folderPath string) ([]library.BookMeta, error) {
	books, err := library.ScanFolder(folderPath)
	if err != nil {
		return nil, err
	}

	for _, b := range books {
		if err := a.store.UpsertBook(b); err != nil {
			fmt.Fprintf(os.Stderr, "failed to save book %s: %v\n", b.Title, err)
		}
		a.bookPaths[b.ID] = b.FilePath
	}

	return a.store.GetBooks()
}

func (a *App) AddBook(filePath string) ([]library.BookMeta, error) {
	meta, err := library.ScanFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	if err := a.store.UpsertBook(meta); err != nil {
		return nil, err
	}
	a.bookPaths[meta.ID] = meta.FilePath

	return a.store.GetBooks()
}

func (a *App) GetLibrary() ([]library.BookMeta, error) {
	return a.store.GetBooks()
}

func (a *App) UpdateBookCover(bookID string, coverBase64 string) error {
	return a.store.UpdateBookCover(bookID, coverBase64)
}

func (a *App) LogReadingSession(date string, durationSecs int, pagesTurned int) error {
	return a.store.LogReadingSession(date, durationSecs, pagesTurned)
}

func (a *App) MarkBookFinished(bookID string, finished bool) error {
	return a.store.MarkBookFinished(bookID, finished)
}

func (a *App) GetReadingInsights() (store.ReadingInsights, error) {
	return a.store.GetReadingInsights()
}

func (a *App) RemoveBook(bookID string) error {
	a.mu.Lock()
	if r, ok := a.openBooks[bookID]; ok {
		r.Close()
	}
	delete(a.openBooks, bookID)
	a.mu.Unlock()
	delete(a.bookPaths, bookID)
	return a.store.RemoveBook(bookID)
}

func (a *App) OpenBook(bookID string) (epub.BookInfo, error) {
	meta, err := a.store.GetBook(bookID)
	if err == nil && meta.Format == "pdf" {
		return epub.BookInfo{
			Title:  meta.Title,
			Author: meta.Author,
			Format: "pdf",
		}, nil
	}

	reader, err := a.getReader(bookID)
	if err != nil {
		return epub.BookInfo{}, err
	}
	info := reader.Info()
	info.Format = "epub"
	return info, nil
}

func (a *App) GetTOC(bookID string) ([]epub.TOCEntry, error) {
	reader, err := a.getReader(bookID)
	if err != nil {
		return nil, err
	}
	toc := reader.TOC()
	if toc == nil {
		toc = []epub.TOCEntry{}
	}
	return toc, nil
}

func (a *App) GetChapter(bookID string, spineIndex int) (string, error) {
	reader, err := a.getReader(bookID)
	if err != nil {
		return "", err
	}
	return reader.ChapterHTML(spineIndex)
}

func (a *App) SaveProgress(bookID string, spineIndex int, scrollOffset float64) error {
	return a.store.SaveProgress(epub.ReadingPosition{
		BookID:       bookID,
		SpineIndex:   spineIndex,
		ScrollOffset: scrollOffset,
	})
}

func (a *App) GetProgress(bookID string) (epub.ReadingPosition, error) {
	return a.store.GetProgress(bookID)
}

func (a *App) AddHighlight(h epub.Highlight) error {
	return a.store.SaveHighlight(h)
}

func (a *App) GetHighlights(bookID string) ([]epub.Highlight, error) {
	return a.store.GetHighlights(bookID)
}

func (a *App) DeleteHighlight(id string) error {
	return a.store.DeleteHighlight(id)
}

func (a *App) UpdateHighlightNote(id string, note string) error {
	return a.store.UpdateHighlightNote(id, note)
}

func (a *App) SaveBookmark(b epub.Bookmark) error {
	return a.store.SaveBookmark(b)
}

func (a *App) GetBookmarks(bookID string) ([]epub.Bookmark, error) {
	return a.store.GetBookmarks(bookID)
}

func (a *App) DeleteBookmark(id string) error {
	return a.store.DeleteBookmark(id)
}

func (a *App) SearchBook(bookID string, query string) ([]epub.SearchResult, error) {
	reader, err := a.getReader(bookID)
	if err != nil {
		return nil, err
	}
	return reader.Search(query), nil
}

func (a *App) GetChapterWordCount(bookID string, spineIndex int) (int, error) {
	reader, err := a.getReader(bookID)
	if err != nil {
		return 0, err
	}
	return reader.ChapterWordCount(spineIndex), nil
}

func (a *App) SaveSettings(settings epub.AppSettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return a.store.SaveSetting("app_settings", string(data))
}

func (a *App) GetSettings() epub.AppSettings {
	val := a.store.GetSetting("app_settings")
	if val == "" {
		return epub.AppSettings{
			Theme:      "dark",
			FontSize:   1.0,
			FontFamily: "'Inter', system-ui, sans-serif",
			MaxWidth:   780,
			LineHeight: 1.7,
			TextAlign:  "left",
		}
	}
	var s epub.AppSettings
	json.Unmarshal([]byte(val), &s)
	if s.MaxWidth == 0 {
		s.MaxWidth = 780
	}
	if s.LineHeight == 0 {
		s.LineHeight = 1.7
	}
	if s.TextAlign == "" {
		s.TextAlign = "left"
	}
	return s
}

func (a *App) SelectFolder() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select EPUB Library Folder",
	})
	return dir, err
}

func (a *App) SelectFile() (string, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Book File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Books and Documents (*.epub, *.pdf)", Pattern: "*.epub;*.pdf"},
			{DisplayName: "EPUB Files (*.epub)", Pattern: "*.epub"},
			{DisplayName: "PDF Documents (*.pdf)", Pattern: "*.pdf"},
		},
	})
	return file, err
}

func (a *App) GetPDFData(bookID string) ([]byte, error) {
	meta, err := a.store.GetBook(bookID)
	filePath := ""
	if err == nil && meta.FilePath != "" {
		filePath = meta.FilePath
	} else {
		a.mu.Lock()
		p, ok := a.bookPaths[bookID]
		a.mu.Unlock()
		if ok {
			filePath = p
		}
	}
	if filePath == "" {
		return nil, fmt.Errorf("book %q not found", bookID)
	}
	return os.ReadFile(filePath)
}

func (a *App) getReader(bookID string) (*epub.Reader, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if r, ok := a.openBooks[bookID]; ok {
		return r, nil
	}

	filePath, ok := a.bookPaths[bookID]
	if !ok {
		return nil, fmt.Errorf("book %q not found", bookID)
	}

	r, err := epub.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open book: %w", err)
	}

	a.openBooks[bookID] = r
	return r, nil
}
