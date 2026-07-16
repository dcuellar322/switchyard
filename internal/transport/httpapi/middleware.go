package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"switchyard.dev/switchyard/internal/foundation/correlation"
)

const correlationHeader = "X-Correlation-ID"

func withCorrelation(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		id := r.Header.Get(correlationHeader)
		if id == "" {
			generated, err := correlation.NewID()
			if err != nil {
				writeProblem(w, r, http.StatusInternalServerError, "INTERNAL", "Internal server error", "A correlation identifier could not be created.")
				return
			}
			id = generated
		}
		w.Header().Set(correlationHeader, id)
		r = r.WithContext(correlation.WithID(r.Context(), id))
		next.ServeHTTP(w, r)
		logger.InfoContext(
			r.Context(),
			"http request completed",
			"component", "httpapi",
			"correlation_id", id,
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(startedAt),
		)
	})
}
