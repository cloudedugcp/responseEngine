package actioner

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/storage"
	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/types"
)

type StorageActioner struct {
	client *storage.Client
	bucket string
	db     *db.Database
}

func NewStorageActioner(cfg config.ActionerConfig, db *db.Database) (*StorageActioner, error) {
	bucket, ok := cfg.Params["bucket"].(string)
	if !ok {
		return nil, fmt.Errorf("bucket must be specified in params")
	}
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %v", err)
	}
	return &StorageActioner{client: client, bucket: bucket, db: db}, nil
}

func (sa *StorageActioner) Execute(event types.Event, params map[string]interface{}) error {
	objectName, ok := params["object_name"].(string)
	if !ok {
		return fmt.Errorf("object_name must be a string")
	}
	ctx := context.Background()
	w := sa.client.Bucket(sa.bucket).Object(objectName).NewWriter(ctx)
	defer w.Close()

	logData := fmt.Sprintf("IP: %s, Rule: %s, Log: %s, Timestamp: %s", event.IP, event.RuleName, event.Log, event.Timestamp)
	if _, err := w.Write([]byte(logData)); err != nil {
		return fmt.Errorf("failed to write to storage: %v", err)
	}
	log.Printf("Stored event in bucket %s, object %s", sa.bucket, objectName)
	return nil
}

func (sa *StorageActioner) Name() string {
	return "storage"
}
