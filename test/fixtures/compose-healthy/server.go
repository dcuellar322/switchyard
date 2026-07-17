// Command compose-healthy-fixture is an offline Docker integration target.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "healthcheck":
			if err := checkHealth(); err != nil {
				log.Print(err)
				os.Exit(1)
			}
			return
		case "unhealthy":
			os.Exit(1)
		}
	}
	if err := serve(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func serve() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "switchyard healthy compose fixture")
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	})
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	log.Print("info healthy compose fixture ready")
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func checkHealth() error {
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
