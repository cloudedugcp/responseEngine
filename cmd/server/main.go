package main

import (
	"log"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/server" // Коректний імпорт пакету
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
	var firewallProjectID, firewallCredsFile string
	var firewallTimeout time.Duration
	for _, sa := range cfg.Scenarios {
		for _, act := range sa.Actioners {
			if act.Name == "firewall" {
				projectID, ok := act.Params["project_id"].(string)
				if !ok {
					log.Fatalf("firewall: project_id must be a string")
				}
				firewallProjectID = projectID

				credsFile, ok := act.Params["credentials_file"].(string)
				if !ok {
					log.Fatalf("firewall: credentials_file must be a string")
				}
				firewallCredsFile = credsFile

				timeoutStr, ok := act.Params["timeout"].(string)
				if !ok {
					log.Fatalf("firewall: timeout must be a string")
				}
				firewallTimeout, err = time.ParseDuration(timeoutStr)
				if err != nil {
					log.Fatalf("firewall: invalid timeout value %s: %v", timeoutStr, err)
				}

				firewallActioner, err := actioner.NewFirewallActioner(firewallProjectID, firewallCredsFile, firewallTimeout, database)
				if err != nil {
					log.Fatalf("Failed to initialize firewall actioner: %v", err)
				}
				actioners["firewall"] = firewallActioner
			}
			// Додайте ініціалізацію інших actioners (наприклад, storage, sigma) за потреби
		}
	}

	srv := server.NewServer(cfg, database, actioners)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
