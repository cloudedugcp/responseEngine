package actioner

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/storage"
	"github.com/cloudedugcp/responseEngine/internal/config"
	"gopkg.in/yaml.v2"
)

// SigmaHQActioner конвертує подію в формат SigmaHQ і записує її в Google Cloud Storage.
type SigmaHQActioner struct {
	cfg config.ActionerConfig
}

// SigmaHQEvent представляє структуру події у форматі SigmaHQ.
type SigmaHQEvent struct {
	Title       string `yaml:"title"`
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Level       string `yaml:"level"`
	LogSource   struct {
		Product string `yaml:"product"`
		Service string `yaml:"service"`
	} `yaml:"logsource"`
	Detection struct {
		Selection map[string]string `yaml:"selection"`
		Condition string            `yaml:"condition"`
	} `yaml:"detection"`
	Fields         []string `yaml:"fields"`
	FalsePositives []string `yaml:"falsepositives"`
}

// NewSigmaHQActioner створює новий SigmaHQActioner.
func NewSigmaHQActioner(cfg config.ActionerConfig) *SigmaHQActioner {
	return &SigmaHQActioner{cfg: cfg}
}

// Execute конвертує подію в SigmaHQ формат і записує в бакет.
func (s *SigmaHQActioner) Execute(ip string) error {
	// Отримуємо bucket_name із конфігурації напряму
	bucketName := s.cfg.BucketName
	if bucketName == "" {
		return fmt.Errorf("missing bucket_name in SigmaHQActioner config")
	}

	// Формуємо SigmaHQ подію
	sigmaEvent := SigmaHQEvent{
		Title:       fmt.Sprintf("Detected Failed SSH Login Attempt for IP %s", ip),
		ID:          fmt.Sprintf("event-%s-%s", ip, time.Now().Format("20060102T150405Z")),
		Description: "Detects failed SSH login attempts based on Falco event",
		Level:       "medium",
		LogSource: struct {
			Product string `yaml:"product"`
			Service string `yaml:"service"`
		}{
			Product: "falco",
			Service: "ssh",
		},
		Detection: struct {
			Selection map[string]string `yaml:"selection"`
			Condition string            `yaml:"condition"`
		}{
			Selection: map[string]string{
				"rule":   "Detect Failed SSH Login Attempts",
				"fd.rip": ip,
			},
			Condition: "selection",
		},
		Fields:         []string{"rule", "fd.rip"},
		FalsePositives: []string{"Legitimate SSH login attempts"},
	}

	// Конвертуємо в YAML
	yamlData, err := yaml.Marshal(&sigmaEvent)
	if err != nil {
		log.Printf("Failed to marshal SigmaHQ event to YAML: %v", err)
		return err
	}

	// Ініціалізуємо клієнт Google Cloud Storage
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("Failed to create GCS client: %v", err)
		return err
	}
	defer client.Close()

	// Формуємо ім’я файлу
	fileName := fmt.Sprintf("sigmahq/%s-%s.yaml", ip, time.Now().Format("20060102T150405Z"))
	bucket := client.Bucket(bucketName)
	obj := bucket.Object(fileName)

	// Записуємо файл у бакет
	w := obj.NewWriter(ctx)
	if _, err := w.Write(yamlData); err != nil {
		log.Printf("Failed to write to GCS bucket %s: %v", bucketName, err)
		return err
	}
	if err := w.Close(); err != nil {
		log.Printf("Failed to close GCS writer: %v", err)
		return err
	}

	log.Printf("Successfully wrote SigmaHQ event to GCS bucket %s as %s", bucketName, fileName)
	return nil
}

// Name повертає ім’я діяча.
func (s *SigmaHQActioner) Name() string {
	return "sigmahq"
}
