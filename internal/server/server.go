package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Server struct {
	config Config
	logger *slog.Logger
	srv    *http.Server
}

type Config struct {
	Addr string
}

// Create Server instance with the given configuration and logger.
func New(config Config, logger *slog.Logger) *Server {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    config.Addr,
		Handler: mux,
	}

	s := &Server{
		config: config,
		logger: logger,
		srv:    srv,
	}
	mux.HandleFunc("/falco-events", s.handleFalcoEvents)
	return s
}

// Handle incoming POST requests with Falco events.
func (s *Server) handleFalcoEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		s.logger.Warn("Method not allowed", "method", r.Method, "path", r.URL.Path)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.URL.Path != "/falco-events" {
		s.logger.Warn("Invalid endpoint", "path", r.URL.Path)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	for {
		var event map[string]interface{}
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			s.logger.Error("Failed to decode JSON", "error", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		s.logger.Info("Received Falco event",
			"time", event["time"],
			"rule", event["rule"],
			"priority", event["priority"],
		)

		eventJSON, _ := json.MarshalIndent(event, "", "  ")
		fmt.Println("Parsed Falco event:")
		fmt.Println(string(eventJSON))
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		s.logger.Error("Failed to write response", "error", err)
	}
}

// Run server and handles graceful shutdown
func (s *Server) Run(ctx context.Context) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		s.logger.Info("Starting server", "addr", s.config.Addr)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("Shutting down server due to context cancellation")
	case sig := <-sigChan:
		s.logger.Info("Shutting down server due to signal", "signal", sig)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("Failed to shutdown server gracefully", "error", err)
		return fmt.Errorf("shutdown error: %w", err)
	}

	s.logger.Info("Server stopped")
	return nil
}
