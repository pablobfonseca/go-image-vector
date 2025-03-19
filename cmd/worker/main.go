package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pablobfonseca/go-image-vector/database"
	"github.com/pablobfonseca/go-image-vector/queue"
	"github.com/pablobfonseca/go-image-vector/worker"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	// Set default values
	viper.SetDefault("WORKER_COUNT", 4)
	viper.SetDefault("REDIS_ADDR", "localhost:6379")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("REDIS_PASSWORD", "")

	if err := viper.ReadInConfig(); err != nil {
		log.Println("Warning: Error reading .env file:", err)
	}

	// Load environment variables
	viper.AutomaticEnv()

	// Connect to database
	database.Connect()

	// Initialize queue
	queue.Initialize()

	// Setup context with cancellation for clean shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get number of workers from config
	numWorkers := viper.GetInt("WORKER_COUNT")
	if numWorkers <= 0 {
		numWorkers = 4
	}

	log.Printf("Starting %d workers...", numWorkers)

	// Start worker pool
	workerPool := worker.RunWorkers(ctx, numWorkers)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Stopping workers...")
	workerPool.Stop()
	log.Println("Workers stopped")
}
