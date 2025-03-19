package models

import "github.com/pgvector/pgvector-go"

type ImageEmbedding struct {
	ID         uint            `gorm:"primaryKey" json:"id"`
	FilePath   string          `gorm:"unique" json:"file_path"`
	Text       string          `gorm:"text" json:"text"`
	Embedding  pgvector.Vector `gorm:"type:vector(768)" json:"embedding"`
	IsBatch    bool            `gorm:"default:false" json:"is_batch"`
	BatchID    string          `gorm:"index" json:"batch_id"`
	BatchPaths []string        `gorm:"-" json:"batch_paths,omitempty"`
}
