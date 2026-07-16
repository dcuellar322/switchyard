package adapters

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/fleet/domain"
)

func TestHTTPSPeerClientRequiresMutualTLSAndExactServerPin(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	caCertificate, caKey, caPEM := createTestCA(t)
	serverCertificate, serverRaw, _, _ := createTestCertificate(t, caCertificate, caKey, "server", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, true)
	clientCertificate, _, clientCertPEM, clientKeyPEM := createTestCertificate(t, caCertificate, caKey, "controller", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, false)
	caPath := writeCertificateFixture(t, directory, "ca.pem", caPEM)
	clientPath := writeCertificateFixture(t, directory, "client.pem", clientCertPEM)
	clientKeyPath := writeCertificateFixture(t, directory, "client-key.pem", clientKeyPEM)

	agent := &agentHTTPStub{}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if errors.Is(err, syscall.EPERM) {
		t.Skip("sandbox does not permit loopback listeners")
	}
	if err != nil {
		t.Fatal(err)
	}
	serverPool := x509.NewCertPool()
	serverPool.AddCert(caCertificate)
	server := &http.Server{Handler: NewAgentHandler(agent), ReadHeaderTimeout: time.Second}
	tlsListener := tls.NewListener(listener, &tls.Config{
		MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{{Certificate: [][]byte{serverRaw}, PrivateKey: serverCertificate.PrivateKey}},
		ClientCAs: serverPool, ClientAuth: tls.RequireAndVerifyClientCert,
	})
	go func() { _ = server.Serve(tlsListener) }()
	t.Cleanup(func() { _ = server.Close() })

	digest := sha256.Sum256(serverRaw)
	machine := domain.Machine{
		Endpoint: "https://" + listener.Addr().String(), CertificateFingerprint: hex.EncodeToString(digest[:]),
		Credentials: domain.CredentialReferences{CACertificate: caPath, ClientCertificate: clientPath, ClientKey: clientKeyPath},
	}
	client := NewHTTPSPeerClient()
	snapshot, err := client.Snapshot(context.Background(), machine)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.Identity.MachineID != "" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	clientDigest := sha256.Sum256(clientCertificate.Certificate[0])
	if agent.fingerprint != hex.EncodeToString(clientDigest[:]) {
		t.Fatalf("controller fingerprint = %q", agent.fingerprint)
	}

	machine.CertificateFingerprint = strings.Repeat("0", 64)
	if _, err := client.Snapshot(context.Background(), machine); err == nil || !strings.Contains(err.Error(), "pin changed") {
		t.Fatalf("wrong pin error = %v", err)
	}
}

func createTestCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Switchyard test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return certificate, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw})
}

func createTestCertificate(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, name string, usage []x509.ExtKeyUsage, server bool) (tls.Certificate, []byte, []byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: usage,
	}
	if server {
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyRaw, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyRaw})
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return certificate, raw, certificatePEM, keyPEM
}

func writeCertificateFixture(t *testing.T, directory, name string, value []byte) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, value, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
