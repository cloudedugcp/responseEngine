package actioner

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/storage"
)

// StorageActioner - діяч для Google Cloud Storage
type StorageActioner struct {
	bucketName string
	logCount   int
	client     *storage.Client
}

// NewStorageActioner - створює новий StorageActioner
func NewStorageActioner(cfg ActionerConfig) (*StorageActioner, error) {
	client, err := storage.NewClient(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %v", err)
	}

	// Обробка log_count із підтримкою int і float64
	var logCount int
	switch v := cfg.Params["log_count"].(type) {
	case int:
		logCount = v
	case float64:
		logCount = int(v)
	default:
		return nil, fmt.Errorf("log_count must be a number, got %T", v)
	}

	return &StorageActioner{
		bucketName: cfg.Params["bucket_name"].(string),
		logCount:   logCount,
		client:     client,
	}, nil
}

// Execute - зберігає лог у Google Cloud Storage
func (sa *StorageActioner) Execute(event Event, params map[string]interface{}) error {
	prefix := params["prefix"].(string)
	ctx := context.Background()
	bucket := sa.client.Bucket(sa.bucketName)
	objectName := fmt.Sprintf("%s%s_%d", prefix, event.IP, time.Now().UnixNano())

	w := bucket.Object(objectName).NewWriter(ctx)
	defer w.Close()

	logData := fmt.Sprintf("IP: %s, Rule: %s, Time: %s", event.IP, event.RuleName, time.Now().Format(time.RFC3339))
	if _, err := w.Write([]byte(logData)); err != nil {
		log.Printf("Failed to write to storage: %v", err)
		return err
	}
	return nil
}

// Name - повертає ім'я діяча
func (sa *StorageActioner) Name() string { return "storage" }
