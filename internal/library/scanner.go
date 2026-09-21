package library

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"epub-reader/internal/epub"
)

func ScanFolder(folderPath string) ([]BookMeta, error) {
	var books []BookMeta

	err := filepath.Walk(folderPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".epub") {
			return nil
		}

		meta, parseErr := extractMeta(p)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", p, parseErr)
			return nil
		}

		books = append(books, meta)
		return nil
	})

	return books, err
}

func ScanFile(filePath string) (BookMeta, error) {
	return extractMeta(filePath)
}

func extractMeta(filePath string) (BookMeta, error) {
	reader, err := epub.Open(filePath)
	if err != nil {
		return BookMeta{}, err
	}
	defer reader.Close()

	info := reader.Info()

	absPath, _ := filepath.Abs(filePath)
	hash := sha256.Sum256([]byte(absPath))
	id := fmt.Sprintf("%x", hash[:8])

	title := info.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}

	return BookMeta{
		ID:          id,
		Title:       title,
		Author:      info.Author,
		FilePath:    absPath,
		CoverBase64: info.CoverBase64,
		AddedAt:     time.Now(),
	}, nil
}
