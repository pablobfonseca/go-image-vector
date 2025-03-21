package worker

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pablobfonseca/go-image-vector/database"
	"github.com/pablobfonseca/go-image-vector/models"
	"github.com/pablobfonseca/go-image-vector/queue"
	"github.com/pablobfonseca/go-image-vector/services"
	"github.com/pgvector/pgvector-go"
	"github.com/spf13/viper"
)

// Task types
const (
	TaskTypeAnalyzeImage          = "analyze_image"
	TaskTypeAnalyzeMultipleImages = "analyze_multiple_images"
)

// Worker represents a background worker that processes tasks from a queue
type Worker struct {
	queueName  string
	numWorkers int
	stopChan   chan struct{}
	doneChan   chan struct{}
}

// NewWorker creates a new worker that processes tasks from the specified queue
func NewWorker(queueName string, numWorkers int) *Worker {
	return &Worker{
		queueName:  queueName,
		numWorkers: numWorkers,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
	}
}

// Start begins processing tasks from the queue
func (w *Worker) Start() {
	log.Printf("Starting %d workers for queue %s", w.numWorkers, w.queueName)

	for i := range w.numWorkers {
		go w.processItems(i)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping workers...")
		close(w.stopChan)
	}()
}

// Stop signals the workers to stop processing tasks
func (w *Worker) Stop() {
	log.Println("Stopping workers...")
	close(w.stopChan)

	// Wait for all workers to finish
	for range w.numWorkers {
		<-w.doneChan
	}

	log.Println("All workers stopped")
}

