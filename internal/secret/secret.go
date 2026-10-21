// Package secret keeps connection secrets. A Vault hands plaintext to the
// session manager and nothing else; at rest, secrets are AES-256-GCM
// ciphertext in the store, or references resolved at use ("env:NAME").
package secret

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/errs"
)

// Vault stores and resolves secrets by connection ID.
type Vault interface {
	Get(ctx context.Context, id uuid.UUID) (string, error)
	Set(ctx context.Context, id uuid.UUID, secret string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// Blobs is the storage a Vault encrypts into; store.Secrets satisfies it.
type Blobs interface {
	Get(ctx context.Context, id uuid.UUID) ([]byte, error)
	Set(ctx context.Context, id uuid.UUID, blob []byte) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// Local is a vault that encrypts with a key Mesa holds (APP_KEY in server
// mode, a key file under the config directory on desktop and CLI).
type Local struct {
	aead  cipher.AEAD
	blobs Blobs
}

// NewLocal derives a 256-bit key from keyMaterial (a hex key or any passphrase).
func NewLocal(keyMaterial string, blobs Blobs) (*Local, error) {
	if strings.TrimSpace(keyMaterial) == "" {
		return nil, errs.New(errs.Invalid, "secret key is empty")
	}
	var key []byte
	if raw, err := hex.DecodeString(keyMaterial); err == nil && len(raw) == 32 {
		key = raw
	} else {
		sum := sha256.Sum256([]byte(keyMaterial))
		key = sum[:]
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Local{aead: aead, blobs: blobs}, nil
}

// GenerateKey returns a fresh 32-byte key as hex.
func GenerateKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Get decrypts the secret for id, resolving "env:NAME" references.
func (l *Local) Get(ctx context.Context, id uuid.UUID) (string, error) {
	blob, err := l.blobs.Get(ctx, id)
	if err != nil {
		return "", err
	}
	plain, err := l.open(blob)
	if err != nil {
		return "", err
	}
	return Resolve(plain)
}

// Set encrypts and stores the secret (or reference) for id.
func (l *Local) Set(ctx context.Context, id uuid.UUID, secret string) error {
	blob, err := l.seal([]byte(secret))
	if err != nil {
		return err
	}
	return l.blobs.Set(ctx, id, blob)
}

func (l *Local) Delete(ctx context.Context, id uuid.UUID) error { return l.blobs.Delete(ctx, id) }

func (l *Local) seal(plain []byte) ([]byte, error) {
	nonce := make([]byte, l.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return append(nonce, l.aead.Seal(nil, nonce, plain, nil)...), nil
}

func (l *Local) open(blob []byte) (string, error) {
	n := l.aead.NonceSize()
	if len(blob) < n {
		return "", errs.New(errs.Invalid, "stored secret is corrupt")
	}
	plain, err := l.aead.Open(nil, blob[:n], blob[n:], nil)
	if err != nil {
		return "", errs.Wrap(errs.Forbidden, "stored secret cannot be decrypted with this key", err)
	}
	return string(plain), nil
}

// Resolve turns a reference into a value: "env:NAME" reads the environment,
// anything else is returned as is. Keychain references are reserved for the
// desktop build.
func Resolve(s string) (string, error) {
	switch {
	case strings.HasPrefix(s, "env:"):
		name := strings.TrimPrefix(s, "env:")
		v, ok := os.LookupEnv(name)
		if !ok {
			return "", errs.Newf(errs.Unavailable, "environment variable %s is not set", name)
		}
		return v, nil
	case strings.HasPrefix(s, "keychain:"):
		return "", errs.New(errs.Unsupported, "keychain references need the desktop build")
	}
	return s, nil
}

// IsReference reports whether s is a reference rather than a literal secret.
func IsReference(s string) bool {
	return strings.HasPrefix(s, "env:") || strings.HasPrefix(s, "keychain:")
}
