package cli

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	teamApplication "switchyard.dev/switchyard/internal/team/application"
)

const signingKeySchema = "switchyard.signing-key/v1"

type signingKeyFile struct {
	SchemaVersion string `json:"schemaVersion"`
	PublisherID   string `json:"publisherId"`
	PublicKey     string `json:"publicKey"`
	PrivateKey    string `json:"privateKey"`
}

func readSigningKey(path string) (signingKeyFile, ed25519.PrivateKey, error) {
	encoded, err := readBoundedFile(path, 8<<10)
	if err != nil {
		return signingKeyFile{}, nil, err
	}
	var key signingKeyFile
	if err := decodeStrictJSON(encoded, &key); err != nil || key.SchemaVersion != signingKeySchema {
		return signingKeyFile{}, nil, errors.New("signing key file is invalid")
	}
	privateKey, err := base64.StdEncoding.DecodeString(key.PrivateKey)
	if err != nil || len(privateKey) != ed25519.PrivateKeySize {
		return signingKeyFile{}, nil, errors.New("signing private key is invalid")
	}
	publicKey := ed25519.PrivateKey(privateKey).Public().(ed25519.PublicKey)
	if base64.StdEncoding.EncodeToString(publicKey) != key.PublicKey || teamApplication.PublisherID(publicKey) != key.PublisherID {
		return signingKeyFile{}, nil, errors.New("signing key public identity does not match")
	}
	return key, ed25519.PrivateKey(privateKey), nil
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	value, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(value)) > limit {
		return nil, fmt.Errorf("file exceeds %d byte limit", limit)
	}
	return value, nil
}

func writeExclusiveFile(path string, value []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(value); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func decodeStrictJSON(value []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("JSON contains multiple values")
	}
	return nil
}
