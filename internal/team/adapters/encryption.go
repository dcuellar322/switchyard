package adapters

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"

	"switchyard.dev/switchyard/internal/team/domain"
)

const maximumSyncDocumentSize = 10 << 20

// GenerateSyncIdentity returns one age X25519 identity and its public recipient.
func GenerateSyncIdentity() (identity, recipient string, err error) {
	generated, err := age.GenerateX25519Identity()
	if err != nil {
		return "", "", err
	}
	return generated.String(), generated.Recipient().String(), nil
}

// EncryptSync produces an ASCII-armored standard age file for one or more
// explicitly supplied recipients.
func EncryptSync(document domain.SyncDocument, recipientValues []string) ([]byte, error) {
	if len(recipientValues) == 0 || len(recipientValues) > 100 {
		return nil, errors.New("one to 100 age recipients are required")
	}
	recipients := make([]age.Recipient, 0, len(recipientValues))
	for _, value := range recipientValues {
		recipient, err := age.ParseX25519Recipient(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("parse age recipient: %w", err)
		}
		recipients = append(recipients, recipient)
	}
	plain, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	if len(plain) > maximumSyncDocumentSize {
		return nil, errors.New("sync document exceeds the size limit")
	}
	var output bytes.Buffer
	armored := armor.NewWriter(&output)
	encrypted, err := age.Encrypt(armored, recipients...)
	if err != nil {
		_ = armored.Close()
		return nil, err
	}
	if _, err := encrypted.Write(plain); err != nil {
		return nil, err
	}
	if err := encrypted.Close(); err != nil {
		return nil, err
	}
	if err := armored.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// DecryptSync decrypts a bounded armored age document with one explicitly
// selected local identity.
func DecryptSync(encrypted []byte, identityValue string) (domain.SyncDocument, error) {
	if len(encrypted) == 0 || len(encrypted) > maximumSyncDocumentSize*2 {
		return domain.SyncDocument{}, errors.New("encrypted sync document is invalid or too large")
	}
	identity, err := age.ParseX25519Identity(strings.TrimSpace(identityValue))
	if err != nil {
		return domain.SyncDocument{}, fmt.Errorf("parse age identity: %w", err)
	}
	reader, err := age.Decrypt(armor.NewReader(bytes.NewReader(encrypted)), identity)
	if err != nil {
		return domain.SyncDocument{}, fmt.Errorf("decrypt sync document: %w", err)
	}
	plain, err := io.ReadAll(io.LimitReader(reader, maximumSyncDocumentSize+1))
	if err != nil {
		return domain.SyncDocument{}, err
	}
	if len(plain) > maximumSyncDocumentSize {
		return domain.SyncDocument{}, errors.New("decrypted sync document exceeds the size limit")
	}
	var document domain.SyncDocument
	decoder := json.NewDecoder(bytes.NewReader(plain))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return domain.SyncDocument{}, fmt.Errorf("decode sync document: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.SyncDocument{}, errors.New("sync document contains multiple JSON values")
	}
	return document, nil
}
