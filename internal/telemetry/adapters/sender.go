// Package adapters contains the bounded HTTPS telemetry delivery adapter.
package adapters

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"switchyard.dev/switchyard/internal/telemetry/domain"
)

// HTTPSender delivers bounded JSON over TLS 1.3 with strict timeouts.
type HTTPSender struct{ client *http.Client }

// NewHTTPSender constructs the production anonymous metrics delivery adapter.
func NewHTTPSender() *HTTPSender {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13},
		ForceAttemptHTTP2: true, MaxIdleConns: 2, IdleConnTimeout: 30 * time.Second,
	}
	return &HTTPSender{client: &http.Client{Transport: transport, Timeout: 10 * time.Second}}
}

// Send posts one bounded payload and accepts only a successful HTTP response.
func (s *HTTPSender) Send(ctx context.Context, endpoint string, payload domain.Payload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if len(encoded) > 64<<10 {
		return errors.New("anonymous telemetry payload exceeds the size limit")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Switchyard-anonymous-metrics")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("anonymous telemetry endpoint returned HTTP %d", response.StatusCode)
	}
	return nil
}
