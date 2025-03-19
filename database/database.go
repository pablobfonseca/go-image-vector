package database

import (
	"fmt"
	"log"

	"github.com/pablobfonseca/go-image-vector/models"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() {
	host := viper.GetString("DB_HOST")
	user := viper.GetString("DB_USER")
	password := viper.GetString("DB_PASSWORD")
	dbname := viper.GetString("DB_NAME")
	port := viper.GetString("DB_PORT")
	sslmode := viper.GetString("DB_SSLMODE")

	// Validate that all required environment variables are set
	if host == "" || user == "" || password == "" || dbname == "" || port == "" || sslmode == "" {
		log.Fatal("Missing required database environment variables. Please ensure DB_HOST, DB_USER, DB_PASSWORD, DB_NAME, DB_PORT, and DB_SSLMODE are set")
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		host, user, password, dbname, port, sslmode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	db.Exec("CREATE EXTENSION IF NOT EXISTS vector;")
	db.Exec("CREATE INDEX IF NOT EXISTS ON image_embeddings USING hnsw (embedding vector_cosine_ops);")
	db.AutoMigrate(&models.ImageEmbedding{})

	DB = db
	fmt.Println("Database connected successfully!")
}
