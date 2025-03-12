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

	// Ініціалізація FirewallActioner
	if firewallCfg, ok := cfg.Actioners["firewall"]; ok && firewallCfg.Type == "gcp_firewall" {
		// Глобальні параметри з actioners.firewall.params
		projID, exists := firewallCfg.Params["project_id"]
		if !exists {
			log.Fatalf("firewall: project_id is missing in global actioners config")
		}
		firewallProjectID, ok := projID.(string)
		if !ok {
			log.Fatalf("firewall: project_id must be a string, got %T", projID)
		}

		credsFile, exists := firewallCfg.Params["credentials_file"]
		if !exists {
			log.Fatalf("firewall: credentials_file is missing in global actioners config")
		}
		firewallCredsFile, ok := credsFile.(string)
		if !ok {
			log.Fatalf("firewall: credentials_file must be a string, got %T", credsFile)
		}

		timeout, exists := firewallCfg.Params["timeout"]
		if !exists {
			log.Fatalf("firewall: timeout is missing in global actioners config")
		}
		var firewallTimeout time.Duration
		switch t := timeout.(type) {
		case string:
			firewallTimeout, err = time.ParseDuration(t)
			if err != nil {
				log.Fatalf("firewall: invalid timeout value %s: %v", t, err)
			}
		case float64:
			firewallTimeout = time.Duration(t) * time.Second // Число як секунди
		case int:
			firewallTimeout = time.Duration(t) * time.Second // Додано підтримку int
		default:
			log.Fatalf("firewall: timeout must be a string, number, or integer, got %T", timeout)
		}

		firewallActioner, err := actioner.NewFirewallActioner(firewallProjectID, firewallCredsFile, firewallTimeout, database)
		if err != nil {
			log.Fatalf("Failed to initialize firewall actioner: %v", err)
		}
		actioners["firewall"] = firewallActioner
	}

	// Додайте ініціалізацію інших actioners за потреби (наприклад, storage, sigma)
	if storageCfg, ok := cfg.Actioners["storage"]; ok && storageCfg.Type == "gcp_storage" {
		// Ініціалізація storage (додайте свою логіку)
		log.Println("Storage actioner initialized (placeholder)")
	}

	if sigmaCfg, ok := cfg.Actioners["sigma"]; ok && sigmaCfg.Type == "sigma_storage" {
		// Ініціалізація sigma (додайте свою логіку)
		log.Println("Sigma actioner initialized (placeholder)")
	}

	srv := server.NewServer(cfg, database, actioners)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
