package compose

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/moby/client"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

const commandOutputLimit = 4 << 20

type dockerConnection struct {
	ContextName   string
	Host          string
	SkipTLSVerify bool
	TLSPath       string
	FromEnv       bool
}

type contextResolver struct {
	runner commandRunner
}

func (r contextResolver) Resolve(ctx context.Context, requested, workingDirectory string) (dockerConnection, error) {
	if requested == "" && os.Getenv(client.EnvOverrideHost) != "" {
		return dockerConnection{FromEnv: true}, nil
	}
	name := requested
	if name == "" {
		var stdout, stderr limitedBuffer
		err := r.runner.Run(ctx, domain.Command{Executable: "docker", Arguments: []string{"context", "show"}, WorkingDirectory: workingDirectory}, &stdout, &stderr)
		if err != nil {
			return dockerConnection{}, commandError("read active Docker context", err, stderr.String())
		}
		name = strings.TrimSpace(stdout.String())
	}
	if name == "" {
		return dockerConnection{}, errors.New("docker context name is empty")
	}
	var stdout, stderr limitedBuffer
	err := r.runner.Run(ctx, domain.Command{Executable: "docker", Arguments: []string{"context", "inspect", name}, WorkingDirectory: workingDirectory}, &stdout, &stderr)
	if err != nil {
		return dockerConnection{}, commandError("inspect Docker context", err, stderr.String())
	}
	var documents []struct {
		Name      string `json:"Name"`
		Endpoints map[string]struct {
			Host          string `json:"Host"`
			SkipTLSVerify bool   `json:"SkipTLSVerify"`
		} `json:"Endpoints"`
		Storage struct {
			TLSPath string `json:"TLSPath"`
		} `json:"Storage"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &documents); err != nil || len(documents) != 1 {
		return dockerConnection{}, errors.New("docker context inspect returned an invalid document")
	}
	endpoint, ok := documents[0].Endpoints["docker"]
	if !ok || endpoint.Host == "" {
		return dockerConnection{}, fmt.Errorf("docker context %q has no Docker endpoint", name)
	}
	return dockerConnection{
		ContextName: documents[0].Name, Host: endpoint.Host,
		SkipTLSVerify: endpoint.SkipTLSVerify, TLSPath: documents[0].Storage.TLSPath,
	}, nil
}

func (c dockerConnection) cliPrefix() []string {
	if c.ContextName == "" {
		return nil
	}
	return []string{"--context", c.ContextName}
}

func (c dockerConnection) newClient() (*client.Client, error) {
	if c.FromEnv {
		return client.New(client.FromEnv, client.WithUserAgent("switchyard/compose-runtime"))
	}
	if strings.HasPrefix(c.Host, "ssh://") {
		return nil, fmt.Errorf("docker SSH context %q is not supported by the Engine SDK observer", c.ContextName)
	}
	options := []client.Opt{client.WithHost(c.Host), client.WithUserAgent("switchyard/compose-runtime")}
	if strings.HasPrefix(c.Host, "tcp://") {
		if c.SkipTLSVerify {
			return nil, fmt.Errorf("docker context %q disables TLS verification and is refused by Switchyard", c.ContextName)
		}
		ca, cert, key := contextTLSFiles(c.TLSPath)
		options = append(options, client.WithTLSClientConfig(ca, cert, key))
	}
	return client.New(options...)
}

func contextTLSFiles(root string) (string, string, string) {
	if root == "" {
		return "", "", ""
	}
	directory := filepath.Join(root, "docker")
	files := []string{filepath.Join(directory, "ca.pem"), filepath.Join(directory, "cert.pem"), filepath.Join(directory, "key.pem")}
	for index, path := range files {
		if _, err := os.Stat(path); err != nil {
			files[index] = ""
		}
	}
	return files[0], files[1], files[2]
}

type limitedBuffer struct {
	bytes.Buffer
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	remaining := commandOutputLimit - b.Len()
	if remaining <= 0 {
		return len(value), nil
	}
	if len(value) > remaining {
		_, _ = b.Buffer.Write(value[:remaining])
		return len(value), nil
	}
	return b.Buffer.Write(value)
}

func commandError(action string, err error, detail string) error {
	detail = strings.TrimSpace(strings.ReplaceAll(detail, "\x00", ""))
	if len(detail) > 2048 {
		detail = detail[:2048] + "..."
	}
	if detail == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, detail)
}
