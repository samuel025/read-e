package library

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"epub-reader/internal/epub"
)

var pdfPageRegex = regexp.MustCompile(`(?i)/Type\s*/Page\b`)
var pdfCountRegex = regexp.MustCompile(`/Count\s+(\d+)`)

func CountPDFPages(filePath string) int {
	f, err := os.Open(filePath)
	if err != nil {
		return 0
	}
	defer f.Close()

	buf := make([]byte, 128*1024)
	var overlap []byte
	count := 0
	maxCountHeader := 0

	for {
		n, err := f.Read(buf)
		if n > 0 {
			chunk := append(overlap, buf[:n]...)
			matches := pdfPageRegex.FindAll(chunk, -1)
			count += len(matches)

			if maxCountHeader == 0 {
				cMatches := pdfCountRegex.FindAllSubmatch(chunk, -1)
				for _, cm := range cMatches {
					if len(cm) > 1 {
						var val int
						if _, scanErr := fmt.Sscanf(string(cm[1]), "%d", &val); scanErr == nil && val > maxCountHeader {
							maxCountHeader = val
						}
					}
				}
			}

			if len(chunk) > 64 {
				overlap = make([]byte, 64)
				copy(overlap, chunk[len(chunk)-64:])
			} else {
				overlap = nil
			}
		}
		if err != nil {
			break
		}
	}

	if count > 0 {
		return count
	}
	return maxCountHeader
}

func ScanFolder(folderPath string) ([]BookMeta, error) {
	var books []BookMeta

	err := filepath.Walk(folderPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if ext != ".epub" && ext != ".pdf" {
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
	absPath, _ := filepath.Abs(filePath)
	hash := sha256.Sum256([]byte(absPath))
	id := fmt.Sprintf("%x", hash[:8])
	ext := strings.ToLower(filepath.Ext(filePath))

	if ext == ".pdf" {
		title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
		totalCount := CountPDFPages(absPath)
		return BookMeta{
			ID:          id,
			Title:       title,
			Author:      "",
			FilePath:    absPath,
			CoverBase64: "",
			Format:      "pdf",
			TotalCount:  totalCount,
			AddedAt:     time.Now(),
		}, nil
	}

	reader, err := epub.Open(filePath)
	if err != nil {
		return BookMeta{}, err
	}
	defer reader.Close()

	info := reader.Info()

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
		Format:      "epub",
		TotalCount:  info.SpineCount,
		AddedAt:     time.Now(),
	}, nil
}
