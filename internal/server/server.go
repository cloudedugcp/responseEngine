package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
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
	Addr     string
	Fail2Ban bool
	BanTime  int
	LogPath  string
	JailName string
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
	mux.HandleFunc("/unban", s.handleUnban)
	return s
}

// Handle incoming POST requests with Falco events.
func (s *Server) handleFalcoEvents(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

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

		// Перевірка наявності IP у полях події
		var ip string
		if fields, ok := event["fields"].(map[string]interface{}); ok {
			if ipVal, ok := fields["source.ip"].(string); ok {
				ip = ipVal
			}
		}

		if ip != "" && s.config.Fail2Ban {
			s.blockIP(ip)
		}
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		s.logger.Error("Failed to write response", "error", err)
	}

	s.logger.Info("Request processed", "duration", time.Since(startTime))
}

func (s *Server) handleUnban(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.logger.Warn("Method not allowed", "method", r.Method)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := r.URL.Query().Get("ip")
	if ip == "" {
		s.logger.Warn("IP parameter missing")
		http.Error(w, "IP parameter required", http.StatusBadRequest)
		return
	}

	if err := s.UnblockIP(ip); err != nil {
		s.logger.Error("Unban failed", "ip", ip, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "IP %s unbanned", ip)
}

func (s *Server) blockIP(ip string) {
	// Enable Fail2Ban log
	logFile, err := os.OpenFile(s.config.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.logger.Error("Failed to open log file", "error", err)
		return
	}
	defer logFile.Close()
	fmt.Fprintf(logFile, "Blocked IP: %s\n", ip)

	// block ip Fail2Ban
	cmd := exec.Command("fail2ban-client", "set", s.config.JailName, "banip", ip)
	if err := cmd.Run(); err != nil {
		s.logger.Error("Failed to ban IP", "ip", ip, "error", err)
		return
	}
	s.logger.Info("IP banned", "ip", ip, "bantime", s.config.BanTime)

	// bantime Fail2Ban
	cmd = exec.Command("fail2ban-client", "set", s.config.JailName, "bantime", fmt.Sprintf("%d", s.config.BanTime))
	if err := cmd.Run(); err != nil {
		s.logger.Error("Failed to set bantime", "ip", ip, "bantime", s.config.BanTime, "error", err)
	}
}

func (s *Server) UnblockIP(ip string) error {
	if !s.config.Fail2Ban {
		return fmt.Errorf("Fail2Ban is disabled in config")
	}

	cmd := exec.Command("fail2ban-client", "set", s.config.JailName, "unbanip", ip)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unban IP %s: %w", ip, err)
	}
	s.logger.Info("IP unbanned", "ip", ip)
	return nil
}

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
