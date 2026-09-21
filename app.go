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
	dbPath := filepath.Join(configDir, "epub-reader", "library.db")

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
	reader, err := a.getReader(bookID)
	if err != nil {
		return epub.BookInfo{}, err
	}
	return reader.Info(), nil
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
		}
	}
	var s epub.AppSettings
	json.Unmarshal([]byte(val), &s)
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
		Title: "Select EPUB File",
		Filters: []runtime.FileFilter{
			{DisplayName: "EPUB Files", Pattern: "*.epub"},
		},
	})
	return file, err
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
