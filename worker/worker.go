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
)

// Task types
const (
	TaskTypeAnalyzeMedia = "analyze_media"
	TaskTypeAnalyzeImage = "analyze_image" // Kept for backward compatibility
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
			case TaskTypeAnalyzeMedia, TaskTypeAnalyzeImage:
				result, processErr = processImageAnalysisTask(task)
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

// processMediaAnalysisTask processes a media (image or video) analysis task
func processMediaAnalysisTask(task *queue.TaskPayload) (map[string]any, error) {
	// Extract file path from task data
	filePath, ok := task.Data["file_path"].(string)
	if !ok {
		return nil, nil
	}

	// Detect media type
	mediaType := services.DetectMediaType(filePath)

	// Extract text from media using AI
	text, err := services.ExtractTextFromMedia(filePath)
	if err != nil {
		return nil, err
	}

	// Generate embedding from text
	embedding, err := services.GenerateEmbedding(text)
	if err != nil {
		return nil, err
	}

	// Save to database
	mediaEntry := models.MediaEmbedding{
		FilePath:  filePath,
		MediaType: mediaType,
		Text:      text,
		Embedding: pgvector.NewVector(embedding),
	}

	if err := database.DB.Create(&mediaEntry).Error; err != nil {
		return nil, err
	}

	// Return result
	return map[string]interface{}{
		"id":         mediaEntry.ID,
		"file_path":  mediaEntry.FilePath,
		"media_type": mediaEntry.MediaType,
		"text":       mediaEntry.Text,
	}, nil
}

// processImageAnalysisTask handles legacy image analysis tasks
func processImageAnalysisTask(task *queue.TaskPayload) (map[string]any, error) {
	return processMediaAnalysisTask(task)
}

// RunWorkers starts a pool of workers for media processing
func RunWorkers(ctx context.Context, numWorkers int) *Worker {
	worker := NewWorker(queue.MediaProcessingQueue, numWorkers)
	worker.Start()
	return worker
}
