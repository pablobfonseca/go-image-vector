package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/pablobfonseca/go-image-vector/database"
	"github.com/pablobfonseca/go-image-vector/models"
	"github.com/pablobfonseca/go-image-vector/queue"
	"github.com/pablobfonseca/go-image-vector/services"
	"github.com/pablobfonseca/go-image-vector/worker"
	"github.com/pgvector/pgvector-go"
	"github.com/rs/cors"
	"github.com/spf13/viper"
)

// uploadImage handles image uploads and queues analysis tasks
func uploadImage(w http.ResponseWriter, r *http.Request) {
	uploadsDir := "./uploads"
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(uploadsDir, 0755); err != nil {
			http.Error(w, "Failed to create uploads directory", http.StatusInternalServerError)
			return
		}
	}

	r.ParseMultipartForm(50 << 20)

	form := r.MultipartForm
	files := form.File["images"]

	if len(files) == 0 {
		http.Error(w, "No images uploaded", http.StatusBadRequest)
		return
	}

	if len(files) > 5 {
		http.Error(w, "Maximum 5 images allowed", http.StatusBadRequest)
		return
	}

	// Check if batch analysis is requested
	batchAnalyze := r.FormValue("batch_analyze") == "true"

	taskIDs := []string{}
	filePaths := []string{}

	// Save all the uploaded files
	for _, handler := range files {
		file, err := handler.Open()
		if err != nil {
			http.Error(w, "Failed to open uploaded file: "+err.Error(), http.StatusBadRequest)
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

		filePaths = append(filePaths, filePath)

		// If not doing batch analysis, queue each image individually
		if !batchAnalyze {
			// Queue the image analysis task
			taskData := map[string]any{
				"file_path": filePath,
			}

			taskID, err := queue.Enqueue(queue.ImageProcessingQueue, worker.TaskTypeAnalyzeImage, taskData)
			if err != nil {
				http.Error(w, "Failed to queue image for processing: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Set initial task status
			queue.SetTaskStatus(taskID, "pending")
			taskIDs = append(taskIDs, taskID)
		}
	}

	// If batch analysis is requested, queue a single task for all images
	if batchAnalyze && len(filePaths) > 0 {
		// Queue the batch analysis task
		taskData := map[string]any{
			"file_paths": filePaths,
		}

		taskID, err := queue.Enqueue(queue.ImageProcessingQueue, worker.TaskTypeAnalyzeMultipleImages, taskData)
		if err != nil {
			http.Error(w, "Failed to queue batch image analysis: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Set initial task status
		queue.SetTaskStatus(taskID, "pending")
		taskIDs = append(taskIDs, taskID)
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"message":       "Images uploaded and queued for processing",
		"task_ids":      taskIDs,
		"batch_analyze": batchAnalyze,
	})
}

// getTaskStatus retrieves the status of a task
func getTaskStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["taskID"]

	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	status, err := queue.GetTaskStatus(taskID)
	if err != nil {
		http.Error(w, "Failed to get task status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if status == "completed" {
		result, err := queue.GetTaskResult(taskID)
		if err != nil {
			http.Error(w, "Failed to get task result: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]any{
			"task_id": taskID,
			"status":  status,
			"result":  result,
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"task_id": taskID,
		"status":  status,
	})
}

// searchImages finds similar images based on text query
func searchImages(w http.ResponseWriter, r *http.Request) {
	var req struct {
		QueryText string `json:"query"`
		TopK      int    `json:"top_k"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.TopK <= 0 {
		req.TopK = 5
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

	// For batch results, fetch the associated image paths if they exist
	for i, result := range results {
		if result.IsBatch && result.BatchID != "" {
			// Get all the batch paths for this batch from Redis
			batchResult, err := queue.GetTaskResult(result.BatchID)
			if err == nil && batchResult != nil {
				if batchPaths, ok := batchResult["batch_paths"].([]any); ok {
					// Convert the interface slice to string slice
					stringPaths := make([]string, 0, len(batchPaths))
					for _, path := range batchPaths {
						if strPath, ok := path.(string); ok {
							stringPaths = append(stringPaths, strPath)
						}
					}
					results[i].BatchPaths = stringPaths
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func main() {
	database.Connect()

	queue.Initialize()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	numWorkers := viper.GetInt("WORKER_COUNT")
	if numWorkers <= 0 {
		numWorkers = 4
	}

	workerPool := worker.RunWorkers(ctx, numWorkers)
	defer workerPool.Stop()

	r := mux.NewRouter()
	apiRouter := r.PathPrefix("/api/v1").Subrouter()

	apiRouter.HandleFunc("/upload", uploadImage).Methods("POST")
	apiRouter.HandleFunc("/search", searchImages).Methods("POST")
	apiRouter.HandleFunc("/tasks/{taskID}", getTaskStatus).Methods("GET")

	r.HandleFunc("/upload", uploadImage).Methods("POST")
	r.HandleFunc("/search", searchImages).Methods("POST")

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

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", getPort()),
		Handler: handler,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("Server starting on port %s...\n", getPort())
		serverErrors <- srv.ListenAndServe()
	}()

	// Listen for OS signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received or an error occurs
	select {
	case err := <-serverErrors:
		log.Fatalf("Error starting server: %v", err)

	case <-shutdown:
		log.Println("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := srv.Shutdown(ctx)
		if err != nil {
			log.Printf("Error during server shutdown: %v", err)
			err = srv.Close()
		}

		switch {
		case err != nil:
			log.Fatalf("Error during server shutdown: %v", err)
		case ctx.Err() != nil:
			log.Fatalf("Timeout during shutdown: %v", ctx.Err())
		}
	}
}

// getPort returns the port to listen on
func getPort() string {
	port := viper.GetString("PORT")
	if port == "" {
		port = "8080"
	}
	return port
}

func init() {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	viper.SetDefault("PORT", "8080")
	viper.SetDefault("WORKER_COUNT", 4)
	viper.SetDefault("REDIS_ADDR", "localhost:6379")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("REDIS_PASSWORD", "")

	if err := viper.ReadInConfig(); err != nil {
		log.Println("Warning: Error reading .env file:", err)
	}
}
