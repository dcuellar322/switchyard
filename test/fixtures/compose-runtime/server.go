// Command compose-runtime-fixture serves an offline Docker integration target.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	//nolint:gosec // G301: this non-secret directory is shared with the fixture's Docker volume.
	if err := os.MkdirAll("/state", 0o755); err != nil {
		return err
	}
	//nolint:gosec // G306: the readiness marker contains only a timestamp and is intentionally container-readable.
	if err := os.WriteFile(filepath.Join("/state", "ready"), []byte(time.Now().UTC().Format(time.RFC3339Nano)), 0o644); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "switchyard compose runtime fixture")
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprintln(w, "ok") })
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Print("info fixture ready")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func healthcheck() error {
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get("http://127.0.0.1:8080/health")
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned HTTP %d", response.StatusCode)
	}
	return nil
}