// processItems continuously processes tasks from the queue
func (w *Worker) processItems(workerID int) {
	log.Printf("Worker %d started", workerID)
	defer func() {
		log.Printf("Worker %d stopped", workerID)
		w.doneChan <- struct{}{}
	}()

	for {
		select {
		case <-w.stopChan:
			return
		default:
			// Try to get a task from the queue with a timeout
			task, err := queue.Dequeue(w.queueName, 5*time.Second)
			if err != nil {
				log.Printf("Error dequeueing task: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if task == nil {
				// No task available, try again after a short delay
				time.Sleep(500 * time.Millisecond)
				continue
			}

			log.Printf("Worker %d processing task %s of type %s", workerID, task.TaskID, task.TaskType)

			// Update task status to "processing"
			if err := queue.SetTaskStatus(task.TaskID, "processing"); err != nil {
				log.Printf("Error updating task status: %v", err)
			}

			// Process the task based on its type
			var processErr error
			var result map[string]any

			switch task.TaskType {
			case TaskTypeAnalyzeImage:
				result, processErr = processImageAnalysisTask(task)
			case TaskTypeAnalyzeMultipleImages:
				result, processErr = processMultipleImagesAnalysisTask(task)
			default:
				processErr = nil
				result = map[string]any{
					"error": "unknown task type",
				}
			}

			// Update task status based on result
			if processErr != nil {
				log.Printf("Error processing task %s: %v", task.TaskID, processErr)
				if err := queue.SetTaskStatus(task.TaskID, "failed"); err != nil {
					log.Printf("Error updating task status: %v", err)
				}
				if err := queue.StoreTaskResult(task.TaskID, map[string]any{
					"error": processErr.Error(),
				}); err != nil {
					log.Printf("Error storing task result: %v", err)
				}
			} else {
				if err := queue.SetTaskStatus(task.TaskID, "completed"); err != nil {
					log.Printf("Error updating task status: %v", err)
				}
				if err := queue.StoreTaskResult(task.TaskID, result); err != nil {
					log.Printf("Error storing task result: %v", err)
				}
			}
		}
	}
}

// processImageAnalysisTask processes an image analysis task
func processImageAnalysisTask(task *queue.TaskPayload) (map[string]any, error) {
	// Extract file path from task data
	filePath, ok := task.Data["file_path"].(string)
	if !ok {
		return nil, nil
	}

	// Extract text from image using AI
	text, err := services.ExtractTextFromImage(filePath)
	if err != nil {
		return nil, err
	}

	// Generate embedding from text
	embedding, err := services.GenerateEmbedding(text)
	if err != nil {
		return nil, err
	}

	// Save to database
	imageEntry := models.ImageEmbedding{
		FilePath:  filePath,
		Text:      text,
		Embedding: pgvector.NewVector(embedding),
	}

	if err := database.DB.Create(&imageEntry).Error; err != nil {
		return nil, err
	}

	// Return result
	return map[string]any{
		"id":        imageEntry.ID,
		"file_path": imageEntry.FilePath,
		"text":      imageEntry.Text,
	}, nil
}

// processMultipleImagesAnalysisTask processes a batch of images together for journey analysis
func processMultipleImagesAnalysisTask(task *queue.TaskPayload) (map[string]any, error) {
	// Extract file paths from task data
	filePaths, ok := task.Data["file_paths"].([]any)
	if !ok {
		return nil, nil
	}

	// Convert to string slice
	stringPaths := make([]string, 0, len(filePaths))
	for _, path := range filePaths {
		if strPath, ok := path.(string); ok {
			stringPaths = append(stringPaths, strPath)
		}
	}

	if len(stringPaths) == 0 {
		return nil, nil
	}

	// Get optional configuration from task data or use defaults
	maxChunkSize := viper.GetInt("BATCH_CHUNK_SIZE")
	maxParallel := viper.GetInt("BATCH_MAX_PARALLEL") // Default: run 4 parallel operations

	if val, ok := task.Data["max_chunk_size"].(float64); ok {
		maxChunkSize = int(val)
	}
	if val, ok := task.Data["max_parallel"].(float64); ok {
		maxParallel = int(val)
	}

	// Log processing configuration
	log.Printf("Processing batch with %d images: chunk_size=%d, parallel=%d",
		len(stringPaths), maxChunkSize, maxParallel)

	// Extract text from multiple images using parallel processing
	var journeyText string
	var err error

	// Time the operation
	startTime := time.Now()

	// If batch is small, use standard method, otherwise use parallel method
	if len(stringPaths) <= maxChunkSize {
		journeyText, err = services.ExtractTextFromMultipleImages(stringPaths)
	} else {
		journeyText, err = services.ParallelExtractTextFromImages(stringPaths, maxChunkSize, maxParallel)
	}

	processingTime := time.Since(startTime)
	log.Printf("Batch processing completed in %v", processingTime)

	if err != nil {
		return nil, err
	}

	// Generate embedding from the combined journey text
	embedding, err := services.GenerateEmbedding(journeyText)
	if err != nil {
		return nil, err
	}

	// Generate a batch ID to link all images in this batch
	batchID := task.TaskID

	// Create a combined record for the journey
	journeyEntry := models.ImageEmbedding{
		FilePath:   stringPaths[0],
		Text:       journeyText,
		Embedding:  pgvector.NewVector(embedding),
		IsBatch:    true,
		BatchID:    batchID,
		BatchPaths: stringPaths,
	}

	if err := database.DB.Create(&journeyEntry).Error; err != nil {
		return nil, err
	}

	// Return result with all file paths in the batch
	return map[string]any{
		"id":                 journeyEntry.ID,
		"file_path":          journeyEntry.FilePath,
		"text":               journeyEntry.Text,
		"file_count":         len(stringPaths),
		"is_batch":           true,
		"batch_id":           batchID,
		"batch_paths":        stringPaths,
		"processing_time_ms": processingTime.Milliseconds(),
	}, nil
}

// RunWorkers starts a pool of workers for image processing
func RunWorkers(ctx context.Context, numWorkers int) *Worker {
	worker := NewWorker(queue.ImageProcessingQueue, numWorkers)
	worker.Start()
	return worker
}
