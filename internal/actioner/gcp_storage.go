package actioner

import (
	"context"
	"fmt"

	"github.com/cloudedugcp/responseEngine/internal/config"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

type GCPStorage struct {
	cfg config.ActionerConfig
}

func (g *GCPStorage) Name() string {
	return "gcp_storage"
}

func (g *GCPStorage) Execute(ip string) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithCredentialsFile(g.cfg.CredentialsFile))
	if err != nil {
		return err
	}
	bucket := client.Bucket(g.cfg.BucketName)
	obj := bucket.Object(fmt.Sprintf("logs-%s.txt", ip))
	w := obj.NewWriter(ctx)
	// Запис логів у Storage
	if _, err := w.Write([]byte("Log data for " + ip)); err != nil {
		return err
	}
	return w.Close()
}
