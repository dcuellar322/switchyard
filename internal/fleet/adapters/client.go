package adapters

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/fleet/domain"
)

const maximumRemoteResponse = 1 << 20

// HTTPSPeerClient connects directly to an explicitly configured peer endpoint.
// An operator-provided tunnel may carry that connection, but TLS identity and
// application authorization remain Switchyard responsibilities.
type HTTPSPeerClient struct{ timeout time.Duration }

// NewHTTPSPeerClient creates a bounded peer client.
func NewHTTPSPeerClient() *HTTPSPeerClient { return &HTTPSPeerClient{timeout: 10 * time.Second} }

// Snapshot reads the peer's bounded inventory.
func (c *HTTPSPeerClient) Snapshot(ctx context.Context, machine domain.Machine) (domain.Snapshot, error) {
	var snapshot domain.Snapshot
	if err := c.do(ctx, machine, http.MethodGet, "/remote/v1/snapshot", nil, &snapshot); err != nil {
		return domain.Snapshot{}, err
	}
	return snapshot, nil
}

// Operate submits one typed, confirmed lifecycle action.
func (c *HTTPSPeerClient) Operate(ctx context.Context, machine domain.Machine, request domain.OperationRequest) (domain.OperationReceipt, error) {
	var receipt domain.OperationReceipt
	if err := c.do(ctx, machine, http.MethodPost, "/remote/v1/operations", request, &receipt); err != nil {
		return domain.OperationReceipt{}, err
	}
	return receipt, nil
}

func (c *HTTPSPeerClient) do(ctx context.Context, machine domain.Machine, method, path string, input, output any) error {
	client, endpoint, err := c.client(machine)
	if err != nil {
		return err
	}
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = strings.NewReader(string(encoded))
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint+path, body)
	if err != nil {
		return err
	}
	request.Header.Set(protocolHeader, domain.ProtocolVersion)
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("connect to authenticated remote peer: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.Header.Get(protocolHeader) != domain.ProtocolVersion {
		return errors.New("remote peer returned an incompatible protocol")
	}
	limited := io.LimitReader(response.Body, maximumRemoteResponse+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(payload) > maximumRemoteResponse {
		return errors.New("remote response exceeded the size limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var problem remoteProblem
		_ = json.Unmarshal(payload, &problem)
		if problem.Code == "" {
			problem.Code = "REMOTE_REQUEST_FAILED"
		}
		return fmt.Errorf("remote peer rejected request with %s (%d)", problem.Code, response.StatusCode)
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("decode remote response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("remote response contains multiple JSON values")
	}
	return nil
}

func (c *HTTPSPeerClient) client(machine domain.Machine) (*http.Client, string, error) {
	endpoint, err := url.Parse(normalizeRemoteEndpoint(machine.Endpoint))
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" {
		return nil, "", errors.New("remote endpoint is invalid")
	}
	caDocument, err := os.ReadFile(machine.Credentials.CACertificate)
	if err != nil {
		return nil, "", fmt.Errorf("read remote CA certificate: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caDocument) {
		return nil, "", errors.New("remote CA certificate is invalid")
	}
	certificate, err := tls.LoadX509KeyPair(machine.Credentials.ClientCertificate, machine.Credentials.ClientKey)
	if err != nil {
		return nil, "", fmt.Errorf("load remote client identity: %w", err)
	}
	expected, err := hex.DecodeString(machine.CertificateFingerprint)
	if err != nil || len(expected) != sha256.Size {
		return nil, "", errors.New("remote certificate pin is invalid")
	}
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{certificate},
		ServerName: endpoint.Hostname(),
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return errors.New("remote peer certificate is missing")
			}
			actual := sha256.Sum256(state.PeerCertificates[0].Raw)
			if !equalBytes(actual[:], expected) {
				return errors.New("remote peer certificate pin changed")
			}
			return nil
		},
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment, TLSClientConfig: tlsConfig,
		ForceAttemptHTTP2: true, MaxIdleConns: 4, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second,
	}
	return &http.Client{Transport: transport, Timeout: c.timeout}, endpoint.String(), nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}
