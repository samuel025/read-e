package library

import "time"

type BookMeta struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	FilePath    string    `json:"filePath"`
	CoverBase64 string    `json:"coverBase64"`
	AddedAt     time.Time `json:"addedAt"`
	HasProgress bool      `json:"hasProgress"`
}
