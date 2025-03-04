package main

import (
	"log"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/server"
)

func main() {
	// Завантаження конфігурації
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Ініціалізація бази даних
	database, err := db.NewDatabase("./actions.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Ініціалізація діячів
	actioners := make(map[string]actioner.Actioner)
	for name, acfg := range cfg.Actioners {
		switch acfg.Type {
		case "gcp_firewall":
			fw, err := actioner.NewFirewallActioner(acfg) // Тип уже співпадає
			if err != nil {
				log.Printf("Failed to initialize firewall actioner: %v", err)
				continue
			}
			actioners[name] = fw
		case "gcp_storage":
			st, err := actioner.NewStorageActioner(acfg) // Тип уже співпадає
			if err != nil {
				log.Printf("Failed to initialize storage actioner: %v", err)
				continue
			}
			actioners[name] = st
		}
	}

	// Запуск сервера
	srv := server.NewServer(cfg, database, actioners)
	log.Printf("Starting server on port %s", cfg.Server.ListenPort)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
