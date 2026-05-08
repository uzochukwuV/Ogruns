package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/0xprotocol/verification-layer/pkg/types"
)

// SignalEncryptor provides AES-256-GCM encryption for active signals at rest.
// Only active signals need encryption - closed signals can be public.
type SignalEncryptor struct {
	key    []byte
	gcm    cipher.AEAD
	mu     sync.RWMutex
	keyDir string
}

// NewSignalEncryptor creates a new encryptor using the provided private key as seed.
// The actual encryption key is derived via SHA-256 to ensure proper key length.
func NewSignalEncryptor(privateKeyHex string) (*SignalEncryptor, error) {
	// Derive 256-bit key from private key using SHA-256
	hash := sha256.Sum256([]byte(privateKeyHex + ":signal-encryption-key"))
	key := hash[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &SignalEncryptor{
		key: key,
		gcm: gcm,
	}, nil
}

// Encrypt encrypts plaintext data using AES-256-GCM
func (e *SignalEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Generate random nonce
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt: nonce is prepended to ciphertext
	ciphertext := e.gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (e *SignalEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(ciphertext) < e.gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract nonce from beginning of ciphertext
	nonce := ciphertext[:e.gcm.NonceSize()]
	ciphertext = ciphertext[e.gcm.NonceSize():]

	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// EncryptSignals encrypts a slice of signal envelopes to bytes
func (e *SignalEncryptor) EncryptSignals(signals []types.SignalEnvelope) ([]byte, error) {
	data, err := json.Marshal(signals)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signals: %w", err)
	}
	return e.Encrypt(data)
}

// DecryptSignals decrypts bytes back to signal envelopes
func (e *SignalEncryptor) DecryptSignals(ciphertext []byte) ([]types.SignalEnvelope, error) {
	plaintext, err := e.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}

	var signals []types.SignalEnvelope
	if err := json.Unmarshal(plaintext, &signals); err != nil {
		return nil, fmt.Errorf("failed to unmarshal signals: %w", err)
	}
	return signals, nil
}

// EncryptToBase64 encrypts and returns base64-encoded ciphertext
func (e *SignalEncryptor) EncryptToBase64(plaintext []byte) (string, error) {
	ciphertext, err := e.Encrypt(plaintext)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptFromBase64 decrypts base64-encoded ciphertext
func (e *SignalEncryptor) DecryptFromBase64(encoded string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	return e.Decrypt(ciphertext)
}

// SaveEncryptedSignals saves encrypted signals to a file
func (e *SignalEncryptor) SaveEncryptedSignals(filepath string, signals []types.SignalEnvelope) error {
	ciphertext, err := e.EncryptSignals(signals)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, ciphertext, 0600) // Restrictive permissions
}

// LoadEncryptedSignals loads and decrypts signals from a file
func (e *SignalEncryptor) LoadEncryptedSignals(filepath string) ([]types.SignalEnvelope, error) {
	ciphertext, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No persisted signals
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return e.DecryptSignals(ciphertext)
}

// EncryptedSignalStore provides persistent encrypted storage for active signals
type EncryptedSignalStore struct {
	encryptor *SignalEncryptor
	filepath  string
	mu        sync.Mutex
}

// NewEncryptedSignalStore creates a new encrypted signal store
func NewEncryptedSignalStore(dataDir, privateKeyHex string) (*EncryptedSignalStore, error) {
	encryptor, err := NewSignalEncryptor(privateKeyHex)
	if err != nil {
		return nil, err
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	return &EncryptedSignalStore{
		encryptor: encryptor,
		filepath:  filepath.Join(dataDir, "active_signals.enc"),
	}, nil
}

// Save persists signals to encrypted storage
func (s *EncryptedSignalStore) Save(signals []types.SignalEnvelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.encryptor.SaveEncryptedSignals(s.filepath, signals)
}

// Load retrieves signals from encrypted storage
// If the file is corrupted (wrong key, etc.), it deletes it and returns empty slice
func (s *EncryptedSignalStore) Load() ([]types.SignalEnvelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	signals, err := s.encryptor.LoadEncryptedSignals(s.filepath)
	if err != nil {
		// File is corrupted or encrypted with different key - delete and start fresh
		os.Remove(s.filepath)
		return nil, nil
	}
	return signals, nil
}

// Clear removes the encrypted signal file
func (s *EncryptedSignalStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.filepath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
