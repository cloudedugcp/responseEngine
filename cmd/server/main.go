package main

import (
	"log"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/server"
	"github.com/cloudedugcp/responseEngine/internal/types"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	database, err := db.NewDatabase("response_engine.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	actioners := make(map[string]types.Actioner)
	for name, actCfg := range cfg.Actioners {
		switch actCfg.Type {
		case "gcp_firewall":
			timeoutStr, ok := actCfg.Params["timeout"].(string)
			if !ok {
				log.Fatalf("Invalid timeout for actioner %s", name)
			}
			timeout, err := time.ParseDuration(timeoutStr)
			if err != nil {
				log.Fatalf("Failed to parse timeout for actioner %s: %v", name, err)
			}
			projectID, _ := actCfg.Params["project_id"].(string)
			credentialsFile, _ := actCfg.Params["credentials_file"].(string)

			firewallActioner, err := actioner.NewFirewallActioner(projectID, credentialsFile, timeout, database)
			if err != nil {
				log.Fatalf("Failed to initialize firewall actioner %s: %v", name, err)
			}
			actioners[name] = firewallActioner

		case "sigma":
			sigmaActioner, err := actioner.NewSigmaActioner(actCfg, database)
			if err != nil {
				log.Fatalf("Failed to initialize sigma actioner %s: %v", name, err)
			}
			actioners[name] = sigmaActioner

		case "storage":
			storageActioner, err := actioner.NewStorageActioner(actCfg, database)
			if err != nil {
				log.Fatalf("Failed to initialize storage actioner %s: %v", name, err)
			}
			actioners[name] = storageActioner

		default:
			log.Printf("Unknown actioner type: %s", actCfg.Type)
		}
	}

	srv := server.NewServer(cfg, database, actioners)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
