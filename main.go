package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/pablobfonseca/go-image-vector/database"
	"github.com/pablobfonseca/go-image-vector/models"
	"github.com/pablobfonseca/go-image-vector/services"
	"github.com/pgvector/pgvector-go"
	"github.com/rs/cors"
	"github.com/spf13/viper"
)

func uploadImage(w http.ResponseWriter, r *http.Request) {
	// Ensure uploads directory exists
	uploadsDir := "./uploads"
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(uploadsDir, 0755); err != nil {
			http.Error(w, "Failed to create uploads directory", http.StatusInternalServerError)
			return
		}
	}

	r.ParseMultipartForm(10 << 20) // 10M max upload size
	file, handler, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Failed to upload file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create a unique filename with original extension
	filePath := fmt.Sprintf("%s/%d_%s", uploadsDir,
		time.Now().UnixNano(), handler.Filename)

	out, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Failed to save file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		http.Error(w, "Failed while copying file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	text, err := services.ExtractTextFromImage(filePath)
	if err != nil {
		http.Error(w, "Failed to analyze image: "+err.Error(), http.StatusInternalServerError)
		return
	}

	embedding, err := services.GenerateEmbedding(text)
	if err != nil {
		http.Error(w, "Failed to generate embedding: "+err.Error(), http.StatusInternalServerError)
		return
	}

	imageEntry := models.ImageEmbedding{FilePath: filePath, Text: text, Embedding: pgvector.NewVector(embedding)}
	if err := database.DB.Create(&imageEntry).Error; err != nil {
		http.Error(w, "Failed to save to database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Image uploaded & analyzed successfully", "text": text})
}

func searchImages(w http.ResponseWriter, r *http.Request) {
	var req struct {
		QueryText string `json:"query"`
		TopK      int    `json:"top_k"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	queryEmbedding, err := services.GenerateEmbedding(req.QueryText)
	if err != nil {
		http.Error(w, "Failed to generate embedding", http.StatusBadRequest)
		return
	}

	var results []models.ImageEmbedding
	if err := database.DB.Raw(`SELECT * FROM image_embeddings ORDER BY embedding <-> ? LIMIT ?`,
		pgvector.NewVector(queryEmbedding), req.TopK).Scan(&results).Error; err != nil {
		http.Error(w, "Failed to search database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func main() {
	database.Connect()

	r := mux.NewRouter()

	r.HandleFunc("/upload", uploadImage).Methods("POST")
	r.HandleFunc("/search", searchImages).Methods("POST")

	// Serve static files from the "uploads" directory
	uploadsDir := "./uploads"
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(uploadsDir, 0755); err != nil {
			log.Fatal("Failed to create uploads directory:", err)
		}
	}
	fs := http.FileServer(http.Dir(uploadsDir))
	r.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", fs))

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	handler := c.Handler(r)

	fmt.Println("Server running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

func init() {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	if err := viper.ReadInConfig(); err != nil {
		log.Println("Warning: Error reading .env file:", err)
	}
}
