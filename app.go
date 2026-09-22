package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

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

func (a *App) UpdateBookTotalCount(bookID string, totalCount int) error {
	return a.store.UpdateBookTotalCount(bookID, totalCount)
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
		totalCount := meta.TotalCount
		if totalCount <= 0 {
			totalCount = library.CountPDFPages(meta.FilePath)
			if totalCount > 0 {
				_ = a.store.UpdateBookTotalCount(bookID, totalCount)
			}
		}
		return epub.BookInfo{
			Title:      meta.Title,
			Author:     meta.Author,
			Format:     "pdf",
			SpineCount: totalCount,
		}, nil
	}

	reader, err := a.getReader(bookID)
	if err != nil {
		return epub.BookInfo{}, err
	}
	info := reader.Info()
	info.Format = "epub"
	if err == nil && meta.TotalCount <= 0 && info.SpineCount > 0 {
		_ = a.store.UpdateBookTotalCount(bookID, info.SpineCount)
	}
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

func (a *App) GetReadingStats(bookID string, spineIndex int) (epub.BookReadingStats, error) {
	reader, err := a.getReader(bookID)
	if err != nil {
		return epub.BookReadingStats{}, err
	}
	totalWords, remWords, chapWords := reader.ReadingStats(spineIndex)
	bookMins := int(math.Round(float64(remWords) / 220.0))
	chapMins := int(math.Round(float64(chapWords) / 220.0))
	if chapMins == 0 && chapWords > 0 {
		chapMins = 1
	}
	return epub.BookReadingStats{
		TotalWords:      totalWords,
		RemainingWords:  remWords,
		ChapterWords:    chapWords,
		BookMinutesLeft: bookMins,
		ChapterMinutes:  chapMins,
	}, nil
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

// ServeHTTP enables streaming PDFs directly over HTTP range requests via Wails AssetServer
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/pdf/") {
		rawID := strings.TrimPrefix(r.URL.Path, "/pdf/")
		bookID, unescapeErr := url.PathUnescape(rawID)
		if unescapeErr != nil {
			bookID = rawID
		}
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

		if filePath != "" {
			if _, statErr := os.Stat(filePath); statErr == nil {
				w.Header().Set("Content-Type", "application/pdf")
				w.Header().Set("Accept-Ranges", "bytes")
				http.ServeFile(w, r, filePath)
				return
			}
		}
	}
	http.NotFound(w, r)
}

// ReleaseMemory closes open readers and triggers aggressive GC and OS heap trimming
func (a *App) ReleaseMemory() {
	a.mu.Lock()
	for id, r := range a.openBooks {
		r.Close()
		delete(a.openBooks, id)
	}
	a.mu.Unlock()

	debug.FreeOSMemory()
}

type DefinitionItem struct {
	Definition string   `json:"definition"`
	Example    string   `json:"example,omitempty"`
	Synonyms   []string `json:"synonyms,omitempty"`
}

type MeaningItem struct {
	PartOfSpeech string           `json:"partOfSpeech"`
	Definitions  []DefinitionItem `json:"definitions"`
	Synonyms     []string         `json:"synonyms,omitempty"`
}

type PhoneticItem struct {
	Text  string `json:"text,omitempty"`
	Audio string `json:"audio,omitempty"`
}

type DictionaryEntry struct {
	Word      string         `json:"word"`
	Phonetic  string         `json:"phonetic,omitempty"`
	Phonetics []PhoneticItem `json:"phonetics,omitempty"`
	Meanings  []MeaningItem  `json:"meanings"`
	Cached    bool           `json:"cached"`
}

var (
	dictHTMLTagRegex    = regexp.MustCompile(`<[^>]+>`)
	dictHTMLStyleRegex  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	dictHTMLScriptRegex = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
)

