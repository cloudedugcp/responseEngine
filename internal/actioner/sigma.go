package actioner

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
	"gopkg.in/yaml.v3"
)

// SigmaActioner - діяч для конвертації логів Falco у формат SigmaHQ і збереження в Google Cloud Storage
type SigmaActioner struct {
	bucketName string
	client     *storage.Client
}

// NewSigmaActioner - створює новий SigmaActioner
func NewSigmaActioner(cfg ActionerConfig) (*SigmaActioner, error) {
	var clientOptions []option.ClientOption
	if credsFile, ok := cfg.Params["credentials_file"].(string); ok && credsFile != "" {
		clientOptions = append(clientOptions, option.WithCredentialsFile(credsFile))
	}

	client, err := storage.NewClient(context.Background(), clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create sigma storage client: %v", err)
	}

	return &SigmaActioner{
		bucketName: cfg.Params["bucket_name"].(string),
		client:     client,
	}, nil
}

// SigmaRule - структура для базового Sigma-запису
type SigmaRule struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	LogSource   struct {
		Category string `yaml:"category"`
		Product  string `yaml:"product"`
	} `yaml:"logsource"`
	Detection struct {
		Selection map[string]string `yaml:"selection"`
		Condition string            `yaml:"condition"`
	} `yaml:"detection"`
	Fields []string `yaml:"fields"`
	Level  string   `yaml:"level"`
}

// Execute - перетворює лог Falco у Sigma-запис і зберігає в Google Cloud Storage
func (sa *SigmaActioner) Execute(event Event, params map[string]interface{}) error {
	prefix := params["prefix"].(string)
	ctx := context.Background()
	bucket := sa.client.Bucket(sa.bucketName)
	objectName := fmt.Sprintf("%s%s_%d.yaml", prefix, event.IP, time.Now().UnixNano())

	// Формуємо базовий Sigma-запис
	sigmaRule := SigmaRule{
		Title:       fmt.Sprintf("Suspicious Activity Detected for IP %s", event.IP),
		Description: fmt.Sprintf("Detected %s: %s", event.RuleName, event.Log),
		LogSource: struct {
			Category string `yaml:"category"`
			Product  string `yaml:"product"`
		}{
			Category: "network",
			Product:  "falco",
		},
		Detection: struct {
			Selection map[string]string `yaml:"selection"`
			Condition string            `yaml:"condition"`
		}{
			Selection: map[string]string{
				"src_ip": event.IP,
				"event":  event.RuleName,
			},
			Condition: "selection",
		},
		Fields: []string{"src_ip", "event"},
		Level:  "high",
	}

	// Перетворюємо у YAML
	yamlData, err := yaml.Marshal(&sigmaRule)
	if err != nil {
		log.Printf("Failed to marshal Sigma rule for IP %s: %v", event.IP, err)
		return fmt.Errorf("failed to marshal Sigma rule: %v", err)
	}

	// Завантажуємо у Storage
	w := bucket.Object(objectName).NewWriter(ctx)
	if _, err := w.Write(yamlData); err != nil {
		log.Printf("Failed to write Sigma rule to storage object %s: %v", objectName, err)
		return fmt.Errorf("failed to write Sigma rule to storage: %v", err)
	}
	if err := w.Close(); err != nil {
		log.Printf("Failed to close writer for storage object %s: %v", objectName, err)
		return fmt.Errorf("failed to close Sigma storage writer: %v", err)
	}

	// Перевірка запису
	attrs, err := bucket.Object(objectName).Attrs(ctx)
	if err != nil {
		log.Printf("Failed to verify Sigma storage object %s: %v", objectName, err)
		return fmt.Errorf("failed to verify Sigma storage object: %v", err)
	}
	log.Printf("Successfully wrote Sigma rule (%d bytes) to storage object %s", attrs.Size, objectName)

	return nil
}

// Name - повертає ім'я діяча
func (sa *SigmaActioner) Name() string { return "sigma" }
