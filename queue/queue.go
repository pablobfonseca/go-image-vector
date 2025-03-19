package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

const (
	MediaProcessingQueue = "media_processing"
	ImageProcessingQueue = "image_processing" // Kept for backward compatibility
)

var (
	redisClient *redis.Client
	ctx         = context.Background()
)

type TaskPayload struct {
	TaskID   string         `json:"task_id"`
	TaskType string         `json:"task_type"`
	Data     map[string]any `json:"data"`
	Created  time.Time      `json:"created"`
}

// Initialize sets up the Redis connection
func Initialize() {
	redisAddr := viper.GetString("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisPassword := viper.GetString("REDIS_PASSWORD")
	redisDB := viper.GetInt("REDIS_DB")

	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	// Ping Redis to ensure connection is working
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Printf("Warning: Redis connection failed: %v. Queue functionality will be disabled.", err)
	} else {
		log.Println("Redis connected successfully")
	}
}

// Enqueue adds a task to the specified queue
func Enqueue(queueName string, taskType string, data map[string]any) (string, error) {
	if redisClient == nil {
		return "", fmt.Errorf("redis client not initialized")
	}

	taskID := fmt.Sprintf("%d", time.Now().UnixNano())
	task := TaskPayload{
		TaskID:   taskID,
		TaskType: taskType,
		Data:     data,
		Created:  time.Now(),
	}

	taskJSON, err := json.Marshal(task)
	if err != nil {
		return "", err
	}

	err = redisClient.RPush(ctx, queueName, taskJSON).Err()
	if err != nil {
		return "", err
	}

	return taskID, nil
}

// Dequeue retrieves a task from the queue with timeout
func Dequeue(queueName string, timeout time.Duration) (*TaskPayload, error) {
	if redisClient == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}

	// BLPOP blocks until an element is available, or until timeout
	result, err := redisClient.BLPop(ctx, timeout, queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No message available
		}
		return nil, err
	}

	// Result contains queue name at index 0 and payload at index 1
	if len(result) < 2 {
		return nil, fmt.Errorf("invalid result format from redis")
	}

	var task TaskPayload
	err = json.Unmarshal([]byte(result[1]), &task)
	if err != nil {
		return nil, err
	}

	return &task, nil
}

// GetTaskStatus retrieves the status of a task
func GetTaskStatus(taskID string) (string, error) {
	if redisClient == nil {
		return "", fmt.Errorf("redis client not initialized")
	}

	status, err := redisClient.Get(ctx, fmt.Sprintf("task:%s:status", taskID)).Result()
	if err != nil {
		if err == redis.Nil {
			return "unknown", nil
		}
		return "", err
	}

	return status, nil
}

// SetTaskStatus updates the status of a task
func SetTaskStatus(taskID string, status string) error {
	if redisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}

	return redisClient.Set(ctx, fmt.Sprintf("task:%s:status", taskID), status, 24*time.Hour).Err()
}

// StoreTaskResult stores the result of a completed task
func StoreTaskResult(taskID string, result map[string]any) error {
	if redisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}

	return redisClient.Set(ctx, fmt.Sprintf("task:%s:result", taskID), resultJSON, 24*time.Hour).Err()
}

// GetTaskResult retrieves the result of a completed task
func GetTaskResult(taskID string) (map[string]any, error) {
	if redisClient == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}

	resultJSON, err := redisClient.Get(ctx, fmt.Sprintf("task:%s:result", taskID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var result map[string]any
	err = json.Unmarshal([]byte(resultJSON), &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