func cleanDictionaryHTML(text string) string {
	if text == "" {
		return ""
	}
	text = dictHTMLStyleRegex.ReplaceAllString(text, "")
	text = dictHTMLScriptRegex.ReplaceAllString(text, "")
	text = dictHTMLTagRegex.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

// fetchDatamuseInfo queries Datamuse for IPA phonetics and synonyms with a short timeout
func fetchDatamuseInfo(word string) (string, []string) {
	ipaURL := fmt.Sprintf("https://api.datamuse.com/words?sp=%s&qe=sp&md=r&ipa=1", url.QueryEscape(word))
	synURL := fmt.Sprintf("https://api.datamuse.com/words?rel_syn=%s&max=6", url.QueryEscape(word))

	client := &http.Client{Timeout: 2 * time.Second}
	var ipa string
	var synonyms []string

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		resp, err := client.Get(ipaURL)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return
		}
		var items []struct {
			Word string   `json:"word"`
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&items); err == nil && len(items) > 0 {
			for _, item := range items {
				if strings.EqualFold(item.Word, word) {
					for _, tag := range item.Tags {
						if strings.HasPrefix(tag, "ipa_pron:") {
							val := strings.TrimPrefix(tag, "ipa_pron:")
							if val != "" {
								ipa = "/" + val + "/"
								return
							}
						}
					}
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		resp, err := client.Get(synURL)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return
		}
		var items []struct {
			Word string `json:"word"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&items); err == nil {
			for _, item := range items {
				if item.Word != "" && !strings.EqualFold(item.Word, word) {
					synonyms = append(synonyms, item.Word)
				}
			}
		}
	}()

	wg.Wait()
	return ipa, synonyms
}

// lookupWiktionary queries the fast, reliable Wiktionary REST API
func lookupWiktionary(word string) (*DictionaryEntry, error) {
	apiWord := strings.ReplaceAll(word, " ", "_")
	apiURL := fmt.Sprintf("https://en.wiktionary.org/api/rest_v1/page/definition/%s", url.PathEscape(apiWord))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Read-e/1.0 (https://github.com/samuel025/read-e; desktop epub reader)")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no definition found for '%s'", word)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wiktionary returned status %d", resp.StatusCode)
	}

	var data map[string][]struct {
		PartOfSpeech string `json:"partOfSpeech"`
		Definitions  []struct {
			Definition string   `json:"definition"`
			Examples   []string `json:"examples"`
		} `json:"definitions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	enItems, ok := data["en"]
	if !ok || len(enItems) == 0 {
		return nil, fmt.Errorf("no english definitions for '%s'", word)
	}

	entry := &DictionaryEntry{
		Word:   word,
		Cached: false,
	}

	for _, item := range enItems {
		meaning := MeaningItem{
			PartOfSpeech: item.PartOfSpeech,
		}
		for _, d := range item.Definitions {
			defText := cleanDictionaryHTML(d.Definition)
			if defText == "" {
				continue
			}
			var exampleText string
			for _, ex := range d.Examples {
				cleanedEx := cleanDictionaryHTML(ex)
				if cleanedEx != "" {
					exampleText = cleanedEx
					break
				}
			}
			meaning.Definitions = append(meaning.Definitions, DefinitionItem{
				Definition: defText,
				Example:    exampleText,
			})
		}
		if len(meaning.Definitions) > 0 {
			entry.Meanings = append(entry.Meanings, meaning)
		}
	}

	if len(entry.Meanings) == 0 {
		return nil, fmt.Errorf("no valid definitions found for '%s'", word)
	}

	// Fetch IPA and related synonyms asynchronously
	ipa, syns := fetchDatamuseInfo(word)
	if ipa != "" {
		entry.Phonetic = ipa
		entry.Phonetics = append(entry.Phonetics, PhoneticItem{Text: ipa})
	}
	if len(syns) > 0 && len(entry.Meanings) > 0 {
		entry.Meanings[0].Synonyms = syns
	}

	return entry, nil
}

// lookupFreeDictionaryAPI queries api.dictionaryapi.dev as a secondary fallback
func lookupFreeDictionaryAPI(word string) (*DictionaryEntry, error) {
	apiURL := fmt.Sprintf("https://api.dictionaryapi.dev/api/v2/entries/en/%s", url.PathEscape(word))
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rawEntries []struct {
		Word      string `json:"word"`
		Phonetic  string `json:"phonetic"`
		Phonetics []struct {
			Text  string `json:"text"`
			Audio string `json:"audio"`
		} `json:"phonetics"`
		Meanings []struct {
			PartOfSpeech string `json:"partOfSpeech"`
			Definitions  []struct {
				Definition string   `json:"definition"`
				Example    string   `json:"example"`
				Synonyms   []string `json:"synonyms"`
			} `json:"definitions"`
			Synonyms []string `json:"synonyms"`
		} `json:"meanings"`
	}

	if err := json.Unmarshal(bodyBytes, &rawEntries); err != nil || len(rawEntries) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	first := rawEntries[0]
	entry := &DictionaryEntry{
		Word:     first.Word,
		Phonetic: first.Phonetic,
		Cached:   false,
	}

	for _, p := range first.Phonetics {
		if p.Audio != "" || p.Text != "" {
			entry.Phonetics = append(entry.Phonetics, PhoneticItem{
				Text:  p.Text,
				Audio: p.Audio,
			})
		}
	}
	if entry.Phonetic == "" && len(entry.Phonetics) > 0 {
		for _, p := range entry.Phonetics {
			if p.Text != "" {
				entry.Phonetic = p.Text
				break
			}
		}
	}

	for _, m := range first.Meanings {
		meaning := MeaningItem{
			PartOfSpeech: m.PartOfSpeech,
			Synonyms:     m.Synonyms,
		}
		for _, d := range m.Definitions {
			meaning.Definitions = append(meaning.Definitions, DefinitionItem{
				Definition: d.Definition,
				Example:    d.Example,
				Synonyms:   d.Synonyms,
			})
		}
		entry.Meanings = append(entry.Meanings, meaning)
	}

	return entry, nil
}

// LookupWord searches local SQLite cache first, then Wiktionary, then Free Dictionary API fallback
func (a *App) LookupWord(rawWord string) (*DictionaryEntry, error) {
	cleanWord := strings.TrimSpace(rawWord)
	cleanWord = strings.Trim(cleanWord, "\"'“”‘’.,;:!?()[]{}<>-—_`*~/\\")
	cleanWord = strings.ToLower(cleanWord)
	if cleanWord == "" || len(cleanWord) > 60 {
		return nil, fmt.Errorf("invalid word")
	}

	// 1. Check local SQLite cache (instant & offline)
	cachedJSON, err := a.store.GetCachedDefinition(cleanWord)
	if err == nil && cachedJSON != "" {
		var entry DictionaryEntry
		if jsonErr := json.Unmarshal([]byte(cachedJSON), &entry); jsonErr == nil {
			entry.Cached = true
			return &entry, nil
		}
	}

	// 2. Query Wiktionary REST API (fast, high uptime, Wikimedia CDN)
	entry, err := lookupWiktionary(cleanWord)
	if err != nil {
		// 3. Fallback to Free Dictionary API if Wiktionary did not return definitions
		fallbackEntry, fbErr := lookupFreeDictionaryAPI(cleanWord)
		if fbErr == nil && fallbackEntry != nil {
			entry = fallbackEntry
		} else {
			return nil, fmt.Errorf("no definition found for '%s'. Check your spelling or internet connection", cleanWord)
		}
	}

	// 4. Persist to SQLite cache for offline availability
	if entry != nil {
		if encoded, err := json.Marshal(entry); err == nil {
			_ = a.store.SaveCachedDefinition(cleanWord, string(encoded))
		}
	}

	return entry, nil
}

