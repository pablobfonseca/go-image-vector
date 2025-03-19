package models

import "github.com/pgvector/pgvector-go"

type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
)

type MediaEmbedding struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	FilePath  string          `gorm:"unique" json:"file_path"`
	MediaType MediaType       `gorm:"type:varchar(10)" json:"media_type"`
	Text      string          `gorm:"text" json:"text"`
	Embedding pgvector.Vector `gorm:"type:vector(768)" json:"embedding"`
}

// For backward compatibility
type ImageEmbedding = MediaEmbedding
