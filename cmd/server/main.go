package main

import (
	"log"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/server"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	database, err := db.NewDatabase("./response_engine.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Ініціалізація actioners
	actioners := make(map[string]actioner.Actioner)

	// Налаштування для FirewallActioner
	for _, sa := range cfg.Scenarios {
		for _, act := range sa.Actioners {
			if act.Name == "firewall" {
				// Перевірка project_id
				projID, exists := act.Params["project_id"]
				if !exists {
					log.Fatalf("firewall: project_id is missing in config")
				}
				firewallProjectID, ok := projID.(string)
				if !ok {
					log.Fatalf("firewall: project_id must be a string, got %T", projID)
				}

				// Перевірка credentials_file
				credsFile, exists := act.Params["credentials_file"]
				if !exists {
					log.Fatalf("firewall: credentials_file is missing in config")
				}
				firewallCredsFile, ok := credsFile.(string)
				if !ok {
					log.Fatalf("firewall: credentials_file must be a string, got %T", credsFile)
				}

				// Перевірка timeout
				timeout, exists := act.Params["timeout"]
				if !exists {
					log.Fatalf("firewall: timeout is missing in config")
				}
				timeoutStr, ok := timeout.(string)
				if !ok {
					log.Fatalf("firewall: timeout must be a string, got %T", timeout)
				}
				firewallTimeout, err := time.ParseDuration(timeoutStr)
				if err != nil {
					log.Fatalf("firewall: invalid timeout value %s: %v", timeoutStr, err)
				}

				firewallActioner, err := actioner.NewFirewallActioner(firewallProjectID, firewallCredsFile, firewallTimeout, database)
				if err != nil {
					log.Fatalf("Failed to initialize firewall actioner: %v", err)
				}
				actioners["firewall"] = firewallActioner
			}
			// Додайте ініціалізацію інших actioners (storage, sigma) за потреби
		}
	}

	srv := server.NewServer(cfg, database, actioners)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
