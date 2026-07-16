// Package httpapi translates local HTTP requests into application use cases.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

// New constructs the local HTTP and WebSocket router.
func New(system systemQuery, events http.Handler, web http.Handler, logger *slog.Logger) http.Handler {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler { return withCorrelation(logger, next) })

	api := chi.NewRouter()
	generated.HandlerFromMux(&handler{system: system}, api)
	router.Mount("/api/v1", api)
	router.Handle("/ws/v1/events", events)
	router.Handle("/*", web)
	router.Handle("/", web)
	return router
}
