package adapters

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"switchyard.dev/switchyard/internal/telemetry/domain"
)

func TestHTTPSenderDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	requests := 0
	sender := NewHTTPSender()
	sender.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Header:     http.Header{"Location": []string{"https://redirect.example.test/collect"}},
			Body:       io.NopCloser(strings.NewReader("redirect")),
			Request:    request,
		}, nil
	})
	err := sender.Send(context.Background(), "https://metrics.example.test/collect", domain.Payload{})
	if err == nil || !strings.Contains(err.Error(), "redirects are disabled") {
		t.Fatalf("Send() redirect error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("telemetry transport requests = %d, want 1", requests)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
